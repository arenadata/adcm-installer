package assets

import (
	"bytes"
	"compress/gzip"
	"context"
	_ "embed"

	"github.com/docker/docker/client"
)

//go:embed busybox.tar.gz
var busybox []byte

const ImageName = "busybox:stable-uclibc"

func LoadBusybox(ctx context.Context) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()

	r, err := gzip.NewReader(bytes.NewBuffer(busybox))
	if err != nil {
		return err
	}
	defer r.Close()

	resp, err := cli.ImageLoad(ctx, r, true)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}
