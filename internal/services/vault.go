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

package services

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/arenadata/adcm-installer/internal/services/helpers"
	"github.com/arenadata/adcm-installer/pkg/compose"
	"github.com/arenadata/adcm-installer/pkg/utils"

	"github.com/AlecAivazis/survey/v2"
)

type VaultConfig struct {
	enable bool

	DBHost        string `yaml:"vault-db-host"`
	DBPort        uint16 `yaml:"vault-db-port"`
	DBName        string `yaml:"vault-db-name"`
	DBUser        string `yaml:"vault-db-user"`
	DBPassword    string `yaml:"vault-db-pass"`
	DBSSLMode     string `yaml:"vault-db-ssl-mode"`
	DBSSLCaFile   string `yaml:"vault-db-ssl-ca-file"`
	DBSSLCertFile string `yaml:"vault-db-ssl-cert-file"`
	DBSSLKeyFile  string `yaml:"vault-db-ssl-key-file"`
	SSLKeyFile    string `yaml:"vault-ssl-key-file"`
	SSLCertFile   string `yaml:"vault-ssl-cert-file"`
	Image         string `yaml:"vault-image"`
	Tag           string `yaml:"vault-tag"`
	PublishPort   uint16 `yaml:"vault-publish-port"`
	Mode          string `yaml:"vault-mode"`
	UI            *bool  `yaml:"vault-ui"`
}

type VaultConfigFile struct {
	Listener     []map[string]any `json:"listener"`
	VaultStorage `json:",inline"`
}

type VaultStorage struct {
	Storage VaultBackend `json:"storage"`
}
type VaultBackend struct {
	Postgresql VaultBackendPostgresql `json:"postgresql"`
}

type VaultBackendPostgresql struct {
	ConnectionUrl string `json:"connection_url"`
	HA            bool   `json:"ha_enabled,omitempty"`
	//max_idle_connections = 64
	//max_parallel = 32

	// defaults
	//table = "openbao_kv_store"
	//upsert_function = "openbao_kv_put"
	//max_parallel = 128
	//transaction_max_parallel = 64
	//ha_table = "openbao_ha_locks"
	//skip_create_table = false
}

func (prj *Project) vault() {
	config := prj.config.Vault
	if !config.enable {
		return
	}

	name := VaultName
	addService(name, prj.prj)

	managedADPG := prj.config.Adpg.enable
	if prj.interactive {
		modePrompt := &prompt{
			msg:  "Select Vault Deployment mode:",
			def:  config.Mode,
			opts: allowVaultDeploymentModes,
		}
		checkErr(readValue(&config.Mode, modePrompt, survey.Required))

		var ui string
		opts := []string{"true", "false"}
		checkErr(readValue(&ui,
			&prompt{msg: "Vault enable UI", def: fmt.Sprintf("%t", *config.UI), opts: opts}, survey.Required))
		*config.UI = ui == "true"

		checkErr(readValue(&config.Image, &prompt{msg: "Vault image", def: config.Image}))
		checkErr(readValue(&config.Tag, &prompt{msg: "Vault image tag", def: config.Tag}))

		port := strconv.Itoa(int(config.PublishPort))
		checkErr(readValue(&config.PublishPort, &prompt{msg: "Vault publish port", def: port}))
	}

	tcpListener := map[string]any{
		"address": fmt.Sprintf("0.0.0.0:%d", VaultPublishPort),
	}

	config.DBHost = AdpgName
	config.DBPort = ADPGPublishPort

	if config.Mode != VaultDeployModeDev && (prj.interactive || !managedADPG) {
		if !managedADPG {
			checkErr(readValue(&config.DBHost,
				&prompt{msg: "Vault database host:"}, survey.Required))

			portStr := strconv.Itoa(int(config.DBPort))
			checkErr(readValue(&config.DBPort,
				&prompt{msg: "Vault database port:", def: portStr}))
		}

		checkErr(readValue(&config.DBName,
			&prompt{msg: "Vault database name:", def: config.DBName}))
		checkErr(readValue(&config.DBUser,
			&prompt{msg: "Vault database user:", def: config.DBUser}))

		passwdPrompt := &prompt{msg: "Vault database password:", secret: true}
		if managedADPG {
			passwdPrompt.help = "If not set, a random password will be generated"
			checkErr(readValue(&config.DBPassword, passwdPrompt))
		} else {
			checkErr(readValue(&config.DBPassword, passwdPrompt, survey.Required))

			sslPrompt := &prompt{msg: "Select Postgres SSL mode:", def: config.DBSSLMode, opts: allowSSLModes}
			checkErr(readValue(&config.DBSSLMode, sslPrompt, survey.Required))

			if config.DBSSLMode != pgSslModeDisable {
				checkErr(readValue(&config.DBSSLCaFile,
					&prompt{msg: "Vault database SSL CA file path:"}, fileExists))
				checkErr(readValue(&config.DBSSLCertFile,
					&prompt{msg: "Vault database SSL certificate file path:"}, fileExists))
				checkErr(readValue(&config.DBSSLKeyFile,
					&prompt{msg: "Vault database SSL private key file path:"}, fileExists))
			}
		}

		if prj.interactive {
			p := &prompt{msg: "Vault SSL Private Key file path:",
				help: "Leave blank if you do not enable HTTPS"}
			checkErr(readValue(&config.SSLKeyFile, p, fileExists))

			if len(config.SSLKeyFile) > 0 {
				checkErr(readValue(&config.SSLCertFile,
					&prompt{msg: "Vault SSL Certificate file path:"}, fileExists))

				keyTarget := path.Join(helpers.SecretsPath, PemKey)
				tcpListener["tls_key_file"] = keyTarget

				certTarget := path.Join(helpers.SecretsPath, PemCert)
				tcpListener["tls_cert_file"] = certTarget

				prj.AppendHelpers(
					helpers.Secrets(name,
						helpers.Secret{
							Source:   PemKey,
							Target:   keyTarget,
							FileMode: 0o400,
						},
						helpers.Secret{
							Source:   PemCert,
							Target:   certTarget,
							FileMode: 0o440,
						},
					),
				)
			}
		}
	}

	if len(config.SSLKeyFile) == 0 {
		tcpListener["tls_disable"] = true
	}

	if len(config.DBPassword) == 0 {
		config.DBPassword = utils.GenerateRandomString(16)
	}

	if managedADPG {
		prj.AppendHelpers(
			helpers.DependsOn(name,
				helpers.Depended{
					Service:  AdpgName,
					Required: true,
				}),
		)
	}

	if config.Mode == VaultDeployModeDev {
		prj.AppendHelpers(
			helpers.Environment(name,
				helpers.Env{Name: "BAO_DEV_ROOT_TOKEN_ID", Value: utils.Ptr("openbao_secret")},
				helpers.Env{Name: "BAO_DEV_LISTEN_ADDRESS", Value: utils.Ptr("0.0.0.0:8200")},
			),
		)
	} else {
		baoAddr := "http://127.0.0.1:8200/"
		var wgetArg string
		if len(config.SSLKeyFile) > 0 {
			baoAddr = "https://127.0.0.1:8200/"
			wgetArg = " --no-check-certificate"
		}

		healthCheckCommand := fmt.Sprintf("wget%s -q -O - %sv1/sys/health", wgetArg, baoAddr)
		prj.AppendHelpers(
			helpers.Environment(name,
				helpers.Env{Name: "BAO_ADDR", Value: &baoAddr},
			),
			helpers.HealthCheck(name, helpers.HealthCheckConfig{
				Cmd:      []string{"CMD-SHELL", healthCheckCommand},
				Interval: 3 * time.Second,
				Retries:  5,
				Timeout:  5 * time.Second,
			}),
		)

		unMappedSecrets := map[string]string{
			PgDbName: config.DBName,
			PgDbUser: config.DBUser,
			PgDbPass: config.DBPassword,
		}
		xsecretsData := map[string]string{}

		params := url.Values{}
		params.Set("sslmode", config.DBSSLMode)

		if config.DBSSLMode != pgSslModeDisable {
			if len(config.DBSSLCaFile) > 0 {
				b, err := os.ReadFile(config.DBSSLCaFile)
				checkErr(err)
				xsecretsData[PgSslCaKey] = string(b)

				target := path.Join(helpers.SecretsPath, PgSslCaKey)
				params.Set("sslrootcert", target)
				prj.AppendHelpers(
					helpers.Secrets(name, helpers.Secret{
						Source:   PgSslCaKey,
						Target:   target,
						FileMode: 0o440,
					}),
				)
			}
			if len(config.DBSSLCertFile) > 0 {
				b, err := os.ReadFile(config.DBSSLCertFile)
				checkErr(err)
				xsecretsData[PgSslCertKey] = string(b)

				target := path.Join(helpers.SecretsPath, PgSslCertKey)
				params.Set("sslcert", target)
				prj.AppendHelpers(
					helpers.Secrets(name, helpers.Secret{
						Source:   PgSslCertKey,
						Target:   target,
						FileMode: 0o440,
					}),
				)
			}
			if len(config.DBSSLKeyFile) > 0 {
				b, err := os.ReadFile(config.DBSSLKeyFile)
				checkErr(err)
				xsecretsData[PgSslKeyKey] = string(b)

				target := path.Join(helpers.SecretsPath, PgSslKeyKey)
				params.Set("sslkey", target)
				prj.AppendHelpers(
					helpers.Secrets(name, helpers.Secret{
						Source:   PgSslKeyKey,
						Target:   target,
						FileMode: 0o400,
					}),
				)
			}
		}

		u := &url.URL{
			Scheme:   "postgres",
			Host:     fmt.Sprintf("%s:%d", config.DBHost, config.DBPort),
			RawQuery: params.Encode(),
		}

		backend := VaultBackendPostgresql{
			ConnectionUrl: u.String(),
			HA:            config.Mode == VaultDeployModeHa,
		}

		vaultConfig := VaultConfigFile{
			Listener: []map[string]any{{"tcp": tcpListener}},
			VaultStorage: VaultStorage{
				Storage: VaultBackend{Postgresql: backend},
			},
		}

		b, err := json.Marshal(vaultConfig)
		checkErr(err)
		xsecretsData[ConfigJson] = string(b)

		prj.AppendHelpers(
			helpers.Secrets(name, helpers.Secret{
				Source:   ConfigJson,
				Target:   path.Join(helpers.SecretsPath, ConfigJson),
				FileMode: 0o400,
			}),
		)

		if len(config.SSLKeyFile) > 0 {
			b, err := os.ReadFile(config.SSLKeyFile)
			checkErr(err)
			xsecretsData[PemKey] = string(b)
		}
		if len(config.SSLCertFile) > 0 {
			b, err := os.ReadFile(config.SSLCertFile)
			checkErr(err)
			xsecretsData[PemCert] = string(b)
		}

		if prj.crypt != nil {
			var err error
			for k, v := range xsecretsData {
				v, err = prj.crypt.EncryptValue(v)
				checkErr(err)
				xsecretsData[k] = v
			}

			for k, v := range unMappedSecrets {
				v, err = prj.crypt.EncryptValue(v)
				checkErr(err)
				unMappedSecrets[k] = v
			}
		}

		prj.AppendHelpers(helpers.Extension(name, XSecretsKey, &XSecrets{Data: xsecretsData, UnMapped: unMappedSecrets}))
	}

	prj.AppendHelpers(
		helpers.Hostname(name, prj.hostname(name)),
		helpers.Image(name, config.Image+":"+config.Tag),
		helpers.Labels(name, map[string]string{
			compose.ADAppTypeLabelKey:   VaultName,
			compose.ADVaultModeLabelKey: config.Mode,
		}),
	)

	if *config.UI {
		prj.AppendHelpers(
			helpers.Environment(name, helpers.Env{Name: "BAO_UI", Value: utils.Ptr("true")}),
		)
	}

	if config.PublishPort > 0 {
		prj.AppendHelpers(helpers.PublishPort(VaultName, config.PublishPort, VaultPublishPort))
	}
}
