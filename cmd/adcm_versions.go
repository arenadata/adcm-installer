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

package cmd

import (
	"github.com/arenadata/adcm-installer/internal/services"
	"github.com/arenadata/adcm-installer/pkg/registry-client"

	"github.com/blang/semver/v4"
	"github.com/distribution/reference"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var listVersionsCmd = &cobra.Command{
	Use:   "adcm-versions",
	Short: "List versions of Arenadata products",
	Long: `Will list the 5 latest ADCM versions on hub.arenadata.io in semver format,
sorted in descending order.
- --all removes the limitation on the last 5 versions and displays all
        available versions`,
	Run: listVersions,
}

func init() {
	rootCmd.AddCommand(listVersionsCmd)
	listVersionsCmd.Flags().BoolP("all", "a", false, "List all versions")
}

func listVersions(cmd *cobra.Command, _ []string) {
	logger := log.WithField("command", "adcm-versions")

	distributionRef, err := reference.ParseNormalizedNamed(services.ADCMImage)
	if err != nil {
		logger.Fatal(err)
	}

	domain := reference.Domain(distributionRef)
	reg := client.NewRegistryClient(domain)

	tags, err := reg.Tags(reference.Path(distributionRef))
	if err != nil {
		logger.Fatal(err)
	}

	var versions []semver.Version
	for _, tag := range tags {
		ver, err := semver.Parse(tag)
		if err == nil {
			versions = append(versions, ver)
		}
	}

	semver.Sort(versions)

	i := len(versions) - 1
	end := 0
	all, _ := cmd.Flags().GetBool("all")
	if !all {
		end = i - 4
		if end < 0 {
			end = 0
		}
	}

	for ; i >= end; i-- {
		cmd.Println(versions[i].String())
	}
}
