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

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/arenadata/adcm-installer/pkg/vault/unseal"

	"github.com/openbao/openbao/api/v2"
)

type Api struct {
	client *api.Client
}

var _ unseal.Runner = (*Api)(nil)

func New(addr string) (*Api, error) {
	conf := api.DefaultConfig()
	if len(addr) > 0 {
		conf.Address = addr
	}

	client, err := api.NewClient(conf)
	if err != nil {
		return nil, err
	}

	return &Api{client: client}, nil
}

func (a *Api) Init(ctx context.Context) (*unseal.VaultInitData, error) {
	req := &api.InitRequest{
		SecretShares:    5,
		SecretThreshold: 3,
	}

	resp, err := a.client.Sys().InitWithContext(ctx, req)
	if err != nil {
		return nil, err
	}

	r := &unseal.VaultInitData{
		UnsealKeysB64:     resp.KeysB64,
		UnsealKeysHex:     resp.Keys,
		UnsealShares:      req.SecretShares,
		UnsealThreshold:   req.SecretThreshold,
		RecoveryKeysB64:   resp.RecoveryKeysB64,
		RecoveryKeysHex:   resp.RecoveryKeys,
		RecoveryShares:    req.RecoveryShares,
		RecoveryThreshold: req.RecoveryThreshold,
		RootToken:         resp.RootToken,
	}

	return r, nil
}

func (a *Api) RawInitData(ctx context.Context) ([]byte, error) {
	r, err := a.Init(ctx)
	if err != nil {
		return nil, err
	}

	return json.Marshal(r)
}

func (a *Api) Status(ctx context.Context) (*unseal.SealStatusResponse, error) {
	r, err := a.client.Sys().SealStatusWithContext(ctx)
	if err != nil {
		return nil, err
	}

	return (*unseal.SealStatusResponse)(r), nil
}

func (a *Api) Unseal(ctx context.Context, keys []string) error {
	for _, key := range keys {
		resp, err := a.client.Sys().UnsealWithContext(ctx, key)
		if err != nil {
			return err
		}

		if !resp.Sealed {
			return nil
		}
	}

	return fmt.Errorf("vault unseal failed")
}

func (a *Api) EnsureKV2Mounts(ctx context.Context, token string, mountPoints []string) error {
	if len(mountPoints) == 0 {
		return nil
	}

	a.client.SetToken(token)
	mounts, err := a.client.Sys().ListMountsWithContext(ctx)
	if err != nil {
		return fmt.Errorf("secrets list: %w", err)
	}

	for _, mountPoint := range mountPoints {
		if _, ok := mounts[strings.TrimSuffix(mountPoint, "/")+"/"]; ok {
			continue
		}

		input := &api.MountInput{
			Type:    "kv",
			Options: map[string]string{"version": "2"},
		}
		if err = a.client.Sys().MountWithContext(ctx, mountPoint, input); err != nil {
			return fmt.Errorf("enable kv-v2 mount %q: %w", mountPoint, err)
		}
	}

	return nil
}
