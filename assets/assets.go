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

package assets

import (
	"bytes"
	"context"
	_ "embed"

	"github.com/docker/docker/client"
)

//go:embed busybox.tar
var busybox []byte

const ImageName = "busybox:stable-uclibc"

func LoadBusyboxImage(ctx context.Context) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()

	resp, err := cli.ImageLoad(ctx, bytes.NewBuffer(busybox), true)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}
