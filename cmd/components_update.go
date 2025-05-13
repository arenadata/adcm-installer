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
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/Masterminds/semver/v3"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var componentsUpdateCmd = &cobra.Command{
	Aliases: []string{"u"},
	Use:     "update",
	Short:   "Ensure that the latest version of all installed components is installed",
	Run:     componentsUpdate,
}

func init() {
	componentsCmd.AddCommand(componentsUpdateCmd)
}

func componentsUpdate(cmd *cobra.Command, _ []string) {
	logger := log.WithField("command", "components-update")

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 5 * time.Second,
	}

	const link = "https://github.com/arenadata/adcm-installer/releases/latest"

	resp, err := client.Get(link)
	if err != nil {
		logger.Fatal(err)
	}

	loc := resp.Header.Get("Location")
	if len(loc) == 0 {
		return
	}

	u, err := url.Parse(loc)
	if err != nil {
		logger.Fatal(err)
	}

	ver := path.Base(u.Path)
	if ver == "releases" {
		cmd.Println("ADCM Installer has no new releases")
		return
	}

	lastVersion, err := semver.NewVersion(ver)
	if err != nil {
		logger.Fatalf("%s: %s", err, ver)
	}

	currentVersion, err := semver.NewVersion(version)
	if err != nil {
		logger.Fatal("%s: %s", err, version)
	}

	if lastVersion.GreaterThan(currentVersion) {
		cmd.Printf(`There is a new version of adcm-installer %q available. Current version: %q.
You can download the latest version: %s
`, lastVersion, currentVersion, link)
		return
	}

	cmd.Println("Already up to date.")
}
