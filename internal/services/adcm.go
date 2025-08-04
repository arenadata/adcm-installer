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
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/arenadata/adcm-installer/internal/services/helpers"
	"github.com/arenadata/adcm-installer/pkg/compose"
	"github.com/arenadata/adcm-installer/pkg/types"
	"github.com/arenadata/adcm-installer/pkg/utils"

	"github.com/AlecAivazis/survey/v2"
)

type AdcmConfig struct {
	Count          uint8  `yaml:"adcm-count"`
	DBHost         string `yaml:"adcm-db-host"`
	DBPort         uint16 `yaml:"adcm-db-port"`
	DBName         string `yaml:"adcm-db-name"`
	DBUser         string `yaml:"adcm-db-user"`
	DBPassword     string `yaml:"adcm-db-pass"`
	DBSSLMode      string `yaml:"adcm-db-ssl-mode"`
	DBSSLCaFile    string `yaml:"adcm-db-ssl-ca-file"`
	DBSSLCertFile  string `yaml:"adcm-db-ssl-cert-file"`
	DBSSLKeyFile   string `yaml:"adcm-db-ssl-key-file"`
	SSLKeyFile     string `yaml:"adcm-ssl-key-file"`
	SSLCertFile    string `yaml:"adcm-ssl-cert-file"`
	Image          string `yaml:"adcm-image"`
	Tag            string `yaml:"adcm-tag"`
	PublishPort    uint16 `yaml:"adcm-publish-port"`
	PublishSSLPort uint16 `yaml:"adcm-publish-ssl-port"`
	Url            string `yaml:"adcm-url"`
	Volume         string `yaml:"adcm-volume"`

	ip string
}

func (prj *Project) adcm(name string) {
	config := prj.config.Adcm
	prj.config.Adcm.PublishPort++
	prj.config.Adcm.PublishSSLPort++

	if len(name) == 0 {
		name = AdcmName
	} else if name != AdcmName {
		config.DBName = strings.ReplaceAll(name, "-", "_")
		config.DBUser = name
		config.PublishPort = prj.config.Adcm.PublishPort
		config.PublishSSLPort = prj.config.Adcm.PublishSSLPort
		config.Url = fmt.Sprintf("http://%s:%d", config.ip, config.PublishPort)
	}
	addService(name, prj.prj)

	hostname := prj.hostname(name)
	if len(config.Volume) == 0 {
		config.Volume = hostname
	}

	if prj.interactive {
		checkErr(readValue(&config.Image, &prompt{msg: fmt.Sprintf("%s: ADCM image:", name), def: config.Image}))
		checkErr(readValue(&config.Tag, &prompt{msg: fmt.Sprintf("%s: ADCM image tag:", name), def: config.Tag}))

		adcmPublishPortDefault := strconv.Itoa(int(config.PublishPort))
		checkErr(readValue(&config.PublishPort,
			&prompt{msg: fmt.Sprintf("%s: ADCM publish port:", name), def: adcmPublishPortDefault}))
	}

	managedADPG := prj.config.Adpg.enable
	if prj.interactive || !managedADPG {
		if !managedADPG {
			checkErr(readValue(&config.DBHost,
				&prompt{msg: fmt.Sprintf("%s: ADCM database host:", name)}, survey.Required))

			portStr := strconv.Itoa(int(config.DBPort))
			checkErr(readValue(&config.DBPort,
				&prompt{msg: fmt.Sprintf("%s: ADCM database port:", name), def: portStr}))
		}

		checkErr(readValue(&config.DBName,
			&prompt{msg: fmt.Sprintf("%s: ADCM database name:", name), def: config.DBName}))
		checkErr(readValue(&config.DBUser,
			&prompt{msg: fmt.Sprintf("%s: ADCM database user:", name), def: config.DBUser}))

		passwdPrompt := &prompt{msg: fmt.Sprintf("%s: ADCM database password:", name), secret: true}
		if managedADPG {
			passwdPrompt.help = "If not set, a random password will be generated"
			checkErr(readValue(&config.DBPassword, passwdPrompt))
		} else {
			checkErr(readValue(&config.DBPassword, passwdPrompt, survey.Required))

			sslPrompt := &prompt{msg: "Select Postgres SSL mode:", def: config.DBSSLMode, opts: allowSSLModes}
			checkErr(readValue(&config.DBSSLMode, sslPrompt, survey.Required))

			if config.DBSSLMode != pgSslModeDisable {
				checkErr(readValue(&config.DBSSLCaFile,
					&prompt{msg: fmt.Sprintf("%s: ADCM database SSL CA file path:", name)}, fileExists))
				checkErr(readValue(&config.DBSSLCertFile,
					&prompt{msg: fmt.Sprintf("%s: ADCM database SSL certificate file path:", name)}, fileExists))
				checkErr(readValue(&config.DBSSLKeyFile,
					&prompt{msg: fmt.Sprintf("%s: ADCM database SSL private key file path:", name)}, fileExists))
			}
		}
	}

	if prj.interactive {
		checkErr(readValue(&config.Url, &prompt{msg: fmt.Sprintf("%s: ADCM url", name), def: config.Url}))
		checkErr(readValue(&config.Volume,
			&prompt{msg: fmt.Sprintf("%s: ADCM volume name or path:", name), def: config.Volume}))

		p := &prompt{msg: fmt.Sprintf("%s: ADCM SSL Private Key file path:", name),
			help: "Leave blank if you do not enable HTTPS"}
		checkErr(readValue(&config.SSLKeyFile, p, fileExists))
		if len(config.SSLKeyFile) > 0 {
			checkErr(readValue(&config.SSLCertFile,
				&prompt{msg: fmt.Sprintf("%s: ADCM SSL Certificate file path:", name)}, fileExists))

			sslPort := strconv.Itoa(int(config.PublishSSLPort))
			checkErr(readValue(&config.PublishSSLPort,
				&prompt{msg: fmt.Sprintf("%s: ADCM publish SSL port:", name), def: sslPort}))

			prj.AppendHelpers(
				helpers.Secrets(name,
					helpers.Secret{
						Source:   PemKey,
						Target:   path.Join(ADCMMountPath, "conf/ssl/key.pem"),
						FileMode: 0o400,
					},
					helpers.Secret{
						Source:   PemCert,
						Target:   path.Join(ADCMMountPath, "conf/ssl/cert.pem"),
						FileMode: 0o440,
					},
				),
				helpers.PublishPort(name, config.PublishSSLPort, ADCMPublishSSLPort),
			)
		}
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
	} else {
		portStr := strconv.Itoa(int(config.DBPort))
		prj.AppendHelpers(
			helpers.Environment(name,
				helpers.Env{Name: "DB_HOST", Value: &config.DBHost},
				helpers.Env{Name: "DB_PORT", Value: &portStr},
			))
	}

	xsecretsData := map[string]string{
		PgDbName: config.DBName,
		PgDbUser: config.DBUser,
		PgDbPass: config.DBPassword,
	}

	if config.DBSSLMode != pgSslModeDisable {
		sslOpts := types.DbSSLOptions{SSLMode: config.DBSSLMode}

		if len(config.DBSSLCaFile) > 0 {
			b, err := os.ReadFile(config.DBSSLCaFile)
			checkErr(err)
			xsecretsData[PgSslCaKey] = string(b)

			target := path.Join(helpers.SecretsPath, PgSslCaKey)
			sslOpts.SSLRootCert = target
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
			sslOpts.SSLCert = target
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
			sslOpts.SSLKey = target
			prj.AppendHelpers(
				helpers.Secrets(name, helpers.Secret{
					Source:   PgSslKeyKey,
					Target:   target,
					FileMode: 0o400,
				}),
			)
		}

		optStr := sslOpts.String()
		prj.AppendHelpers(helpers.Environment(name, helpers.Env{Name: "DB_OPTIONS", Value: &optStr}))
	}

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

	xsecretsDataEncrypted := xsecretsData
	if prj.crypt != nil {
		var err error
		for k, v := range xsecretsData {
			v, err = prj.crypt.EncryptValue(v)
			checkErr(err)
			xsecretsDataEncrypted[k] = v
		}
	}

	prj.AppendHelpers(
		helpers.Hostname(name, hostname),
		helpers.CapAdd(name, "CAP_CHOWN", "CAP_SETUID", "CAP_SETGID"), // FIXME: run container as non-root
		helpers.Labels(name, map[string]string{compose.ADAppTypeLabelKey: AdcmName}),
		helpers.Image(name, config.Image+":"+config.Tag),
		helpers.Environment(name, helpers.Env{Name: "DEFAULT_ADCM_URL", Value: &config.Url}),
		helpers.Extension(name, XSecretsKey, &XSecrets{Data: xsecretsDataEncrypted}),
		helpers.Volumes(name, config.Volume+":"+ADCMMountPath),
	)

	if config.PublishPort > 0 {
		prj.AppendHelpers(helpers.PublishPort(name, config.PublishPort, ADCMPublishPort))
	}
}
