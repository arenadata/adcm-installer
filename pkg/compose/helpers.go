package compose

import (
	"fmt"
	"strings"
	"time"

	"github.com/arenadata/arenadata-installer/pkg/secrets"
	"github.com/arenadata/arenadata-installer/pkg/utils"

	"github.com/compose-spec/compose-go/v2/format"
	composeTypes "github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/gosimple/slug"
)

type ModHelper func(*composeTypes.Project, *composeTypes.ServiceConfig) error

type ModHelpers []ModHelper

func NewModHelpers() ModHelpers {
	return ModHelpers{}
}

func (h ModHelpers) Run(prj *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
	for _, helper := range h {
		if err := helper(prj, svc); err != nil {
			return err
		}
	}
	return nil
}

func BaseCustomLabels(kind, namespace string) ModHelper {
	return func(prj *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
		svc.CustomLabels = map[string]string{
			api.ProjectLabel:     namespace,
			api.ServiceLabel:     svc.Name,
			api.VersionLabel:     api.ComposeVersion,
			api.OneoffLabel:      "False",
			api.ConfigFilesLabel: "",
			ADLabel:              strings.ToLower(kind),
		}
		return nil
	}
}

func CapAdd(caps ...string) ModHelper {
	return func(_ *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
		if len(svc.CapAdd) == 0 {
			svc.CapAdd = caps
		}
		return nil
	}
}

func CapDrop(caps ...string) ModHelper {
	return func(_ *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
		if len(svc.CapDrop) == 0 {
			svc.CapDrop = caps
		}
		return nil
	}
}

func CapDropAll() ModHelper {
	return CapDrop("ALL")
}

func DependsOn(key string) ModHelper {
	return func(prj *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
		kindName := strings.Split(key, ".")
		if len(kindName) != 2 {
			return fmt.Errorf("invalid depends-on key: %s", key)
		}

		key = Concat("-", kindName[0], kindName[1])
		svc.DependsOn = composeTypes.DependsOnConfig{
			key: composeTypes.ServiceDependency{
				Condition: composeTypes.ServiceConditionHealthy,
				Required:  true,
				Restart:   true,
			},
		}
		return nil
	}
}

type Env struct {
	Name  string
	Value *string
}

func ToEnv(m map[string]*string) []Env {
	envs := make([]Env, len(m))
	i := 0
	for k, v := range m {
		envs[i] = Env{Name: k, Value: v}
		i++
	}
	return envs
}

func Environment(envs ...Env) ModHelper {
	return func(_ *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
		if svc.Environment == nil {
			svc.Environment = make(composeTypes.MappingWithEquals)
		}

		for _, e := range envs {
			if _, ok := svc.Environment[e.Name]; ok {
				continue
			}

			svc.Environment[e.Name] = e.Value
		}
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

func HealthCheck(conf HealthCheckConfig) ModHelper {
	return func(_ *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
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
		return nil
	}
}

type ServiceNetworkConfig struct {
	Name    string
	Aliases []string
}

func Networks(netCfg ...ServiceNetworkConfig) ModHelper {
	return func(_ *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
		svc.Networks = make(map[string]*composeTypes.ServiceNetworkConfig)
		for _, cfg := range netCfg {
			svc.Networks[cfg.Name] = &composeTypes.ServiceNetworkConfig{Aliases: cfg.Aliases}
		}

		return nil
	}
}

func Secrets(sec *secrets.Secrets) ModHelper {
	return func(prj *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
		if len(svc.Name) == 0 {
			return fmt.Errorf("service.name not set")
		}

		if prj.Environment == nil {
			prj.Environment = make(map[string]string)
		}
		if prj.Secrets == nil {
			prj.Secrets = make(composeTypes.Secrets)
		}

		var mode uint32 = 0o440
		for path, file := range sec.Files {
			k := slug.Make(path)
			key := Concat("-", k, svc.Name)

			mountPath := SecretsPath + path
			if path[0] == '/' {
				mountPath = path
			}

			svc.Secrets = append(svc.Secrets, composeTypes.ServiceSecretConfig{
				Source: key,
				Target: mountPath,
				Mode:   &mode,
			})
			prj.Secrets[key] = composeTypes.SecretConfig{Environment: key}
			prj.Environment[key] = file.Data

			if file.EnvKey != nil {
				if len(*file.EnvKey) > 0 {
					svc.Environment[*file.EnvKey] = &mountPath
				} else {
					// generate env key name
					const (
						injEnvKeyPrefix = "SECRET_"
						injEnvKeySuffix = "_FILE"
					)

					envKey := strings.ReplaceAll(k, "-", "_")
					envKey = injEnvKeyPrefix + strings.ToUpper(envKey) + injEnvKeySuffix
					*file.EnvKey = envKey

					svc.Environment[envKey] = &mountPath
				}
			}
		}

		for k, v := range sec.Env {
			svc.Environment[k] = &v
		}

		return nil
	}
}

func ContainerName(ns, kind, name string) ModHelper {
	return func(_ *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
		if len(svc.ContainerName) == 0 {
			svc.ContainerName = containerName(ns, kind, name)
		}
		return nil
	}
}

func Image(image string) ModHelper {
	return func(_ *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
		if len(svc.Image) == 0 {
			svc.Image = image
		}
		return nil
	}
}

func ServiceName(kind, name string) ModHelper {
	return func(_ *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
		if len(svc.Name) == 0 {
			svc.Name = serviceName(kind, name)
		}
		return nil
	}
}

func Configs(configs map[string]string) ModHelper {
	return func(prj *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
		if prj.Configs == nil {
			prj.Configs = make(composeTypes.Configs)
		}

		if len(configs) > 0 && svc.Environment == nil {
			svc.Environment = make(composeTypes.MappingWithEquals)
		}

		var mode uint32 = 0o444
		for path, content := range configs {
			k := slug.Make(path)
			key := svc.Name + "-" + k

			mountPath := ConfigsPath + path
			if path[0] == '/' {
				mountPath = path
			}

			svc.Configs = append(svc.Configs, composeTypes.ServiceConfigObjConfig{
				Source: key,
				Target: mountPath,
				Mode:   &mode,
			})
			prj.Configs[key] = composeTypes.ConfigObjConfig{Environment: key}
			prj.Environment[key] = content
		}
		return nil
	}
}

func Network(networkName string) ModHelper {
	return func(prj *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
		if prj.Networks == nil {
			prj.Networks = make(composeTypes.Networks)
		}
		if _, ok := prj.Networks[networkName]; !ok {
			prj.Networks[networkName] = composeTypes.NetworkConfig{Name: networkName}
		}

		svc.Networks = map[string]*composeTypes.ServiceNetworkConfig{
			networkName: {},
		}

		return nil
	}
}

func Ports(ports []string) ModHelper {
	return func(prj *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
		for _, port := range ports {
			p, err := composeTypes.ParsePortConfig(port)
			if err != nil {
				return err
			}

			svc.Ports = append(svc.Ports, p...)
		}
		return nil
	}
}

func ProjectName(name string) ModHelper {
	return func(prj *composeTypes.Project, _ *composeTypes.ServiceConfig) error {
		if len(prj.Name) == 0 {
			prj.Name = name
		}

		return nil
	}
}

func ReadOnlyRootFilesystem() ModHelper {
	return func(_ *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
		svc.ReadOnly = true
		return nil
	}
}

func SecurityOpts(securityOpts ...string) ModHelper {
	return func(_ *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
		svc.SecurityOpt = securityOpts
		return nil
	}
}

func SecurityOptsNoNewPrivileges() ModHelper {
	return SecurityOpts("no-new-privileges")
}

func TmpFs(mountPoints ...string) ModHelper {
	return func(_ *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
		svc.Tmpfs = mountPoints
		return nil
	}
}

func Volumes(volumes []string) ModHelper {
	return func(prj *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
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
		return nil
	}
}

func User(user, group string) ModHelper {
	return func(_ *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
		if len(svc.User) > 0 {
			return nil
		}
		if len(group) == 0 {
			group = "root"
		}
		svc.User = strings.Join([]string{user, group}, ":")
		return nil
	}
}
