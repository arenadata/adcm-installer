package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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

func secretsShow(cmd *cobra.Command, _ []string) {
	logger := log.WithField("command", "secrets-show")

	configFilePath, _ := cmd.Flags().GetString("file")
	prj, err := readConfigFile(configFilePath)
	if err != nil {
		logger.Fatal(err)
	}

	dec, _, err := readOrCreateNewAgeKey(cmd, "age-key")
	if err != nil {
		logger.Fatal(err)
	}

	out := map[string]string{}
	if xSecrets, ok := prj.Extensions[xsecretsKey].(*xsecrets); ok {
		if xSecrets.AgeRecipient != dec.Recipient().String() {
			logger.Fatal("age_recipient not match")
		}

		for k, v := range xSecrets.Data {
			v, err = dec.DecryptValue(v)
			if err != nil {
				logger.Fatal(err)
			}
			out[k] = v
		}
	}

	if err = toYaml(cmd.OutOrStdout(), out); err != nil {
		logger.Fatal(err)
	}
}
