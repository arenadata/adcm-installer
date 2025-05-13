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
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"time"
)

const userAgent = "ad-registry-client/0.1.0"

var Request = request{}

func NewRequest() request {
	return request{}
}

type requestOption func(*requestOpts)

func RequestHeader(header http.Header) requestOption {
	return func(r *requestOpts) {
		r.header = header
	}
}

func RequestBody(body io.Reader) requestOption {
	return func(r *requestOpts) {
		r.body = body
	}
}

func RequestContext(ctx context.Context) requestOption {
	return func(r *requestOpts) {
		r.ctx = ctx
	}
}

func RequestTimeout(timeout time.Duration) requestOption {
	return func(r *requestOpts) {
		r.timeout = timeout
	}
}

func RequestCredentials(cred Credentials) requestOption {
	return func(r *requestOpts) {
		r.cred = cred
	}
}

func InsecureRequest() requestOption {
	return func(r *requestOpts) {
		r.insecure = true
	}
}

type request struct{}

type requestOpts struct {
	ctx      context.Context
	body     io.Reader
	header   http.Header
	timeout  time.Duration
	cred     Credentials
	insecure bool
}

func newRequest(method, url string, opts ...requestOption) (*http.Response, error) {
	r := new(requestOpts)
	for _, opt := range opts {
		opt(r)
	}

	var nilCtx bool
	if r.ctx == nil {
		r.ctx = context.Background()
		nilCtx = true
	}

	req, err := http.NewRequestWithContext(r.ctx, method, url, r.body)
	if err != nil {
		return nil, err
	}
	if r.cred != nil {
		r.cred.Set(req)
	}

	req.Header.Add("User-Agent", userAgent)
	for k, v := range r.header {
		req.Header[k] = v
	}

	client := new(http.Client)
	if nilCtx {
		client.Timeout = r.timeout
	}
	if r.insecure {
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}

	return client.Do(req)
}

func (request) Get(url string, opts ...requestOption) (*http.Response, error) {
	return newRequest(http.MethodGet, url, opts...)
}

func (request) Post(url string, opts ...requestOption) (*http.Response, error) {
	return newRequest(http.MethodPost, url, opts...)
}

func (request) Put(url string, opts ...requestOption) (*http.Response, error) {
	return newRequest(http.MethodPut, url, opts...)
}

func (request) Patch(url string, opts ...requestOption) (*http.Response, error) {
	return newRequest(http.MethodPatch, url, opts...)
}
