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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arenadata/adcm-installer/pkg/compose"

	"github.com/bmizerany/assert"
	composeTypes "github.com/compose-spec/compose-go/v2/types"
	"github.com/spf13/cobra"
)

func Test_addService(t *testing.T) {
	const name = "my-service"
	prj := &composeTypes.Project{Services: make(composeTypes.Services)}

	addService(name, prj)
	assert.Equal(t, prj.Services[name], composeTypes.ServiceConfig{Name: name})
}

func TestInitConfigDefaults_EmptyConfig(t *testing.T) {
	config := &initConfig{}
	initConfigDefaults(config)

	assert.Equal(t, compose.ADPGPublishPort, config.ADCMDBPort)
	assert.Equal(t, "adcm", config.ADCMDBName)
	assert.Equal(t, "adcm", config.ADCMDBUser)
	assert.Equal(t, postgresSSLMode, config.ADCMDBSSLMode)
	assert.Equal(t, compose.ADCMPublishPort, config.ADCMPublishPort)
	assert.Equal(t, compose.ADCMImage, config.ADCMImage)
	assert.Equal(t, compose.ADCMTag, config.ADCMTag)
	assert.Equal(t, "adcm", config.ADCMVolume)
	assert.Equal(t, compose.ADPGImage, config.ADPGImage)
	assert.Equal(t, compose.ADPGTag, config.ADPGTag)
	assert.Equal(t, compose.ConsulImage, config.ConsulImage)
	assert.Equal(t, compose.ConsulTag, config.ConsulTag)
	assert.Equal(t, compose.ConsulPublishPort, config.ConsulPublishPort)
	assert.Equal(t, compose.VaultImage, config.VaultImage)
	assert.Equal(t, compose.VaultTag, config.VaultTag)
	assert.Equal(t, compose.VaultPublishPort, config.VaultPublishPort)
}

func TestInitConfigDefaults_PartialConfig(t *testing.T) {
	config := &initConfig{
		ADCMDBPort:        1234,
		ADCMDBName:        "customdb",
		ADCMImage:         "custom-image",
		ConsulPublishPort: 9000,
		VaultTag:          "custom-tag",
	}
	initConfigDefaults(config)

	assert.Equal(t, uint16(1234), config.ADCMDBPort)
	assert.Equal(t, "customdb", config.ADCMDBName)
	assert.Equal(t, "custom-image", config.ADCMImage)
	assert.Equal(t, uint16(9000), config.ConsulPublishPort)
	assert.Equal(t, "custom-tag", config.VaultTag)

	assert.Equal(t, "adcm", config.ADCMDBUser)
	assert.Equal(t, postgresSSLMode, config.ADCMDBSSLMode)
	assert.Equal(t, compose.ADCMPublishPort, config.ADCMPublishPort)
	assert.Equal(t, compose.ADCMTag, config.ADCMTag)
	assert.Equal(t, "adcm", config.ADCMVolume)
	assert.Equal(t, compose.ADPGImage, config.ADPGImage)
	assert.Equal(t, compose.ADPGTag, config.ADPGTag)
	assert.Equal(t, compose.ConsulImage, config.ConsulImage)
	assert.Equal(t, compose.ConsulTag, config.ConsulTag)
	assert.Equal(t, compose.VaultImage, config.VaultImage)
	assert.Equal(t, compose.VaultPublishPort, config.VaultPublishPort)
}

func TestInitConfigDefaults_AllFieldsSet(t *testing.T) {
	config := &initConfig{
		ADCMDBHost:        "host",
		ADCMDBPort:        1234,
		ADCMDBName:        "name",
		ADCMDBUser:        "user",
		ADCMDBPassword:    "pass",
		ADCMDBSSLMode:     "mode",
		ADCMDBSSLCaFile:   "ca",
		ADCMDBSSLCertFile: "cert",
		ADCMDBSSLKeyFile:  "key",
		ADCMImage:         "image",
		ADCMTag:           "tag",
		ADCMPublishPort:   1234,
		ADCMUrl:           "url",
		ADCMVolume:        "vol",
		ADPGPassword:      "pgpass",
		ADPGImage:         "pgimage",
		ADPGTag:           "pgtag",
		ADPGPublishPort:   1234,
		ConsulImage:       "consulimg",
		ConsulTag:         "consultag",
		ConsulPublishPort: 1234,
		VaultImage:        "vaultimg",
		VaultTag:          "vaulttag",
		VaultPublishPort:  1234,
	}

	original := *config

	initConfigDefaults(config)

	assert.Equal(t, original, *config)
}

func Test_isConfigExists(t *testing.T) {
	cmd := func(output string, force bool) *cobra.Command {
		cmd := &cobra.Command{}
		initCmdFlags(cmd)

		_ = cmd.Flags().Set("output", output)
		_ = cmd.Flags().Set("force", fmt.Sprint(force))
		return cmd
	}

	absPath, err := filepath.Abs("")
	assert.Equal(t, nil, err)

	tests := []struct {
		name    string
		cmd     *cobra.Command
		want    string
		wantErr bool
	}{
		{"EmptyOutput", cmd("", false), filepath.Join(absPath, "adcm.yaml"), false},
		{"ExistsFile", cmd("../README.md", false), filepath.Join(absPath, "../README.md"), true},
		{"ExistsFileWithForce", cmd("../README.md", true), filepath.Join(absPath, "../README.md"), false},
		{"NotExistsFile", cmd("test-adcm-file-no-test.yaml", false), filepath.Join(absPath, "test-adcm-file-no-test.yaml"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := isConfigExists(tt.cmd); (err != nil) != tt.wantErr {
				t.Errorf("isConfigExists() error = %v, wantErr %v", err, tt.wantErr)
			}

			s, err := tt.cmd.Flags().GetString("output")
			assert.Equal(t, nil, err)
			if s != tt.want {
				t.Errorf("isConfigExists() output = %v, expected %v", s, tt.want)
			}
		})
	}
}

func Test_valuesFromConfigFile(t *testing.T) {
	tests := []struct {
		name        string
		configFile  string
		configData  string
		wantConfig  *initConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "ValidConfig",
			configData: `
adcm-db-host: "localhost"
adcm-db-port: 5432
adcm-db-name: "adcm"
adcm-db-user: "user"
adcm-db-pass: "pass"
adcm-db-ssl-mode: "disable"
adcm-image: "adcm/image"
adcm-tag: "latest"
adcm-publish-port: 8080
adcm-url: "http://example.com"
adcm-volume: "/data"
adpg-pass: "adpgpass"
adpg-image: "adpg/image"
adpg-tag: "1.0.0"
adpg-publish-port: 5433
consul-image: "consul/image"
consul-tag: "1.9.0"
consul-publish-port: 8500
vault-image: "vault/image"
vault-tag: "1.7.0"
vault-publish-port: 8200
`,
			wantConfig: &initConfig{
				ADCMDBHost:        "localhost",
				ADCMDBPort:        5432,
				ADCMDBName:        "adcm",
				ADCMDBUser:        "user",
				ADCMDBPassword:    "pass",
				ADCMDBSSLMode:     "disable",
				ADCMImage:         "adcm/image",
				ADCMTag:           "latest",
				ADCMPublishPort:   8080,
				ADCMUrl:           "http://example.com",
				ADCMVolume:        "/data",
				ADPGPassword:      "adpgpass",
				ADPGImage:         "adpg/image",
				ADPGTag:           "1.0.0",
				ADPGPublishPort:   5433,
				ConsulImage:       "consul/image",
				ConsulTag:         "1.9.0",
				ConsulPublishPort: 8500,
				VaultImage:        "vault/image",
				VaultTag:          "1.7.0",
				VaultPublishPort:  8200,
			},
			wantErr: false,
		},
		{
			name:        "FileDoesNotExist",
			configFile:  "nonexistent.yaml",
			wantErr:     true,
			errContains: "no such file or directory",
		},
		{
			name: "UnknownField",
			configData: `
adcm-db-host: "localhost"
unknown-field: "value"
`,
			wantErr:     true,
			errContains: "not found in type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file if config data is provided
			if tt.configData != "" {
				tmpFile, err := os.CreateTemp("", "config-*.yaml")
				assert.Equal(t, nil, err)
				defer func() { _ = os.Remove(tmpFile.Name()) }()

				_, err = tmpFile.WriteString(tt.configData)
				assert.Equal(t, nil, err)
				err = tmpFile.Close()
				assert.Equal(t, nil, err)

				tt.configFile = tmpFile.Name()
			}

			got, err := valuesFromConfigFile(tt.configFile)

			if tt.wantErr {
				assert.NotEqual(t, nil, err)
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("valuesFromConfigFile() error output = %v, expected %v", err.Error(), tt.errContains)
				}
				return
			}

			assert.Equal(t, nil, err)
			assert.Equal(t, tt.wantConfig, got)
		})
	}
}
