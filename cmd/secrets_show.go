package cmd

import (
	"os"
	"strings"

	"github.com/arenadata/arenadata-installer/apis/app/v1alpha1"
	"github.com/arenadata/arenadata-installer/internal/runtime"
	"github.com/arenadata/arenadata-installer/pkg/secrets"
	"github.com/arenadata/arenadata-installer/pkg/utils"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show application secrets",
	Run:   secretsShow,
}

func init() {
	secretsCmd.AddCommand(showCmd)

	ageKeyFlags(showCmd, "age-key", ageKeyFileName)
	configFileFlags(showCmd)
}

type secretData struct {
	Name            string `yaml:"name"`
	secrets.Secrets `yaml:",inline"`
}

func secretsShow(cmd *cobra.Command, _ []string) {
	logger := log.WithField("command", "secrets-show")

	configFilePath, _ := cmd.Flags().GetString("file")
	if len(configFilePath) == 0 {
		logger.Fatal("config file not provided")
	}

	ageKey, err := readOrCreateNewAgeKey(cmd, "age-key")
	if err != nil {
		logger.Fatal(err)
	}

	docs, err := utils.SplitYamlFile(configFilePath)
	if err != nil {
		logger.Fatal(err)
	}

	enc := yaml.NewEncoder(os.Stdout)
	defer func() { _ = enc.Close() }()
	enc.SetIndent(2)

	for _, doc := range docs {
		obj, err := runtime.Decode(doc)
		if err != nil {
			logger.Fatal(err)
		}
		app, ok := obj.(*v1alpha1.Application)
		if !ok {
			logger.Fatalf("object is not Application type: %s", doc)
		}

		if v := app.Annotations[v1alpha1.SecretsAgeKey]; len(v) > 0 {
			sec, err := secrets.DecryptData(ageKey, v, app.Annotations[v1alpha1.SecretsAgeRecipientKey])
			if err != nil {
				logger.Fatal(err)
			}

			if err = enc.Encode(secretData{
				Name:    strings.ToLower(app.Kind) + "." + app.Name,
				Secrets: *sec,
			}); err != nil {
				logger.Fatal(err)
			}
		}
	}
}
