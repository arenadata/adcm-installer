package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	imageCopy "github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	log "github.com/sirupsen/logrus"
)

const (
	defaultPolicy = `{
    "default": [{"type": "insecureAcceptAnything"}],
    "transports": {"docker-daemon": {"": [{"type":"insecureAcceptAnything"}]}}
}`
	imageDstPattern = "docker-archive:%s:busybox:stable-uclibc"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Missing image destination file")
	}

	if _, err := os.Stat(os.Args[1]); err == nil {
		log.Infof("Skipping. File %s already exists", os.Args[1])
		return
	}

	var policy *signature.Policy
	if err := json.Unmarshal([]byte(defaultPolicy), &policy); err != nil {
		log.Fatal(err)
	}

	policyContext, err := signature.NewPolicyContext(policy)
	if err != nil {
		log.Fatal(err)
	}

	srcRef, err := alltransports.ParseImageName("docker://docker.io/library/busybox:stable-uclibc")
	if err != nil {
		log.Fatal(err)
	}

	dstRef, err := alltransports.ParseImageName(fmt.Sprintf(imageDstPattern, os.Args[1]))
	if err != nil {
		log.Fatal(err)
	}

	srcSysCtx := &types.SystemContext{
		ArchitectureChoice: "amd64",
		OSChoice:           "linux",
	}

	ctx := context.Background()
	_, err = imageCopy.Image(ctx, policyContext, dstRef, srcRef, &imageCopy.Options{
		SourceCtx:            srcSysCtx,
		ReportWriter:         os.Stdout,
		MaxParallelDownloads: 3,
	})
	if err != nil {
		log.Fatal(err)
	}
}
