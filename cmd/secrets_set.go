package cmd

import (
	"bytes"
	"github.com/arenadata/adcm-installer/pkg/secrets"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:     "set <key> <value>",
	Short:   "Set or update a x-secret value",
	PreRunE: cobra.ExactArgs(2),
	Run:     secretSetValue,
}

func init() {
	secretsCmd.AddCommand(setCmd)

	ageKeyFlags(setCmd, "age-key", ageKeyFileName)
	configFileFlags(setCmd)
	/*
		--interactive, -i
	*/
}

func secretSetValue(cmd *cobra.Command, args []string) {
	logger := log.WithField("command", "secrets-set")

	configFilePath, _ := cmd.Flags().GetString("file")
	prj, err := readConfigFile(configFilePath)
	if err != nil {
		logger.Fatal(err)
	}
	if len(configFilePath) == 0 {
		configFilePath = prj.ComposeFiles[0]
	}

	var enc *secrets.AgeCrypt
	enc, _, err = readOrCreateNewAgeKey(cmd, "age-key")
	if err != nil {
		logger.Fatal(err)
	}

	xSecrets, ok := prj.Extensions[xsecretsKey].(*xsecrets)
	if !ok {
		logger.Fatal("xsecrets extension not match")
	}

	if xSecrets.AgeRecipient != enc.Recipient().String() {
		logger.Fatal("age_recipient not match")
	}

	val, err := enc.EncryptValue(args[1])
	if err != nil {
		logger.Fatal(err)
	}

	xSecrets.Data[args[0]] = val

	buf := new(bytes.Buffer)
	if err = toYaml(buf, prj); err != nil {
		return
	}

	if err = os.WriteFile(configFilePath, buf.Bytes(), 0640); err != nil {
		logger.Fatal(err)
	}
}
