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

func LoadBusybox(ctx context.Context) error {
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
