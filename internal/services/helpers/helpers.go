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

package helpers

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/arenadata/adcm-installer/pkg/utils"

	"github.com/compose-spec/compose-go/v2/format"
	composeTypes "github.com/compose-spec/compose-go/v2/types"
	composeUtils "github.com/docker/compose/v2/pkg/utils"
)

const SecretsPath = "/run/adi_secrets"

func (h *ModHelpers) Apply(prj *composeTypes.Project) error {
	for _, helper := range *h {
		if err := helper(prj); err != nil {
			return err
		}
	}
	*h = NewModHelpers()
	return nil
}

func CapAdd(svcName string, caps ...string) ModHelper {
	return func(prj *composeTypes.Project) error {
		if len(caps) == 0 {
			return nil
		}

		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("CapAdd: service %q not found", svcName)
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
			return fmt.Errorf("CapDrop: service %q not found", svcName)
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
			return fmt.Errorf("Command: service %q not found", svcName)
		}

		svc.Command = command

		prj.Services[svcName] = svc
		return nil
	}
}

func ContainerName(svcName string) ModHelper {
	return func(prj *composeTypes.Project) error {
		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("ContainerName: service %q not found", svcName)
		}

		svc.ContainerName = prj.Name + "-" + svc.Name

		prj.Services[svcName] = svc
		return nil
	}
}

func DependsOn(svcName string, dependsOnService ...Depended) ModHelper {
	return func(prj *composeTypes.Project) error {
		if len(dependsOnService) == 0 {
			return nil
		}

		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("DependsOn: service %q not found", svcName)
		}

		if svc.DependsOn == nil {
			svc.DependsOn = make(composeTypes.DependsOnConfig)
		}

		for _, dep := range dependsOnService {
			cond := dep.Condition
			if len(cond) == 0 {
				cond = composeTypes.ServiceConditionHealthy
			}

			svc.DependsOn[dep.Service] = composeTypes.ServiceDependency{
				Condition: cond,
				Required:  dep.Required,
				Restart:   dep.Restart,
			}
		}

		prj.Services[svcName] = svc
		return nil
	}
}

func Entrypoint(svcName string, name string, args ...string) ModHelper {
	return func(prj *composeTypes.Project) error {
		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("Entrypoint: service %q not found", svcName)
		}

		svc.Entrypoint = append([]string{name}, args...)
		prj.Services[svcName] = svc
		return nil
	}
}

func Environment(svcName string, envs ...Env) ModHelper {
	return func(prj *composeTypes.Project) error {
		if len(envs) == 0 {
			return nil
		}

		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("Environment: service %q not found", svcName)
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

func Extension(svcName, name string, v any) ModHelper {
	return func(prj *composeTypes.Project) error {
		if len(svcName) == 0 {
			if prj.Extensions == nil {
				prj.Extensions = make(composeTypes.Extensions)
			}
			prj.Extensions[name] = v
			return nil
		}

		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("Extension: service %q not found", svcName)
		}

		if svc.Extensions == nil {
			svc.Extensions = make(composeTypes.Extensions)
		}
		svc.Extensions[name] = v

		prj.Services[svcName] = svc
		return nil
	}
}

func HealthCheck(svcName string, conf HealthCheckConfig) ModHelper {
	return func(prj *composeTypes.Project) error {
		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("HealthCheck: service %q not found", svcName)
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
			return fmt.Errorf("Hostname: service %q not found", svcName)
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
			return fmt.Errorf("Image: service %q not found", svcName)
		}

		svc.Image = image

		prj.Services[svcName] = svc
		return nil
	}
}

func CustomLabels(svcName string, labels map[string]string) ModHelper {
	return func(prj *composeTypes.Project) error {
		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("CustomLabels: service %q not found", svcName)
		}

		for k, v := range labels {
			svc.CustomLabels.Add(k, v)
		}

		prj.Services[svcName] = svc
		return nil
	}
}

func Labels(svcName string, labels map[string]string) ModHelper {
	return func(prj *composeTypes.Project) error {
		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("Labels: service %q not found", svcName)
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

func ProjectNetwork(name string, conf *composeTypes.NetworkConfig) ModHelper {
	return func(prj *composeTypes.Project) error {
		if prj.Networks == nil {
			prj.Networks = make(composeTypes.Networks)
		}

		if conf == nil {
			conf = &composeTypes.NetworkConfig{Name: fmt.Sprintf("%s_%s", prj.Name, name)}
		}
		prj.Networks[name] = *conf
		return nil
	}
}

func Network(svcName, name string, conf *composeTypes.ServiceNetworkConfig) ModHelper {
	return func(prj *composeTypes.Project) error {
		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("Network: service %q not found", svcName)
		}

		if svc.Networks == nil {
			svc.Networks = make(map[string]*composeTypes.ServiceNetworkConfig)
		}
		svc.Networks[name] = conf

		prj.Services[svcName] = svc
		return nil
	}
}

func (s Secret) fileMode() *composeTypes.FileMode {
	if s.FileMode == 0 {
		return nil
	}

	m := composeTypes.FileMode(s.FileMode)
	return &m
}

func Secrets(svcName string, secrets ...Secret) ModHelper {
	return func(prj *composeTypes.Project) error {
		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("Secrets: service %q not found", svcName)
		}

		hasSecret := func(sec []composeTypes.ServiceSecretConfig, s Secret) bool {
			for _, svcSecret := range sec {
				if svcSecret.Source == s.Source {
					return true
				}
			}
			return false
		}

		for _, sec := range secrets {
			if hasSec := hasSecret(svc.Secrets, sec); hasSec {
				continue
			}

			target := sec.Target
			if len(target) == 0 {
				target = path.Join(SecretsPath, sec.Source)
			}

			svc.Secrets = append(svc.Secrets, composeTypes.ServiceSecretConfig{
				Source: sec.Source,
				Target: target,
				Mode:   sec.fileMode(),
			})

			if len(sec.EnvFileKey) > 0 {
				svc.Environment[sec.EnvFileKey] = &target
			}

			if len(sec.EnvKey) > 0 {
				svc.Environment[sec.EnvKey] = &sec.Value
			}

		}

		prj.Services[svcName] = svc
		return nil
	}
}

func ProjectSecrets(secrets ...Secret) ModHelper {
	return func(prj *composeTypes.Project) error {
		if prj.Secrets == nil {
			prj.Secrets = make(composeTypes.Secrets)
		}
		if prj.Environment == nil {
			prj.Environment = make(composeTypes.Mapping)
		}

		for _, sec := range secrets {
			prj.Secrets[sec.Source] = composeTypes.SecretConfig{
				Name:        sec.Source,
				Environment: sec.Source,
			}
			prj.Environment[sec.Source] = sec.Value
		}

		return nil
	}
}

func SecretsPermission(svcName string, secret Secret) ModHelper {
	return func(prj *composeTypes.Project) error {
		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("SecretsPermission: service %q not found", svcName)
		}

		if len(secret.UID) == 0 && len(secret.GID) == 0 {
			return nil
		}

		for i := range svc.Secrets {
			svc.Secrets[i].UID = secret.UID
			svc.Secrets[i].GID = secret.GID
			svc.Secrets[i].Mode = nil
		}

		prj.Services[svcName] = svc
		return nil
	}
}

func Platform(svcName, platform string) ModHelper {
	return func(prj *composeTypes.Project) error {
		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("Platform: service %q not found", svcName)
		}

		svc.Platform = platform

		prj.Services[svcName] = svc
		return nil
	}
}

func Profiles(svcName string, profiles ...string) ModHelper {
	return func(prj *composeTypes.Project) error {
		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("Profiles: service %q not found", svcName)
		}

		svc.Profiles = profiles

		prj.Services[svcName] = svc
		return nil
	}
}

func PublishPort(svcName string, publishPort, targetPort uint16) ModHelper {
	return func(prj *composeTypes.Project) error {
		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("PublishPort: service %q not found", svcName)
		}

		publishPortString := strconv.Itoa(int(publishPort))
		for _, svcPort := range svc.Ports {
			if svcPort.Published == publishPortString {
				return nil
			}
		}

		ports, err := composeTypes.ParsePortConfig(fmt.Sprintf("%d:%d", publishPort, targetPort))
		if err != nil {
			return err
		}
		svc.Ports = append(svc.Ports, ports...)

		prj.Services[svcName] = svc
		return nil
	}
}

func PullPolicy(svcName, pullPolicy string) ModHelper {
	return func(prj *composeTypes.Project) error {
		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("PullPolicy: service %q not found", svcName)
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
			return fmt.Errorf("ReadOnlyRootFilesystem: service %q not found", svcName)
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
			return fmt.Errorf("RestartPolicy: service %q not found", svcName)
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
			return fmt.Errorf("SecurityOpts: service %q not found", svcName)
		}

		svc.SecurityOpt = securityOpts
		prj.Services[svcName] = svc
		return nil
	}
}

func SecurityOptsNoNewPrivileges(svcName string) ModHelper {
	return SecurityOpts(svcName, "no-new-privileges")
}

func (fs TmpFs) Mount() string {
	if len(fs.MountOptions) == 0 {
		return fs.Target
	}
	return fmt.Sprintf("%s:%s", fs.Target, strings.Join(fs.MountOptions.Values(), ","))
}

func MountTmpFs(svcName string, mounts ...TmpFs) ModHelper {
	return func(prj *composeTypes.Project) error {
		if len(mounts) == 0 {
			return nil
		}

		svc, ok := prj.Services[svcName]
		if !ok {
			return fmt.Errorf("MountTmpFs: service %q not found", svcName)
		}

		for _, mnt := range mounts {
			target := mnt.Mount()
			if !composeUtils.StringContains(svc.Tmpfs, target) {
				svc.Tmpfs = append(svc.Tmpfs, target)
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
			return fmt.Errorf("Volumes: service %q not found", svcName)
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
			return fmt.Errorf("User: service %q not found", svcName)
		}

		if len(svc.User) > 0 || len(user) == 0 {
			return nil
		}

		uids := []string{user}
		if len(group) > 0 {
			uids = append(uids, group)
		}

		svc.User = strings.Join(uids, ":")

		prj.Services[svcName] = svc
		return nil
	}
}
