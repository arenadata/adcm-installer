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

	"github.com/arenadata/adcm-installer/internal/services/helpers"
	"github.com/arenadata/adcm-installer/pkg/compose"
)

type ConsulConfig struct {
	enable bool

	Image       string `yaml:"consul-image"`
	Tag         string `yaml:"consul-tag"`
	PublishPort uint16 `yaml:"consul-publish-port"`
	Volume      string `yaml:"consul-volume"`
}

func (prj *Project) consul() {
	config := prj.config.Consul
	if !config.enable {
		return
	}

	name := ConsulName
	addService(name, prj.prj)

	hostname := prj.hostname(name)
	if len(config.Volume) == 0 {
		config.Volume = hostname
	}

	if prj.interactive {
		checkErr(readValue(&config.Image, &prompt{msg: "Consul image", def: config.Image}))
		checkErr(readValue(&config.Tag, &prompt{msg: "Consul image tag", def: config.Tag}))

		portStr := strconv.Itoa(int(config.PublishPort))
		checkErr(readValue(&config.PublishPort, &prompt{msg: "Consul publish port", def: portStr}))
		checkErr(readValue(&config.Volume, &prompt{msg: "Consul volume name or path", def: config.Volume}))
	}

	prj.AppendHelpers(
		helpers.Hostname(name, hostname),
		helpers.Command(name, []string{"agent", "-dev", "-bind=0.0.0.0"}),
		helpers.Image(name, config.Image+":"+config.Tag),
		helpers.Labels(name, map[string]string{compose.ADAppTypeLabelKey: ConsulName}),
	)

	if config.PublishPort > 0 {
		prj.AppendHelpers(helpers.PublishPort(name, config.PublishPort, ConsulPublishPort))
	}
}
