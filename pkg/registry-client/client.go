package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrNoMorePages = errors.New("no more pages")

type Credentials interface {
	Set(r *http.Request)
}

type AuthBasic struct {
	login    string
	password string
}

func NewAuthBasic(login, password string) *AuthBasic {
	return &AuthBasic{login: login, password: password}
}

func (a *AuthBasic) Set(r *http.Request) {
	r.SetBasicAuth(a.login, a.password)
}

type authTokenResponse struct {
	Token string `json:"token"`
}

type AuthToken struct {
	scheme string
	token  string
}

func NewAuthToken(token, scheme string) *AuthToken {
	if len(scheme) == 0 {
		scheme = "Bearer"
	}
	return &AuthToken{scheme: scheme, token: token}
}

func (t *AuthToken) Set(r *http.Request) {
	r.Header.Set("Authorization", fmt.Sprintf("%s %s", t.scheme, t.token))
}

type RegistryOption func(*RegistryClient)

func WihCredentials(cred Credentials) RegistryOption {
	return func(rc *RegistryClient) {
		rc.cred = cred
	}
}

func WithInsecure() RegistryOption {
	return func(rc *RegistryClient) {
		rc.insecure = true
	}
}

func WithDisableSSL() RegistryOption {
	return func(rc *RegistryClient) {
		rc.ssl = false
	}
}

func WithBlobsDir(path string) RegistryOption {
	return func(rc *RegistryClient) {
		rc.layersDir = path
	}
}

type RegistryClient struct {
	cred        Credentials
	sessionCred Credentials

	insecure bool
	ssl      bool
	url      string

	layersDir string
}

func NewRegistryClient(url string, opts ...RegistryOption) *RegistryClient {
	rc := &RegistryClient{ssl: true}
	for _, opt := range opts {
		opt(rc)
	}

	if !strings.HasPrefix(url, "http") {
		pfx := "http"
		if rc.ssl {
			pfx += "s"
		}

		url = fmt.Sprintf("%s://%s", pfx, url)
	}
	rc.url = url

	return rc
}

func (rc *RegistryClient) auth(hdr http.Header) error {
	rc.sessionCred = rc.cred

	var challenges []*challenge
	if len(hdr.Get("WWW-Authenticate")) > 0 {
		challenges = parseWWWAuthenticateHeader(hdr)
		if len(challenges) == 0 {
			return fmt.Errorf("no authentication challenge in the WWW-Authenticate header provided")
		}
		if len(challenges) == 1 && strings.ToLower(challenges[0].scheme) == "basic" {
			return nil
		}
	} else if rc.cred != nil {
		return nil
	} else {
		return fmt.Errorf("no WWW-Authenticate header and credentials provided")
	}

	var chl *challenge
	for _, c := range challenges {
		if strings.ToLower(c.scheme) == "bearer" {
			chl = c
			break
		}
	}
	if chl == nil {
		return fmt.Errorf("no authentication challenge in the WWW-Authenticate header found")
	}

	u, err := urlFromChallenge(chl)
	if err != nil {
		return fmt.Errorf("could not parse authentication challenge: %v", err)
	}

	opts := []requestOption{
		RequestTimeout(5 * time.Second),
		RequestCredentials(rc.cred),
	}
	if rc.insecure {
		opts = append(opts, InsecureRequest())
	}

	resp, err := Request.Get(u, opts...)
	if err != nil {
		return fmt.Errorf("could not retrieve authentication data: %v", err)
	}

	if resp.StatusCode == http.StatusOK {
		tok := authTokenResponse{}
		dec := json.NewDecoder(resp.Body)
		if err = dec.Decode(&tok); err != nil {
			return err
		}

		rc.sessionCred = NewAuthToken(tok.Token, chl.scheme)
		return nil
	}

	b, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("invalid status code %d (%s): %q", resp.StatusCode, resp.Status, b)
}

func (rc *RegistryClient) repoUrl(repo string) string {
	u := rc.url
	if u[len(u)-1] != '/' {
		u += "/"
	}
	u += "v2"

	if repo[0] != '/' {
		u += "/"
	}

	return u + repo
}

func (rc *RegistryClient) get(url string, header http.Header) (*http.Response, error) {
	opts := []requestOption{
		RequestTimeout(5 * time.Second),
		RequestHeader(header),
	}

	resp, err := Request.Get(url, append(opts, RequestCredentials(rc.sessionCred))...)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		if err = rc.auth(resp.Header); err != nil {
			return nil, fmt.Errorf("could not authenticate: %v", err)
		}
		return Request.Get(url, append(opts, RequestCredentials(rc.sessionCred))...)
	}

	return resp, nil
}

func (rc *RegistryClient) resp(url, mediaType string) ([]byte, error) {
	hdr := http.Header{}
	hdr.Set("Accept", mediaType)

	resp, err := rc.get(url, hdr)
	if err != nil {
		return nil, err
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid status code %d (%s): %s", resp.StatusCode, resp.Status, b)
	}

	return b, nil
}

func (rc *RegistryClient) Blob(repo, tag, mediaType string) ([]byte, error) {
	u := fmt.Sprintf("%s/blobs/%s", rc.repoUrl(repo), tag)
	return rc.resp(u, mediaType)
}

func (rc *RegistryClient) Manifest(repo, tag, mediaType string) ([]byte, error) {
	u := fmt.Sprintf("%s/manifests/%s", rc.repoUrl(repo), tag)
	return rc.resp(u, mediaType)
}

type tagsResponse struct {
	Tags []string `json:"tags"`
}

func (rc *RegistryClient) Tags(repo string) (tags []string, err error) {
	url := fmt.Sprintf("%s/tags/list", rc.repoUrl(repo))
	var response tagsResponse
	for {
		url, err = rc.getPaginatedJSON(url, &response)
		if errors.Is(err, ErrNoMorePages) {
			tags = append(tags, response.Tags...)
			return tags, nil
		} else if err != nil {
			return nil, err
		} else {
			tags = append(tags, response.Tags...)
		}
	}
}

func (rc *RegistryClient) getPaginatedJSON(url string, response interface{}) (string, error) {
	resp, err := rc.get(url, nil)
	if err != nil {
		return "", err
	}

	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(response)
	if err != nil {
		return "", err
	}
	return getNextLink(resp)
}

func getNextLink(resp *http.Response) (string, error) {
	for _, link := range resp.Header[http.CanonicalHeaderKey("Link")] {
		for _, part := range strings.Split(link, ";") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "<") && strings.HasSuffix(part, ">") {
				return part[1 : len(part)-1], nil
			}
		}
	}
	return "", ErrNoMorePages
}
