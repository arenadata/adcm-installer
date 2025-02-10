package cmd

import (
	"net/http"

	"github.com/arenadata/arenadata-installer/pkg/compose"

	"github.com/blang/semver/v4"
	"github.com/heroku/docker-registry-client/registry"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var listVersionsCmd = &cobra.Command{
	Aliases: []string{"v"},
	Use:     "versions",
	Short:   "List versions of Arenadata products",
	Run:     listVersions,
}

func init() {
	listCmd.AddCommand(listVersionsCmd)
	listVersionsCmd.Flags().BoolP("all", "a", false, "List all versions")

	listVersionsCmd.Flags().Bool("adcm", false, "List ADCM versions")
	listVersionsCmd.MarkFlagsOneRequired("adcm")
	//listVersionsCmd.MarkFlagsMutuallyExclusive("adcm")
}

func listVersions(cmd *cobra.Command, _ []string) {
	logger := log.WithField("command", "list-versions")

	u := "https://" + compose.ADImageRegistry
	transport := registry.WrapTransport(http.DefaultTransport, u, "", "")
	reg := &registry.Registry{
		URL: u,
		Client: &http.Client{
			Transport: transport,
		},
		Logf: logger.Debugf,
	}

	var image string
	if getBool(cmd, "adcm") {
		image = compose.ADCMImageName
	}

	tags, err := reg.Tags(image)
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
