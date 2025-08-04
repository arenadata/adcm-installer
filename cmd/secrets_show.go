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
	"github.com/arenadata/adcm-installer/internal/services"
	"github.com/arenadata/adcm-installer/pkg/secrets"
	"gopkg.in/yaml.v3"

	composeTypes "github.com/compose-spec/compose-go/v2/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show application secrets",
	Long: `Displays decrypted secrets config file.
- --age-key takes the value of the private key in clear text. Has priority over
            --age-key-file
- --age-key-file takes the value of the path to the file with the private key
- --file specifies the path to the config file`,
	Run: secretsShow,
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

	aes, err := encoder(cmd, prj)
	if err != nil {
		logger.Fatal(err)
	}

	xSecrets, unMappedxSecrets, err := secretsDecrypt(prj.Services, aes)
	if err != nil {
		logger.Fatal(err)
	}

	enc := yaml.NewEncoder(cmd.OutOrStdout())
	defer func() {
		if e := enc.Close(); e != nil {
			err = e
		}
	}()

	enc.SetIndent(2)
	_ = enc.Encode(xSecrets)
	_ = enc.Encode(unMappedxSecrets)
}

func secretsDecrypt(serviceList composeTypes.Services, dec secrets.Secrets) (map[string]map[string]string, map[string]map[string]string, error) {
	sec := make(map[string]map[string]string)
	unMappedSec := make(map[string]map[string]string)
	for svcName, service := range serviceList {
		svc, ok := service.Extensions[services.XSecretsKey]
		if ok {
			xsec := svc.(*services.XSecrets)
			s, err := decrypt(dec, xsec.Data)
			if err != nil {
				return nil, nil, err
			}
			sec[svcName] = s

			un, err := decrypt(dec, xsec.UnMapped)
			if err != nil {
				return nil, nil, err
			}
			unMappedSec[svcName] = un
		}
	}
	return sec, unMappedSec, nil
}

func decrypt(dec secrets.Secrets, m map[string]string) (map[string]string, error) {
	if m == nil {
		return nil, nil
	}
	if dec == nil {
		return m, nil
	}

	var err error
	out := map[string]string{}
	for k, v := range m {
		v, err = dec.DecryptValue(v)
		if err != nil {
			return nil, err
		}

		out[k] = v
	}

	return out, nil
}
