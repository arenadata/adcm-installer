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

package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/arenadata/adcm-installer/pkg/vault/unseal"
)

type Host struct {
	bin string
}

func New() (unseal.Runner, error) {
	bin, err := lookupBinPath()
	if err != nil {
		return nil, err
	}

	return &Host{bin: bin}, nil
}

func lookupBinPath() (string, error) {
	for _, bin := range []string{"vault", "bao"} {
		if s, err := exec.LookPath(bin); err == nil {
			return s, nil
		}
	}

	return "", fmt.Errorf("vault/bao executable not found in $PATH")
}

func (h *Host) cmd(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, h.bin, args...)
	cmd.Env = append(os.Environ(), unseal.EnvFormatJson)
	return cmd
}

func (h *Host) unmarshalStatus(b []byte) (*unseal.SealStatusResponse, error) {
	var status *unseal.SealStatusResponse
	if err := json.Unmarshal(b, &status); err != nil {
		return nil, fmt.Errorf("status: %v", err)
	}
	return status, nil
}

func (h *Host) Status(ctx context.Context) (*unseal.SealStatusResponse, error) {
	resp, err := h.cmd(ctx, "status").Output()
	if err != nil {
		var e *exec.ExitError
		if errors.As(err, &e) && e.ExitCode() != 1 && e.ExitCode() != 2 {
			errStr := err.Error()
			if len(errStr) == 0 {
				errStr = string(resp)
			}
			return nil, fmt.Errorf("call command failed: %s", errStr)
		}
	}

	return h.unmarshalStatus(resp)
}

func (h *Host) RawInitData(ctx context.Context) ([]byte, error) {
	resp, err := h.cmd(ctx, "operator", "init").Output()
	if err != nil {
		return nil, fmt.Errorf("call command failed: %v", err)
	}
	return resp, nil
}

func (h *Host) Init(ctx context.Context) (*unseal.VaultInitData, error) {
	resp, err := h.RawInitData(ctx)
	if err != nil {
		return nil, err
	}

	var init *unseal.VaultInitData
	if err = json.Unmarshal(resp, &init); err != nil {
		return nil, err
	}
	return init, nil
}

func (h *Host) Unseal(ctx context.Context, keys []string) error {
	for _, key := range keys {
		resp, err := h.cmd(ctx, "operator", "unseal", key).Output()
		if err != nil {
			return fmt.Errorf("call command failed: %v", err)
		}

		status, err := h.unmarshalStatus(resp)
		if err != nil {
			return err
		}

		if !status.Sealed {
			return nil
		}
	}

	return fmt.Errorf("vault unseal failed")
}
