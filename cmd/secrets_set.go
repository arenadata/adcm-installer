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
	"strings"

	"github.com/arenadata/adcm-installer/internal/services"
	"github.com/arenadata/adcm-installer/internal/services/helpers"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set <service-name>.<key> <value>",
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

	value := args[1]
	pathKeyParts := strings.Split(args[0], ".")
	if len(pathKeyParts) != 2 {
		logger.Fatalf("Invalid key format: %s", args[0])
	}
	svcName, secKey := pathKeyParts[0], pathKeyParts[1]

	svc, ok := prj.Services[svcName]
	if !ok {
		logger.Fatalf("Service %s not found", svcName)
	}

	aes, err := encoder(cmd, prj)
	if err != nil {
		logger.Fatal(err)
	}
	if aes != nil {
		value, err = aes.EncryptValue(value)
		if err != nil {
			logger.Fatal(err)
		}
	}

	ext, ok := svc.Extensions[services.XSecretsKey]
	svcExtension := &services.XSecrets{}
	if ok {
		svcExtension = ext.(*services.XSecrets)
	}
	svcExtension.Data[secKey] = value

	servicesModHelpers := helpers.NewModHelpers()
	servicesModHelpers = append(servicesModHelpers, helpers.Extension(svcName, services.XSecretsKey, svcExtension))
	if err = servicesModHelpers.Apply(prj); err != nil {
		logger.Fatal(err)
	}

	buf := new(bytes.Buffer)
	if err = toYaml(buf, prj); err != nil {
		return
	}

	if err = os.WriteFile(configFilePath, buf.Bytes(), 0640); err != nil {
		logger.Fatal(err)
	}
}
