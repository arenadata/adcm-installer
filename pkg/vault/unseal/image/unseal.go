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

package image

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/docker/cli/cli"
	"strings"

	"github.com/arenadata/adcm-installer/pkg/vault/unseal"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/command/container"
	cliflags "github.com/docker/cli/cli/flags"
)

type ContainerOption func(*Container)

type Container struct {
	name string

	opts container.ExecOptions
	cli  *command.DockerCli
	buf  *bytes.Buffer
	bin  string
}

func New(name string, opts ...ContainerOption) (unseal.Runner, error) {
	c := &Container{
		name: name,
		buf:  new(bytes.Buffer),
	}

	for _, opt := range opts {
		opt(c)
	}

	execOpts := container.NewExecOptions()
	_ = execOpts.Env.Set(unseal.EnvFormatJson)
	c.opts = execOpts

	if c.cli == nil {
		cli, err := command.NewDockerCli(command.WithOutputStream(c.buf))
		if err != nil {
			return nil, err
		}
		if err = cli.Initialize(&cliflags.ClientOptions{}); err != nil {
			return nil, err
		}
		c.cli = cli
	}

	if len(c.bin) == 0 {
		if err := c.lookupBinPath(); err != nil {
			return nil, err
		}
	}

	return c, nil
}

func (c *Container) lookupBinPath() error {
	defer c.buf.Reset()

	for _, bin := range []string{"vault", "bao"} {
		c.opts.Command = []string{"which", bin}

		err := container.RunExec(context.Background(), c.cli, c.name, c.opts)
		if err == nil {
			c.bin = strings.TrimSpace(c.buf.String())
			return nil
		}
	}

	return fmt.Errorf("vault/bao executable not found in container %s", c.name)
}

func (c *Container) unmarshal(v any) error {
	defer c.buf.Reset()
	return json.NewDecoder(c.buf).Decode(v)
}

func (c *Container) unmarshalStatus() (*unseal.SealStatusResponse, error) {
	defer c.buf.Reset()

	var status *unseal.SealStatusResponse
	if err := c.unmarshal(&status); err != nil {
		return nil, fmt.Errorf("status: %v", err)
	}
	return status, nil
}

func (c *Container) Status(ctx context.Context) (*unseal.SealStatusResponse, error) {
	c.opts.Command = []string{c.bin, "status"}
	err := container.RunExec(ctx, c.cli, c.name, c.opts)
	if err != nil {
		var e cli.StatusError
		if errors.As(err, &e) && e.StatusCode != 1 && e.StatusCode != 2 {
			errStr := err.Error()
			if len(errStr) == 0 {
				errStr = c.buf.String()
				c.buf.Reset()
			}
			return nil, fmt.Errorf("status: call command failed: %s", errStr)
		}
	}

	return c.unmarshalStatus()
}

func (c *Container) RawInitData(ctx context.Context) ([]byte, error) {
	defer c.buf.Reset()
	if err := c.init(ctx); err != nil {
		return nil, err
	}
	return c.buf.Bytes(), nil
}

func (c *Container) init(ctx context.Context) error {
	c.opts.Command = []string{c.bin, "operator", "init"}
	err := container.RunExec(ctx, c.cli, c.name, c.opts)
	if err != nil {
		return fmt.Errorf("init: call command failed: %v", err)
	}
	return nil
}

func (c *Container) Init(ctx context.Context) (*unseal.VaultInitData, error) {
	if err := c.init(ctx); err != nil {
		return nil, err
	}

	var unsealData *unseal.VaultInitData
	if err := c.unmarshal(&unsealData); err != nil {
		return nil, err
	}
	return unsealData, nil
}

func (c *Container) Unseal(ctx context.Context, keys []string) error {
	for _, key := range keys {
		c.opts.Command = []string{c.bin, "operator", "unseal", key}
		err := container.RunExec(ctx, c.cli, c.name, c.opts)
		if err != nil {
			return fmt.Errorf("unseal: call command failed: %v", err)
		}

		status, err := c.unmarshalStatus()
		if err != nil {
			return err
		}

		if !status.Sealed {
			return nil
		}
	}

	return fmt.Errorf("vault unseal failed")
}
