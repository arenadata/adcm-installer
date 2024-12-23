package cmd

import (
	"os"
	"strings"

	"github.com/arenadata/adcm-installer/compose"
	"github.com/arenadata/adcm-installer/crypt"
	"github.com/arenadata/adcm-installer/models"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// upCmd represents the install command
var upCmd = &cobra.Command{
	Use:   "up",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		ageKey, _ := cmd.Flags().GetString("age-key")

		ageKeyFile, _ := cmd.Flags().GetString("age-key-file")
		if _, err := os.Stat(ageKeyFile); err == nil && len(ageKey) == 0 {
			log.Debugf("Using AGE file: %s", ageKeyFile)
			b, err := os.ReadFile(ageKeyFile)
			if err != nil {
				log.Fatal(err)
			}
			for _, line := range strings.Split(string(b), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "#") {
					continue
				}
				ageKey = strings.TrimSpace(line)
				break
			}
		}

		ageCrypt, err := crypt.New(ageKey)
		if err != nil {
			log.Fatal(err)
		}

		config := &models.Config{
			Secrets: models.NewSecrets(ageCrypt),
		}
		configFile, _ := cmd.Flags().GetString("config")
		if _, err = os.Stat(configFile); err == nil {
			log.Debugf("Using config file: %s", configFile)
			fi, err := os.Open(configFile)
			if err != nil {
				log.Fatal(err)
			}
			defer func() { _ = fi.Close() }()

			dec := yaml.NewDecoder(fi)
			dec.KnownFields(true)
			if err = dec.Decode(&config); err != nil {
				log.Fatal(err)
			}
		}

		models.SetDefaultSecrets(config.Secrets)
		models.SetDefaultsConfig(config)

		prj, err := compose.NewProject(compose.ProjectName, config)
		if err != nil {
			log.Fatal(err)
		}

		comp, err := compose.NewComposeService()
		if err != nil {
			log.Fatal(err)
		}

		if err = comp.Up(cmd.Context(), prj); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(upCmd)
	upCmd.Flags().String("age-key", "", "config file path")
	upCmd.Flags().String("age-key-file", models.AGEKeyFile, "config file path")
	upCmd.MarkFlagsMutuallyExclusive("age-key", "age-key-file")
}
