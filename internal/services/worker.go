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
	"maps"
	"slices"
	"sort"

	"github.com/Masterminds/semver/v3"
	"github.com/arenadata/adcm-installer/pkg/compose"
	composeTypes "github.com/compose-spec/compose-go/v2/types"
	"github.com/distribution/reference"
	log "github.com/sirupsen/logrus"
)

var adcmWorkerCommand = []string{"/etc/worker.sh"}

func (prj *Project) workerInstances() {
	count := *prj.config.Adcm.WorkerCount
	if count == 0 {
		return
	}

	var names []string
	for name, svc := range prj.prj.Services {
		if svc.Labels[compose.ADAppTypeLabelKey] == AdcmName {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return
	}
	sort.Strings(names)

	// the ADCM instances share their data volume, so any of them describes the
	// installation the workers run the jobs for
	adcmName := names[0]

	tag := imageTag(prj.prj.Services[adcmName].Image)
	if !adcmWorkerSupported(tag) {
		checkErr(fmt.Errorf("ADCM %s has no standalone Celery worker (requires %s or newer), "+
			"set job-execution-environment to %q", tag, ADCMWorkerMinTag, JobExecEnvLocal))
	}

	if count == 1 {
		prj.worker(adcmName, WorkerName)
		return
	}
	for i := uint8(1); i <= count; i++ {
		prj.worker(adcmName, fmt.Sprintf("%s-%d", WorkerName, i))
	}
}

func (prj *Project) worker(adcmName, name string) {
	adcmSvc := prj.prj.Services[adcmName]
	addService(name, prj.prj)
	svc := prj.prj.Services[name]

	svc.Labels = map[string]string{
		compose.ADAppTypeLabelKey: WorkerName,
		compose.ADAppAdcmLabelKey: adcmName,
	}
	svc.Hostname = prj.hostname(name)
	svc.Restart = adcmSvc.Restart
	svc.Command = adcmWorkerCommand
	svc.Image = adcmSvc.Image
	svc.CapAdd = slices.Clone(adcmSvc.CapAdd)
	svc.CapDrop = slices.Clone(adcmSvc.CapDrop)
	svc.SecurityOpt = slices.Clone(adcmSvc.SecurityOpt)
	svc.Environment = maps.Clone(adcmSvc.Environment)
	svc.Secrets = slices.Clone(adcmSvc.Secrets)
	svc.Volumes = slices.Clone(adcmSvc.Volumes)
	svc.Networks = maps.Clone(adcmSvc.Networks)
	svc.DependsOn = maps.Clone(adcmSvc.DependsOn)

	svc.DependsOn[adcmName] = composeTypes.ServiceDependency{
		Condition: composeTypes.ServiceConditionHealthy,
		Required:  true,
	}

	prj.prj.Services[name] = svc
}

func adcmWorkerSupported(tag string) bool {
	v, err := semver.NewVersion(tag)
	if err != nil {
		// Non-semver rolling tags (e.g. "develop") track the current release, which ships the worker.
		log.Debugf("ADCM tag %q is not a semver version; assuming the Celery worker "+
			"is supported (ADCM %s or newer)", tag, ADCMWorkerMinTag)
		return true
	}

	return !v.LessThan(semver.MustParse(ADCMWorkerMinTag))
}

func imageTag(image string) string {
	named, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return ""
	}
	if tagged, ok := named.(reference.NamedTagged); ok {
		return tagged.Tag()
	}
	return ""
}
