package cmd

import (
	"net/http"
	"os"

	"github.com/arenadata/adcm-installer/models"
	"github.com/arenadata/adcm-installer/utils"

	"github.com/blang/semver/v4"
	"github.com/heroku/docker-registry-client/registry"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// versionsCmd represents the versions command
var versionsCmd = &cobra.Command{
	Aliases: []string{"v"},
	Use:     "versions",
	Short:   "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.WithField("command", "list-versions")

		configFile, _ := cmd.Flags().GetString("config")
		conf, err := getAdcmConfig(configFile)
		if err != nil {
			logger.Fatal(err)
		}

		u := "https://" + *conf.Image.Registry
		transport := registry.WrapTransport(http.DefaultTransport, u, "", "")
		reg := &registry.Registry{
			URL: u,
			Client: &http.Client{
				Transport: transport,
			},
			Logf: logger.Debugf,
		}

		tags, err := reg.Tags(*conf.Image.Name)
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
	},
}

func init() {
	listCmd.AddCommand(versionsCmd)
	versionsCmd.Flags().StringP("config", "c", models.ADCMConfigFile, "Path to configuration file")
	versionsCmd.Flags().BoolP("all", "a", false, "List all versions")
}

func getAdcmConfig(path string) (*models.ADCMConfig, error) {
	defaultRegistry := models.ADImageRegistry
	var imap map[string]any
	config := &models.ADCMConfig{}

	isConfigFileExists, err := utils.FileExists(path)
	if err != nil {
		return nil, err
	}

	if !isConfigFileExists {
		models.SetDefaultsADCMConfig(config, &defaultRegistry)
		return config, nil
	}

	fi, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = fi.Close() }()

	if err = yaml.NewDecoder(fi).Decode(&imap); err != nil {
		return nil, err
	}

	reg, ok := imap["registry"]
	if ok {
		defaultRegistry = reg.(string)
	}

	adcm, ok := imap["adcm"]
	if !ok {
		models.SetDefaultsADCMConfig(config, &defaultRegistry)
		return config, nil
	}

	b, err := yaml.Marshal(adcm)
	if err != nil {
		return nil, err
	}

	if err = yaml.Unmarshal(b, &config); err != nil {
		return nil, err
	}

	models.SetDefaultsADCMConfig(config, &defaultRegistry)

	return config, nil
}
