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

package unseal

import (
	"context"
	"errors"
)

const EnvFormatJson = "VAULT_FORMAT=json"

var ErrVaultIsAlreadyInitialized = errors.New("vault is already initialized")

type VaultInitData struct {
	UnsealKeysB64     []string `json:"unseal_keys_b64"`
	UnsealKeysHex     []string `json:"unseal_keys_hex"`
	UnsealShares      int      `json:"unseal_shares"`
	UnsealThreshold   int      `json:"unseal_threshold"`
	RecoveryKeysB64   []string `json:"recovery_keys_b64"`
	RecoveryKeysHex   []string `json:"recovery_keys_hex"`
	RecoveryShares    int      `json:"recovery_keys_shares"`
	RecoveryThreshold int      `json:"recovery_keys_threshold"`
	RootToken         string   `json:"root_token"`
}

type SealStatusResponse struct {
	Type         string   `json:"type"`
	Initialized  bool     `json:"initialized"`
	Sealed       bool     `json:"sealed"`
	T            int      `json:"t"`
	N            int      `json:"n"`
	Progress     int      `json:"progress"`
	Nonce        string   `json:"nonce"`
	Version      string   `json:"version"`
	BuildDate    string   `json:"build_date"`
	Migration    bool     `json:"migration"`
	ClusterName  string   `json:"cluster_name,omitempty"`
	ClusterID    string   `json:"cluster_id,omitempty"`
	RecoverySeal bool     `json:"recovery_seal"`
	StorageType  string   `json:"storage_type,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
}

type Runner interface {
	Init(context.Context) (*VaultInitData, error)
	RawInitData(ctx context.Context) ([]byte, error)
	Status(context.Context) (*SealStatusResponse, error)
	Unseal(context.Context, []string) error
}
