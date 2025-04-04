package cmd

import (
	"github.com/arenadata/adcm-installer/pkg/compose"
	"github.com/arenadata/adcm-installer/pkg/registry-client"

	"github.com/blang/semver/v4"
	"github.com/distribution/reference"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var listVersionsCmd = &cobra.Command{
	Use:   "adcm-versions",
	Short: "List versions of Arenadata products",
	Run:   listVersions,
}

func init() {
	rootCmd.AddCommand(listVersionsCmd)
	listVersionsCmd.Flags().BoolP("all", "a", false, "List all versions")
}

func listVersions(cmd *cobra.Command, _ []string) {
	logger := log.WithField("command", "adcm-versions")

	distributionRef, err := reference.ParseNormalizedNamed(compose.ADCMImage)
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
