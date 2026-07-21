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

package services

import (
	"fmt"

	client "github.com/arenadata/adcm-installer/pkg/registry-client"

	"github.com/blang/semver/v4"
	"github.com/distribution/reference"
)

// RegistryVersions fetches the semver-parsed tags for the given image reference
// from its registry, sorted in ascending order. Tags that are not valid semver
// are ignored.
func RegistryVersions(image string) ([]semver.Version, error) {
	distributionRef, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return nil, err
	}

	domain := reference.Domain(distributionRef)
	reg := client.NewRegistryClient(domain)

	tags, err := reg.Tags(reference.Path(distributionRef))
	if err != nil {
		return nil, err
	}

	var versions []semver.Version
	for _, tag := range tags {
		ver, err := semver.Parse(tag)
		if err == nil {
			versions = append(versions, ver)
		}
	}

	semver.Sort(versions)

	return versions, nil
}

// LatestReleasedTag returns the highest released (non-prerelease) semver tag
// available for the image in its registry.
func LatestReleasedTag(image string) (string, error) {
	versions, err := RegistryVersions(image)
	if err != nil {
		return "", err
	}

	for i := len(versions) - 1; i >= 0; i-- {
		if len(versions[i].Pre) == 0 {
			return versions[i].String(), nil
		}
	}

	return "", fmt.Errorf("no released versions found for %q", image)
}
