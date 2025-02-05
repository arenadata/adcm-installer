package compose

import (
	"fmt"
	"strings"

	"github.com/arenadata/arenadata-installer/pkg/secrets"

	"github.com/compose-spec/compose-go/v2/format"
	composeTypes "github.com/compose-spec/compose-go/v2/types"
	"github.com/gosimple/slug"
)

type Helper func(*composeTypes.Project, *composeTypes.ServiceConfig) error

type Helpers []Helper

func NewHelpers() Helpers {
	return Helpers{}
}

func (h Helpers) Run(prj *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
	for _, helper := range h {
		if err := helper(prj, svc); err != nil {
			return err
		}
	}
	return nil
}

func DependsOn(key string) Helper {
	return func(prj *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
		kindName := strings.Split(key, ".")
		if len(kindName) != 2 {
			return fmt.Errorf("invalid depends-on key: %s", key)
		}

		key = ServiceName(kindName[0], kindName[1])
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

func Environment(env map[string]*string, notReplaces ...bool) Helper {
	return func(_ *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
		var notReplace bool
		if len(notReplaces) > 0 && notReplaces[0] {
			notReplace = true
		}

		for k, v := range env {
			if notReplace {
				if _, ok := svc.Environment[k]; ok {
					continue
				}
			}

			svc.Environment[k] = v
		}
		return nil
	}
}

func Secrets(sec *secrets.Secrets) Helper {
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
			key := ServiceName(k, svc.Name)

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

func Configs(configs map[string]string) Helper {
	return func(prj *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
		if prj.Configs == nil {
			prj.Configs = make(composeTypes.Configs)
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

func Network(networkName string) Helper {
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

func Ports(ports []string) Helper {
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

func Volumes(volumes []string) Helper {
	return func(prj *composeTypes.Project, svc *composeTypes.ServiceConfig) error {
		for _, volume := range volumes {
			vol, err := format.ParseVolume(volume)
			if err != nil {
				return err
			}
			if vol.Type == composeTypes.VolumeTypeVolume {
				vol.Bind = nil
				prj.Volumes[vol.Source] = composeTypes.VolumeConfig{Name: vol.Source}
			}

			svc.Volumes = append(svc.Volumes, vol)
		}
		return nil
	}
}
