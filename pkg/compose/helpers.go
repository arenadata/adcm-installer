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

package compose

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/arenadata/adcm-installer/pkg/utils"

	"github.com/compose-spec/compose-go/v2/format"
	composeTypes "github.com/compose-spec/compose-go/v2/types"
)

type ModHelper func(*composeTypes.Project) error

type ModHelpers []ModHelper

func NewModHelpers() ModHelpers {
	return ModHelpers{}
}

func (h ModHelpers) Run(prj *composeTypes.Project) error {
	for _, helper := range h {
		if err := helper(prj); err != nil {
			return err
		}
	}
	return nil
}

func CapAdd(svcName string, caps ...string) ModHelper {
	return func(prj *composeTypes.Project) error {
		if len(caps) == 0 {
			return nil
		}

		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("service %q not found", svcName)
		}

		if len(svc.CapAdd) > 0 {
			return nil
		}
		svc.CapAdd = caps

		prj.Services[svcName] = svc
		return nil
	}
}

func CapDrop(svcName string, caps ...string) ModHelper {
	return func(prj *composeTypes.Project) error {
		if len(caps) == 0 {
			return nil
		}

		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("service %q not found", svcName)
		}

		if len(svc.CapDrop) > 0 {
			return nil
		}
		svc.CapDrop = caps

		prj.Services[svcName] = svc
		return nil
	}
}

func CapDropAll(svcName string) ModHelper {
	return CapDrop(svcName, "ALL")
}

func Command(svcName string, command []string) ModHelper {
	return func(prj *composeTypes.Project) error {
		if len(command) == 0 {
			return nil
		}

		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("service %q not found", svcName)
		}

		svc.Command = command

		prj.Services[svcName] = svc
		return nil
	}
}

type Depended struct {
	Service   string
	Condition string
}

func DependsOn(svcName string, dependsOnService ...Depended) ModHelper {
	return func(prj *composeTypes.Project) error {
		if len(dependsOnService) == 0 {
			return nil
		}

		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("service %q not found", svcName)
		}

		svc.DependsOn = composeTypes.DependsOnConfig{}
		for _, dep := range dependsOnService {
			cond := dep.Condition
			if len(cond) == 0 {
				cond = composeTypes.ServiceConditionHealthy
			}
			svc.DependsOn[dep.Service] = composeTypes.ServiceDependency{
				Condition: cond,
				Required:  true,
				Restart:   true,
			}
		}

		prj.Services[svcName] = svc
		return nil
	}
}

type Env struct {
	Name  string
	Value *string
}

func Environment(svcName string, envs ...Env) ModHelper {
	return func(prj *composeTypes.Project) error {
		if len(envs) == 0 {
			return nil
		}

		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("service %q not found", svcName)
		}

		if svc.Environment == nil {
			svc.Environment = make(composeTypes.MappingWithEquals)
		}

		for _, e := range envs {
			svc.Environment[e.Name] = e.Value
		}

		prj.Services[svcName] = svc
		return nil
	}
}

type HealthCheckConfig struct {
	Cmd           []string
	Interval      time.Duration
	Timeout       time.Duration
	StartPeriod   time.Duration
	StartInterval time.Duration
	Retries       uint64
}

func HealthCheck(svcName string, conf HealthCheckConfig) ModHelper {
	return func(prj *composeTypes.Project) error {
		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("service %q not found", svcName)
		}

		svc.HealthCheck = &composeTypes.HealthCheckConfig{
			Test: conf.Cmd,
		}
		if conf.Interval > 0 {
			svc.HealthCheck.Interval = utils.Ptr(composeTypes.Duration(conf.Interval))
		}
		if conf.Timeout > 0 {
			svc.HealthCheck.Timeout = utils.Ptr(composeTypes.Duration(conf.Timeout))
		}
		if conf.StartPeriod > 0 {
			svc.HealthCheck.StartPeriod = utils.Ptr(composeTypes.Duration(conf.StartPeriod))
		}
		if conf.StartInterval > 0 {
			svc.HealthCheck.StartInterval = utils.Ptr(composeTypes.Duration(conf.StartInterval))
		}
		if conf.Retries > 0 {
			svc.HealthCheck.Retries = utils.Ptr(conf.Retries)
		}

		prj.Services[svcName] = svc
		return nil
	}
}

func Hostname(svcName, hostname string) ModHelper {
	return func(prj *composeTypes.Project) error {
		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("service %q not found", svcName)
		}

		svc.Hostname = hostname

		prj.Services[svcName] = svc
		return nil
	}
}

func Image(svcName, image string) ModHelper {
	return func(prj *composeTypes.Project) error {
		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("service %q not found", svcName)
		}

		svc.Image = image

		prj.Services[svcName] = svc
		return nil
	}
}

func Labels(svcName string, labels map[string]string) ModHelper {
	return func(prj *composeTypes.Project) error {
		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("service %q not found", svcName)
		}
		if svc.Labels == nil {
			svc.Labels = labels
		} else {
			for k, v := range labels {
				svc.Labels[k] = v
			}
		}

		prj.Services[svcName] = svc
		return nil
	}
}

type Secret struct {
	Source   string
	EnvKey   string
	Value    string
	ENV      bool
	Rewrite  bool
	FileMode *uint32
}

func Secrets(svcName string, secrets ...Secret) ModHelper {
	return func(prj *composeTypes.Project) error {
		hasSource := func(sec []composeTypes.ServiceSecretConfig, s Secret) (int, bool) {
			for n, svcSecret := range sec {
				if svcSecret.Source == s.Source {
					return n, true
				}
			}
			return 0, false
		}
		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("service %q not found", svcName)
		}

		if prj.Environment == nil {
			prj.Environment = make(composeTypes.Mapping)
		}
		if prj.Secrets == nil {
			prj.Secrets = make(composeTypes.Secrets)
		}

		for _, sec := range secrets {
			filepath := path.Join(SecretsPath, sec.Source)
			n, hasSrc := hasSource(svc.Secrets, sec)
			var fileMode *composeTypes.FileMode
			if sec.FileMode != nil {
				fileMode = utils.Ptr(composeTypes.FileMode(*sec.FileMode))
			}

			if !hasSrc {
				svc.Secrets = append(svc.Secrets, composeTypes.ServiceSecretConfig{
					Source: sec.Source,
					Target: filepath,
					Mode:   fileMode,
				})
			} else if sec.FileMode != nil {
				svc.Secrets[n].Mode = fileMode
			}

			prj.Secrets[sec.Source] = composeTypes.SecretConfig{Environment: sec.Source}
			if _, ok := prj.Environment[sec.Source]; !ok || sec.Rewrite {
				prj.Environment[sec.Source] = sec.Value
			}

			if len(sec.EnvKey) > 0 {
				svc.Environment[sec.EnvKey+"_FILE"] = &filepath

				if sec.ENV {
					svc.Environment[sec.EnvKey] = &sec.Value
				}

			}
		}

		prj.Services[svcName] = svc
		return nil
	}
}

func Platform(svcName, platform string) ModHelper {
	return func(prj *composeTypes.Project) error {
		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("service %q not found", svcName)
		}

		svc.Platform = platform

		prj.Services[svcName] = svc
		return nil
	}
}

func PublishPort(svcName string, publishPort, targetPort uint16) ModHelper {
	return func(prj *composeTypes.Project) error {
		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("service %q not found", svcName)
		}

		publishPortString := strconv.Itoa(int(publishPort))
		for _, svcPort := range svc.Ports {
			if svcPort.Published == publishPortString {
				return nil
			}
		}

		svc.Ports = append(svc.Ports, composeTypes.ServicePortConfig{
			Mode:      "ingress",
			Target:    uint32(targetPort),
			Published: publishPortString,
		})

		prj.Services[svcName] = svc
		return nil
	}
}

func PullPolicy(svcName, pullPolicy string) ModHelper {
	return func(prj *composeTypes.Project) error {
		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("service %q not found", svcName)
		}

		svc.PullPolicy = pullPolicy

		prj.Services[svcName] = svc
		return nil
	}
}

func ReadOnlyRootFilesystem(svcName string) ModHelper {
	return func(prj *composeTypes.Project) error {
		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("service %q not found", svcName)
		}

		svc.ReadOnly = true

		prj.Services[svcName] = svc
		return nil
	}
}

func RestartPolicy(svcName, restartPolicy string) ModHelper {
	return func(prj *composeTypes.Project) error {
		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("service %q not found", svcName)
		}

		svc.Restart = restartPolicy

		prj.Services[svcName] = svc
		return nil
	}
}

func SecurityOpts(svcName string, securityOpts ...string) ModHelper {
	return func(prj *composeTypes.Project) error {
		if len(securityOpts) == 0 {
			return nil
		}

		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("service %q not found", svcName)
		}

		svc.SecurityOpt = securityOpts
		prj.Services[svcName] = svc
		return nil
	}
}

func SecurityOptsNoNewPrivileges(svcName string) ModHelper {
	return SecurityOpts(svcName, "no-new-privileges")
}

func TmpFs(svcName string, mountPoints ...string) ModHelper {
	return func(prj *composeTypes.Project) error {
		if len(mountPoints) == 0 {
			return nil
		}

		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("service %q not found", svcName)
		}

		for _, mountPoint := range mountPoints {
			if !utils.In(svc.Tmpfs, mountPoint) {
				svc.Tmpfs = append(svc.Tmpfs, mountPoint)
			}
		}

		prj.Services[svcName] = svc
		return nil
	}
}

func Volumes(svcName string, volumes ...string) ModHelper {
	return func(prj *composeTypes.Project) error {
		if len(volumes) == 0 {
			return nil
		}

		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("service %q not found", svcName)
		}

		for _, volume := range volumes {
			vol, err := format.ParseVolume(volume)
			if err != nil {
				return err
			}
			if vol.Type == composeTypes.VolumeTypeVolume {
				if prj.Volumes == nil {
					prj.Volumes = make(composeTypes.Volumes)
				}

				vol.Bind = nil
				prj.Volumes[vol.Source] = composeTypes.VolumeConfig{Name: vol.Source}
			}

			svc.Volumes = append(svc.Volumes, vol)
		}

		prj.Services[svcName] = svc
		return nil
	}
}

func User(svcName, user, group string) ModHelper {
	return func(prj *composeTypes.Project) error {
		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("service %q not found", svcName)
		}

		if len(svc.User) > 0 {
			return nil
		}
		if len(group) == 0 {
			group = "root"
		}
		svc.User = strings.Join([]string{user, group}, ":")

		prj.Services[svcName] = svc
		return nil
	}
}
