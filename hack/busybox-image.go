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

package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/arenadata/adcm-installer/pkg/registry-client"

	"github.com/distribution/reference"
	"github.com/docker/docker/image"
	v1 "github.com/docker/docker/image/v1"
	"github.com/docker/docker/pkg/system"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	log "github.com/sirupsen/logrus"
)

const (
	ociLayout        = `{"imageLayoutVersion": "1.0.0"}`
	manifestFileName = "manifest.json"

	dockerHost           = "docker.io"
	dockerRegistryDomain = "registry-1.docker.io"
)

type manifestItem struct {
	Config       string
	RepoTags     []string
	Layers       []string
	Parent       image.ID                             `json:",omitempty"`
	LayerSources map[digest.Digest]ocispec.Descriptor `json:",omitempty"`
}

func main() {
	if len(os.Args) < 3 {
		log.Fatalf("Missing image destination file")
	}

	if _, err := os.Stat(os.Args[2]); err == nil {
		log.Infof("Skipping. File %s already exists", os.Args[1])
		return
	}

	pull := os.Args[1]
	tarFile := os.Args[2]

	distributionRef, err := reference.ParseNormalizedNamed(pull)
	if err != nil {
		log.Fatal(err)
	}

	domain := reference.Domain(distributionRef)
	if domain == dockerHost {
		domain = dockerRegistryDomain
	}

	repo := reference.Path(distributionRef)
	ref := reference.TagNameOnly(distributionRef)
	tag := ref.(reference.Tagged).Tag()

	reg := client.NewRegistryClient(domain)

	indexBytes, err := reg.Manifest(repo, tag, ocispec.MediaTypeImageIndex)
	if err != nil {
		log.Fatal(err)
	}

	var manifestIndex *ocispec.Index
	if err = unmarshalStrict(indexBytes, &manifestIndex); err != nil {
		log.Fatal(err)
	}

	indexFile := &ocispec.Index{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageIndex,
	}

	var img *image.Image
	var imgDigest digest.Digest
	var manifest *ocispec.Manifest
	var manifestBytes []byte
	for _, item := range manifestIndex.Manifests {
		if item.Platform.OS == "linux" && item.Platform.Architecture == "amd64" {
			imgDigest = item.Digest
			manifestBytes, err = reg.Manifest(repo, imgDigest.String(), item.MediaType)
			if err != nil {
				log.Fatal(err)
			}

			if err = unmarshalStrict(manifestBytes, &manifest); err != nil {
				log.Fatal(err)
			}

			indexFile.Manifests = append(indexFile.Manifests, item)
			break
		}
	}

	if manifest == nil {
		log.Fatal("manifest not found")
	}

	blobsDir := path.Join(ocispec.ImageBlobsDir, imgDigest.Algorithm().String())

	tmpDir := newOutputWriter()
	defer func() {
		if e := tmpDir.Rm(); e != nil {
			log.Fatal(e)
		}
		if err != nil {
			log.Fatal(err)
		}
	}()

	epochStart := time.Unix(0, 0)
	if err = tmpDir.WriteFile(ocispec.ImageLayoutFile, []byte(ociLayout), 0640, epochStart); err != nil {
		return
	}

	if err = tmpDir.WriteFile(imgDigest, manifestBytes, 0640, epochStart); err != nil {
		return
	}

	var b []byte
	b, err = json.Marshal(indexFile)
	if err != nil {
		return
	}

	if err = tmpDir.WriteFile(ocispec.ImageIndexFile, b, 0640, epochStart); err != nil {
		return
	}

	var configBytes []byte
	configBytes, err = reg.Blob(repo, manifest.Config.Digest.String(), manifest.Config.MediaType)
	if err != nil {
		return
	}

	if err = unmarshalStrict(configBytes, &img); err != nil {
		return
	}

	mfstItem := manifestItem{
		RepoTags:     []string{pull},
		LayerSources: make(map[digest.Digest]ocispec.Descriptor),
		Config:       path.Join(blobsDir, manifest.Config.Digest.Encoded()),
	}

	ts := epochStart
	if img.Created != nil {
		ts = *img.Created
	}

	if err = tmpDir.WriteFile(manifest.Config.Digest, configBytes, 0640, ts); err != nil {
		return
	}

	var parent digest.Digest
	var layers []ocispec.Descriptor
	for i, layerID := range manifest.Layers {
		v1ImgCreated := time.Unix(0, 0)
		v1Img := image.V1Image{
			Created: &v1ImgCreated,
			OS:      img.OS,
		}
		if i == len(img.RootFS.DiffIDs)-1 {
			v1Img = img.V1Image
		}
		rootFS := *img.RootFS
		rootFS.DiffIDs = rootFS.DiffIDs[:i+1]

		var v1ID digest.Digest
		v1ID, err = v1.CreateID(v1Img, rootFS.ChainID(), parent)
		if err != nil {
			return
		}

		v1Img.ID = v1ID.Encoded()
		if parent != "" {
			v1Img.Parent = parent.Encoded()
		}

		var imageConfig []byte
		imageConfig, err = json.Marshal(v1Img)
		if err != nil {
			return
		}

		cfgDgst := digest.FromBytes(imageConfig)
		if err = tmpDir.WriteFile(cfgDgst, imageConfig, 0640, ts); err != nil {
			return
		}

		var layerBytes []byte
		layerBytes, err = reg.Blob(repo, layerID.Digest.String(), manifest.Layers[i].MediaType)
		if err != nil {
			return
		}

		if err = tmpDir.WriteFile(layerID.Digest, layerBytes, 0640, ts); err != nil {
			return
		}

		layers = append(layers, layerID)
		parent = v1ID

		mfstItem.Layers = append(mfstItem.Layers, path.Join(blobsDir, layerID.Digest.Encoded()))
		mfstItem.LayerSources[manifest.Layers[i].Digest] = manifest.Layers[i]
	}

	b, err = json.Marshal([]manifestItem{mfstItem})
	if err != nil {
		return
	}

	if err = tmpDir.WriteFile(manifestFileName, b, 0640, epochStart); err != nil {
		return
	}

	if err = tmpDir.Tar(tarFile); err != nil {
		return
	}
}

func unmarshalStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewBuffer(data))
	dec.DisallowUnknownFields()
	return dec.Decode(&v)
}

func newOutputWriter() *outputWriter {
	return &outputWriter{dir: "_tmp/busybox-image"}
}

type outputWriter struct {
	dir string
}

func (ow outputWriter) Rm() error {
	return os.RemoveAll(ow.dir)
}

func (ow outputWriter) WriteFile(filename any, data []byte, perm os.FileMode, ts time.Time) error {
	var file string
	switch fn := filename.(type) {
	case string:
		file = filepath.Join(ow.dir, fn)
	case digest.Digest:
		file = filepath.Join(ow.dir, ocispec.ImageBlobsDir, fn.Algorithm().String(), fn.Encoded())
	default:
		return fmt.Errorf("unknown filename type: %T", fn)
	}

	dir := filepath.Dir(file)
	if fi, err := os.Stat(dir); err != nil {
		if err = os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	} else if !fi.IsDir() {
		return fmt.Errorf("%q is not a directory", dir)
	}

	if err := os.WriteFile(file, data, perm); err != nil {
		return err
	}

	return system.Chtimes(file, ts, ts)
}

func (ow outputWriter) Tar(output string) (err error) {
	ts := time.Unix(0, 0)
	err = filepath.WalkDir(ow.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == ow.dir || !d.IsDir() {
			return err
		}

		return system.Chtimes(path, ts, ts)
	})
	if err != nil {
		return
	}

	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer func() {
		if e := tw.Close(); e != nil {
			err = e
		}
	}()

	pfx := ow.dir
	if pfx[len(pfx)-1] != '/' {
		pfx += "/"
	}

	err = filepath.Walk(ow.dir, func(filePath string, info fs.FileInfo, err error) error {
		if err != nil || filePath == ow.dir {
			return err
		}

		relPath := strings.TrimPrefix(filePath, pfx)
		hdr, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return fmt.Errorf("tar.FileInfoHeader: %s", err)
		}
		hdr.Name = relPath
		hdr.Uid = 0
		hdr.Gid = 0
		hdr.Uname = ""
		hdr.Gname = ""

		if err = tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("tar write header failed: %s", err)
		}

		if info.IsDir() {
			return nil
		}

		b, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		_, err = tw.Write(b)
		return err
	})
	if err != nil {
		return fmt.Errorf("walk dir failed: %s", err)
	}

	if err = tw.Flush(); err != nil {
		return fmt.Errorf("tar flush failed: %s", err)
	}

	return os.WriteFile(output, buf.Bytes(), 0600)
}
