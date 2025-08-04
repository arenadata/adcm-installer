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
	"strconv"
	"time"

	"github.com/arenadata/adcm-installer/internal/services/helpers"
	"github.com/arenadata/adcm-installer/pkg/compose"
	"github.com/arenadata/adcm-installer/pkg/utils"
)

type AdpgConfig struct {
	enable bool

	Password    string `yaml:"adpg-pass"`
	Image       string `yaml:"adpg-image"`
	Tag         string `yaml:"adpg-tag"`
	PublishPort uint16 `yaml:"adpg-publish-port"`
	Volume      string `yaml:"adpg-volume"`
}

func (prj *Project) adpg() {
	config := prj.config.Adpg
	if !config.enable {
		return
	}

	name := AdpgName
	addService(name, prj.prj)

	hostname := prj.hostname(name)
	if len(config.Volume) == 0 {
		config.Volume = hostname
	}

	if prj.interactive {
		checkErr(readValue(&config.Password,
			&prompt{msg: "ADPG superuser password:", help: "If not set, a random password will be generated",
				secret: true}))
		checkErr(readValue(&config.Image, &prompt{msg: "ADPG image", def: config.Image}))
		checkErr(readValue(&config.Tag, &prompt{msg: "ADPG image tag", def: config.Tag}))

		port := strconv.Itoa(int(config.PublishPort))
		checkErr(readValue(&config.PublishPort, &prompt{msg: "ADPG publish port", def: port}))
		checkErr(readValue(&config.Volume, &prompt{msg: "ADPG volume name or path", def: config.Volume}))
	}

	if len(config.Password) == 0 {
		config.Password = utils.GenerateRandomString(16)
	}

	passwd := config.Password
	if prj.crypt != nil {
		var err error
		passwd, err = prj.crypt.EncryptValue(config.Password)
		checkErr(err)
	}

	xsecretsDataEncrypted := map[string]string{
		"password": passwd,
	}

	prj.AppendHelpers(
		helpers.Hostname(name, hostname),
		helpers.Image(name, config.Image+":"+config.Tag),
		helpers.Extension(name, XSecretsKey, &XSecrets{Data: xsecretsDataEncrypted}),
		helpers.Labels(name, map[string]string{compose.ADAppTypeLabelKey: AdpgName}),
		helpers.HealthCheck(name, helpers.HealthCheckConfig{
			Cmd:      []string{"CMD-SHELL", "pg-entrypoint isready postgres"},
			Interval: 3 * time.Second,
			Timeout:  3 * time.Second,
			Retries:  3,
		}),
		helpers.Volumes(name, config.Volume+":"+ADPGDataMountPath),
	)

	if config.PublishPort > 0 {
		prj.AppendHelpers(helpers.PublishPort(name, config.PublishPort, ADPGPublishPort))
	}
}
