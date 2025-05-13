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
	valueLength := len(s)
	for n, c := range s {
		isQuoted := c == '"'
		if isQuoted && !isEscapedData {
			isQuotedData = !isQuotedData
		}

		isEscaped := c == '\\'
		if isEscaped {
			isEscapedData = true
		}

		isCrLf := strings.ContainsRune("\r\n", c)
		isSeparator := strings.ContainsRune(" \t=,", c)

		if !isQuotedData && !isEscapedData && (isSeparator || isCrLf) {
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
			if !isCrLf {
				buf = append(buf, c)
			}
		}

		if !isEscaped && isEscapedData {
			isEscapedData = false
		}

		if n == valueLength-1 {
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
