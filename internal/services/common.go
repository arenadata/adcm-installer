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
	"strings"

	"github.com/arenadata/adcm-installer/assets"
	"github.com/arenadata/adcm-installer/internal/services/helpers"

	composeTypes "github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/compose/v2/pkg/api"
	composeUtils "github.com/docker/compose/v2/pkg/utils"
)

func ChownContainer(prj *composeTypes.Project, svc composeTypes.ServiceConfig) string {
	var mounts []string
	for _, mnt := range svc.Volumes {
		mounts = append(mounts, mnt.Target)
	}

	newSvc := composeTypes.ServiceConfig{
		Name:       "chown-" + svc.Name,
		User:       "0:0",
		Image:      assets.ImageName,
		Entrypoint: composeTypes.ShellCommand{"/bin/sh"},
		Command: []string{
			"-cex",
			fmt.Sprintf("chown -v %s %s", svc.User, strings.Join(mounts, " ")),
		},
		Volumes:  svc.Volumes,
		Profiles: []string{"chown", InitContainerProfile},
	}

	setCustomLabels(prj, &newSvc)
	prj.Services[newSvc.Name] = newSvc
	return newSvc.Name
}

func InitContainer(prj *composeTypes.Project, svc composeTypes.ServiceConfig) string {
	newSvc := composeTypes.ServiceConfig{
		Name:        "init-" + svc.Name,
		User:        svc.User,
		Image:       svc.Image,
		Volumes:     svc.Volumes,
		Secrets:     svc.Secrets,
		Environment: composeTypes.MappingWithEquals{},
		Profiles:    []string{InitContainerProfile},
	}

	setCustomLabels(prj, &newSvc)
	prj.Services[newSvc.Name] = newSvc
	return newSvc.Name
}

func PauseContainer(prj *composeTypes.Project) {
	var depends []helpers.Depended
	for _, svc := range prj.Services {
		if composeUtils.StringContains(svc.Profiles, InitContainerProfile) {
			depends = append(depends,
				helpers.Depended{Service: svc.Name, Condition: composeTypes.ServiceConditionCompletedSuccessfully})
		}
	}

	if len(depends) == 0 {
		return
	}

	newSvc := composeTypes.ServiceConfig{
		Name:       PauseName,
		Image:      assets.ImageName,
		Command:    []string{"sleep", "120"},
		StopSignal: "SIGKILL",
		Profiles:   []string{InitContainerProfile},
	}

	setCustomLabels(prj, &newSvc)
	prj.Services[PauseName] = newSvc

	servicesModHelpers := helpers.NewModHelpers()
	servicesModHelpers = append(servicesModHelpers, helpers.DependsOn(PauseName, depends...))
	_ = servicesModHelpers.Apply(prj)
}

func setCustomLabels(prj *composeTypes.Project, svc *composeTypes.ServiceConfig) {
	svc.CustomLabels = make(composeTypes.Labels)
	svc.CustomLabels.
		Add(api.ProjectLabel, prj.Name).
		Add(api.ServiceLabel, svc.Name).
		Add(api.VersionLabel, api.ComposeVersion).
		Add(api.WorkingDirLabel, prj.WorkingDir).
		Add(api.ConfigFilesLabel, strings.Join(prj.ComposeFiles, ",")).
		Add(api.OneoffLabel, "False")
}
