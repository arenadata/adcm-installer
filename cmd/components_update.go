package cmd

import (
	"fmt"
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
	Use:     "updates",
	Short:   "Ensure that the latest version of all installed components is installed",
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.WithField("command", "components-update")

		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Timeout: 5 * time.Second,
		}

		const link = "https://github.com/arenadata/adcm/releases/latest"

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

		lastVersion, err := semver.NewVersion(path.Base(u.Path))
		if err != nil {
			logger.Fatal(err)
		}

		currentVersion, err := semver.NewVersion(version)
		if err != nil {
			logger.Fatal(err)
		}

		if lastVersion.GreaterThan(currentVersion) {
			fmt.Printf(`There is a new version of adcm-installer %q available. Current version: %q.
You can download the latest version: %s
`, lastVersion, currentVersion, link)
			return
		}

		fmt.Println("Already up to date.")
	},
}

func init() {
	componentsCmd.AddCommand(componentsUpdateCmd)
}
