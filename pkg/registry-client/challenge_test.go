/*
 Copyright (c) 2025 Arenadata Softwer LLC.
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package client

import (
	"net/http"
	"reflect"
	"testing"
)

const (
	bearer1 = `Bearer realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:library/postgres:pull"`
	bearer2 = `Bearer key="key-e=value-a,value-b,value-c", key2=value2,key3=value3`
	basic   = `BASIC realm="Sonatype Nexus Repository Manager"`
	digest1 = `Digest
	username="Mufasa",
realm="http-auth@example.org",
	uri="/dir/index.html",
	 algorithm=MD5,
	nonce="7ypf/xlj9XXwfDPEoM4URrv/xwf94B
cCAzFZH4GiTo0v",
	nc=00000001,
	cnonce="f2/wE4q74E6zIJEtWaHKaf5wv/H5QzzpXusqGemxURZJ",
	qop=auth,
	response="8ca523f5e9506fed4657c9700eebdbec",
	opaque="FQhe/qaU925kfnzjCev0ciny7QMkPqMAFRtzCUYo5tdS"

`
	digest2 = `Digest
		realm="http-auth@example.org",
		qop="auth, auth-int",
		algorithm=SHA-256,
		nonce="7ypf/xlj9XXwfDPEoM4URrv/xwf94BcCAzFZH4GiTo0v",
		opaque="FQhe/qaU925kfnzjCev0ciny7QMkPqMAFRtzCUYo5tdS",`

	allWWWAuthorization = bearer1 + "\n, " + basic + "," + digest2 + digest1
)

var (
	bearer1Challenge = &challenge{"Bearer", map[string]string{"realm": "https://auth.docker.io/token", "scope": "repository:library/postgres:pull", "service": "registry.docker.io"}}
	bearer2Challenge = &challenge{"Bearer", map[string]string{"key": "key-e=value-a,value-b,value-c", "key2": "value2", "key3": "value3"}}
	basicChallenge   = &challenge{"BASIC", map[string]string{"realm": "Sonatype Nexus Repository Manager"}}
	digest1Challenge = &challenge{"Digest", map[string]string{"algorithm": "MD5", "cnonce": "f2/wE4q74E6zIJEtWaHKaf5wv/H5QzzpXusqGemxURZJ", "nc": "00000001", "nonce": "7ypf/xlj9XXwfDPEoM4URrv/xwf94BcCAzFZH4GiTo0v", "opaque": "FQhe/qaU925kfnzjCev0ciny7QMkPqMAFRtzCUYo5tdS", "qop": "auth", "realm": "http-auth@example.org", "response": "8ca523f5e9506fed4657c9700eebdbec", "uri": "/dir/index.html", "username": "Mufasa"}}
	digest2Challenge = &challenge{"Digest", map[string]string{"algorithm": "SHA-256", "nonce": "7ypf/xlj9XXwfDPEoM4URrv/xwf94BcCAzFZH4GiTo0v", "opaque": "FQhe/qaU925kfnzjCev0ciny7QMkPqMAFRtzCUYo5tdS", "qop": "auth, auth-int", "realm": "http-auth@example.org"}}

	allWWWAuthorizationChallenge []*challenge
)

func init() {
	allWWWAuthorizationChallenge = append(allWWWAuthorizationChallenge, bearer1Challenge)
	allWWWAuthorizationChallenge = append(allWWWAuthorizationChallenge, basicChallenge)
	allWWWAuthorizationChallenge = append(allWWWAuthorizationChallenge, digest2Challenge)
	allWWWAuthorizationChallenge = append(allWWWAuthorizationChallenge, digest1Challenge)
}

func Test_parseWWWAuthenticateHeader(t *testing.T) {
	hdr := func(s ...string) http.Header {
		k := http.CanonicalHeaderKey("www-authenticate")
		return http.Header{k: s}
	}

	tests := []struct {
		name string
		hdr  http.Header
		want []*challenge
	}{
		{"Bearer1", hdr(bearer1), []*challenge{bearer1Challenge}},
		{"Bearer2", hdr(bearer2), []*challenge{bearer2Challenge}},
		{"Basic", hdr(basic), []*challenge{basicChallenge}},
		{"Digest1", hdr(digest1), []*challenge{digest1Challenge}},
		{"Digest2", hdr(digest2), []*challenge{digest2Challenge}},
		{"MultipleWWWAuthorizations1", hdr(allWWWAuthorization), allWWWAuthorizationChallenge},
		{"MultipleWWWAuthorizations2", hdr(bearer1, basic, digest2, digest1), allWWWAuthorizationChallenge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseWWWAuthenticateHeader(tt.hdr); !reflect.DeepEqual(got, tt.want) {
				nGot, nWant := make([]challenge, len(got)), make([]challenge, len(tt.want))
				for i, ch := range got {
					nGot[i] = *ch
				}
				for i, ch := range tt.want {
					nWant[i] = *ch
				}
				t.Errorf("parseWWWAuthenticateHeader() = %v, want %v", nGot, nWant)
			}
		})
	}
}

func Test_parseWWWAuthorization(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want []*challenge
	}{
		{"Bearer1", bearer1, []*challenge{bearer1Challenge}},
		{"Bearer2", bearer2, []*challenge{bearer2Challenge}},
		{"Basic", basic, []*challenge{basicChallenge}},
		{"Digest1", digest1, []*challenge{digest1Challenge}},
		{"Digest2", digest2, []*challenge{digest2Challenge}},
		{"MultipleWWWAuthorizations", allWWWAuthorization, allWWWAuthorizationChallenge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseWWWAuthorization(tt.arg); !reflect.DeepEqual(got, tt.want) {
				nGot, nWant := make([]challenge, len(got)), make([]challenge, len(tt.want))
				for i, ch := range got {
					nGot[i] = *ch
				}
				for i, ch := range tt.want {
					nWant[i] = *ch
				}
				t.Errorf("parseWWWAuthorization() = %#v, want %v", nGot, nWant)
			}
		})
	}
}

func Test_urlFromChallenge(t *testing.T) {
	tests := []struct {
		name    string
		arg     *challenge
		want    string
		wantErr bool
	}{
		{"Bearer1", bearer1Challenge, "https://auth.docker.io/token?scope=repository%3Alibrary%2Fpostgres%3Apull&service=registry.docker.io", false},
		{"Bearer2", bearer2Challenge, "", true},
		{"Basic", basicChallenge, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := urlFromChallenge(tt.arg)
			if (err != nil) != tt.wantErr {
				t.Errorf("urlFromChallenge() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("urlFromChallenge() got = %v, want %v", got, tt.want)
			}
		})
	}
}
