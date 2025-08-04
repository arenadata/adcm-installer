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
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/arenadata/adcm-installer/internal/services"
	"github.com/arenadata/adcm-installer/pkg/secrets"
	"github.com/arenadata/adcm-installer/pkg/utils"

	composeTypes "github.com/compose-spec/compose-go/v2/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	ageKeyFileName = "age.key"
)

var (
	noAgeKeyProvided = errors.New("no age key provided")
)

// secretsCmd represents the secrets command
var secretsCmd = &cobra.Command{
	Aliases: []string{"sec", "secret"},
	Use:     "secrets",
	Short:   "Manage secrets",
	Long:    ``,
}

func init() {
	rootCmd.AddCommand(secretsCmd)
}

func ageEnvKey(key string) string {
	key = strings.ToUpper(key)
	return strings.ReplaceAll(key, "-", "_")
}

func ageKeyFlags(cmd *cobra.Command, key, defaultKeyPath string, markFlags ...func(...string)) {
	if len(key) == 0 {
		panic("age-key must not be empty")
	}

	fileKey := key + "-file"
	envKey := ageEnvKey(key)
	cmd.Flags().String(key, "", "Set private age key. Can be set by "+envKey+" environment variable")
	cmd.Flags().String(fileKey, defaultKeyPath, "Read private age key from file")

	cmd.MarkFlagsMutuallyExclusive(key, fileKey)

	if len(defaultKeyPath) > 0 {
		if isAgeKeyFileExists, err := utils.FileExists(defaultKeyPath); err != nil {
			panic(err)
		} else if isAgeKeyFileExists {
			return
		}
	}

	for _, markFlag := range markFlags {
		markFlag(key, fileKey)
	}
}

func getAgeKey(cmd *cobra.Command, key string) (string, error) {
	ageKey, _ := cmd.Flags().GetString(key)
	if len(ageKey) == 0 {
		envKey := ageEnvKey(key)
		if ageKey = os.Getenv(envKey); len(ageKey) > 0 {
			_ = os.Unsetenv(envKey)
		}
	}

	fileKey := key + "-file"
	if len(ageKey) == 0 || cmd.Flags().Changed(fileKey) {
		ageKeyFile, _ := cmd.Flags().GetString(fileKey)
		isAgeKeyFileExists, err := utils.FileExists(ageKeyFile)
		if err != nil {
			return "", err
		}
		if isAgeKeyFileExists {
			ageKey, err = readAgeKeyFromFile(ageKeyFile)
			if err != nil {
				return "", fmt.Errorf("read AGE key from file %q failed: %v", ageKeyFile, err)
			}
		}
	}

	if len(ageKey) > 0 {
		return ageKey, nil
	}

	return "", noAgeKeyProvided
}

func readAgeKeyFromFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		return strings.TrimSpace(line), nil
	}

	return "", fmt.Errorf("no age key found")
}

func readOrCreateNewAgeKey(cmd *cobra.Command, key string) (*secrets.AgeCrypt, bool, error) {
	ageKey, err := getAgeKey(cmd, key)
	if err != nil && !errors.Is(err, noAgeKeyProvided) {
		return nil, false, err
	} else if err == nil {
		cryptKey, err := secrets.NewAgeCryptFromString(ageKey)
		return cryptKey, false, err
	}

	cryptKey, err := secrets.NewAgeCrypt()
	return cryptKey, true, err
}

func toYaml(output io.Writer, v any) (err error) {
	enc := yaml.NewEncoder(output)
	defer func() {
		if e := enc.Close(); e != nil {
			err = e
		}
	}()

	enc.SetIndent(2)
	return enc.Encode(v)
}

func encoder(cmd *cobra.Command, prj *composeTypes.Project) (secrets.Secrets, error) {
	xSecrets, ok := prj.Extensions[services.XSecretsKey]
	if ok {
		sec := xSecrets.(*services.XSecrets)

		ageKey, err := getAgeKey(cmd, "age-key")
		if err != nil {
			return nil, err
		}

		dec, err := secrets.NewAgeCryptFromString(ageKey)
		if err != nil {
			return nil, err
		}

		if sec.AgeRecipient != dec.Recipient().String() {
			return nil, fmt.Errorf("age_recipient not match")
		}

		aesKey, err := dec.Decrypt(sec.Key)
		if err != nil {
			return nil, err
		}

		return secrets.NewAesCrypt([]byte(aesKey))
	}

	return nil, nil
}
