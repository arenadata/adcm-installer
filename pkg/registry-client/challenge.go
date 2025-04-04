package client

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type challenge struct {
	scheme     string
	parameters map[string]string
}

func parseWWWAuthenticateHeader(header http.Header) []*challenge {
	var out []*challenge
	for _, h := range header[http.CanonicalHeaderKey("WWW-Authenticate")] {
		out = append(out, parseWWWAuthorization(h)...)
	}

	return out
}

func parseWWWAuthorization(s string) []*challenge {
	var challenges []string
	var buf []rune

	var isQuotedData, isEscapedData bool
	for n, c := range s {
		isQuoted := c == '"'
		if isQuoted && !isEscapedData {
			isQuotedData = !isQuotedData
		}

		isEscaped := c == '\\'
		if isEscaped {
			isEscapedData = true
		}

		if isCrLf := strings.ContainsRune("\r\n", c); isCrLf {
			continue
		}

		isSeparator := strings.ContainsRune(" \t=,", c)

		if !isQuotedData && !isEscapedData && isSeparator {
			if len(buf) > 0 {
				challenges = append(challenges, string(buf))
				buf = nil
			}

			if c == '=' {
				challenges = append(challenges, "=")
			}
			continue
		}

		if !(isQuoted && !isEscapedData) {
			buf = append(buf, c)
		}

		if !isEscaped && isEscapedData {
			isEscapedData = false
		}

		if n == len(s)-1 {
			challenges = append(challenges, string(buf))
		}
	}

	var out []*challenge
	l := len(challenges)

	ch := &challenge{parameters: make(map[string]string)}
	for n := 0; n < l; n++ {
		v := challenges[n]

		if challenges[n+1] == "=" {
			n += 2
			ch.parameters[v] = challenges[n]
		} else {
			if len(ch.scheme) > 0 {
				out = append(out, ch)
				ch = &challenge{parameters: make(map[string]string)}
			}
			ch.scheme = v
		}

		if n == l-1 {
			out = append(out, ch)
		}
	}

	return out
}

func urlFromChallenge(challenge *challenge) (string, error) {
	realm, ok := challenge.parameters["realm"]
	if !ok {
		return "", fmt.Errorf("no realm in challenge")
	}
	service, ok := challenge.parameters["service"]
	if !ok {
		return "", fmt.Errorf("no service in challenge")
	}

	u, err := url.Parse(realm)
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("service", service)
	if scope, ok := challenge.parameters["scope"]; ok && len(scope) > 0 {
		q.Set("scope", scope)
	}

	u.RawQuery = q.Encode()

	return u.String(), nil
}
