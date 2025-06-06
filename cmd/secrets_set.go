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
	"bytes"
	"os"

	"github.com/arenadata/adcm-installer/pkg/secrets"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set or update a x-secret value",
	Long: `Allows you to change or add a secret to the configuration file.
- --age-key takes the value of the private key in plain text. Has priority over
            --age-key-file
- --age-key-file takes the value of the path to the file with the private key
- --file specifies the path to the configuration file`,
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
