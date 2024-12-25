package compose

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/arenadata/adcm-installer/models"
	"github.com/arenadata/adcm-installer/utils"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/compose/v2/pkg/api"
)

func addServiceADCM(prj *types.Project, conf *models.Config) error {
	svc := NewService(prj.Name, models.ADCMServiceName, conf.ADCM.Image.String())
	svc.CapAdd = []string{"CAP_CHOWN", "CAP_SETUID", "CAP_SETGID"} // nginx
	svc.CapDrop = []string{"ALL"}

	env := map[string]string{
		"DB_HOST": *conf.Postgres.Connection.Host,
		"DB_PORT": strconv.Itoa(*conf.Postgres.Connection.Port),
		"DB_USER": conf.Secrets.SensitiveData.Postgres.Login,
		"DB_PASS": conf.Secrets.SensitiveData.Postgres.Password,
		"DB_NAME": *conf.Postgres.Connection.Database,
	}

	if conf.Postgres.Connection.SSL != nil && conf.Postgres.Connection.SSL.SSLMode != models.PostgresSSLMode {
		containerSecrets := *conf.Postgres.Connection.SSL

		type mnt struct {
			s *string //source
			t string  //target
		}
		for k, v := range map[string]mnt{
			"ca":   {&containerSecrets.SSLRootCert, "/run/secrets/ca.crt"},
			"cert": {&containerSecrets.SSLCert, "/run/secrets/tls.crt"},
			"key":  {&containerSecrets.SSLKey, "/run/secrets/tls.key"},
		} {
			prj.Secrets[k] = types.SecretConfig{Name: k, File: *v.s}
			svc.Secrets = append(svc.Secrets, types.ServiceSecretConfig{Source: k, Target: v.t})
			*v.s = v.t
		}

		b, err := json.Marshal(containerSecrets)
		if err != nil {
			return err
		}
		env["DB_OPTIONS"] = string(b)
	}

	svc.Environment = EnvironmentFromMap(env)

	svc.Networks = map[string]*types.ServiceNetworkConfig{
		prj.NetworkNames()[0]: {
			Aliases: []string{models.ADCMServiceName},
		},
	}

	ports, err := types.ParsePortConfig(fmt.Sprintf(models.ADCMPortPattern, *conf.ADCM.Publish))
	if err != nil {
		return err
	}
	svc.Ports = ports

	svc.SecurityOpt = []string{"no-new-privileges"}

	if !utils.PtrIsEmpty(conf.ADCM.Volume) {
		volConfig, err := Volume(*conf.ADCM.Volume, models.ADCMVolumeName, models.ADCMVolumeTarget)
		if err != nil {
			return err
		}
		if volConfig.Type == types.VolumeTypeVolume {
			prj.Volumes[models.ADCMVolumeName] = types.VolumeConfig{Name: models.ADCMVolumeName}
		}

		svc.Volumes = append(svc.Volumes, volConfig)
	}

	if *conf.Postgres.Install {
		svc.DependsOn = types.DependsOnConfig{
			models.PostgresServiceName: types.ServiceDependency{
				Condition: types.ServiceConditionHealthy,
				Required:  true,
				Restart:   true,
			},
		}
	}

	prj.Services[svc.Name] = svc

	return nil
}

func addServicePG(prj *types.Project, conf *models.Config) error {
	svc := NewService(prj.Name, models.PostgresServiceName, conf.Postgres.Image.String())
	svc.CapDrop = []string{"ALL"}

	svc.Environment = EnvironmentFromMap(map[string]string{
		"PGUSER":            conf.Secrets.SensitiveData.Postgres.Login,
		"POSTGRES_DB":       *conf.Postgres.Connection.Database,
		"POSTGRES_USER":     conf.Secrets.SensitiveData.Postgres.Login,
		"POSTGRES_PASSWORD": conf.Secrets.SensitiveData.Postgres.Password,
	})

	svc.HealthCheck = &types.HealthCheckConfig{
		Test: types.HealthCheckTest{
			"CMD-SHELL", "pg_isready", "--quiet",
		},
		Interval: utils.Ptr(types.Duration(10 * time.Second)),
		Timeout:  utils.Ptr(types.Duration(3 * time.Second)),
		Retries:  utils.Ptr(uint64(3)),
	}

	svc.Networks = map[string]*types.ServiceNetworkConfig{
		prj.NetworkNames()[0]: {
			Aliases: []string{models.PostgresServiceName, *conf.Postgres.Connection.Host},
		},
	}

	if !utils.PtrIsEmpty(conf.Postgres.Volume) {
		volConfig, err := Volume(*conf.Postgres.Volume, models.PostgresVolumeName, models.PostgresVolumeTarget)
		if err != nil {
			return err
		}
		if volConfig.Type == types.VolumeTypeVolume {
			prj.Volumes[models.PostgresVolumeName] = types.VolumeConfig{Name: models.PostgresVolumeName}
		}
		svc.Volumes = append(svc.Volumes, volConfig)
	}

	svc.User = "postgres:postgres"

	prj.Services[svc.Name] = svc

	return nil
}

func NewService(projectName, name, image string) types.ServiceConfig {
	return types.ServiceConfig{
		Name:          name,
		ContainerName: fmt.Sprintf("%s_%s", projectName, name),
		Image:         image,
		Platform:      "linux/amd64",
		Restart:       types.RestartPolicyOnFailure,
		Labels: map[string]string{
			api.ProjectLabel: projectName,
			api.ServiceLabel: name,
			api.OneoffLabel:  "False",
		},
	}
}

func EnvironmentFromMap(env map[string]string) types.MappingWithEquals {
	var values []string
	for k, v := range env {
		values = append(values, fmt.Sprintf("%s=%s", strings.ToUpper(k), v))
	}

	return types.NewMappingWithEquals(values)
}

func Volume(volume, defSrc, defTarget string) (types.ServiceVolumeConfig, error) {
	var volParts []string
	if len(volume) > 0 {
		volParts = strings.Split(volume, ":")
	}

	if runtime.GOOS == "windows" {
		if len(volParts) == 3 {
			// C:\data:/target
			// C:\data:
			volParts = []string{
				volParts[0] + ":" + volParts[1],
				volParts[2],
			}
		} else if len(volParts) == 2 && len(volParts[0]) == 1 {
			// C:\data
			volParts = []string{volume}
		}
	}

	source := defSrc
	target := defTarget
	switch len(volParts) {
	case 0:
		if len(defSrc) == 0 || len(defTarget) == 0 {
			return types.ServiceVolumeConfig{}, fmt.Errorf("no volume defaults provided: source %q, target %q", defSrc, defTarget)
		}
	case 1:
		source = volParts[0]
		if len(defTarget) == 0 {
			return types.ServiceVolumeConfig{}, fmt.Errorf("default target is empty")
		}
	case 2:
		// :/target
		if len(volParts[0]) > 0 {
			source = volParts[0]
		}

		// source:
		if len(volParts[1]) > 0 {
			target = volParts[1]
		}

		if len(target) == 0 {
			return types.ServiceVolumeConfig{}, fmt.Errorf("default target is empty")
		}
	default:
		return types.ServiceVolumeConfig{}, fmt.Errorf("invalid volume format: %s", volume)
	}

	volConfig := types.ServiceVolumeConfig{
		Target: target,
	}
	if utils.IsPath(source) {
		volConfig.Type = types.VolumeTypeBind
		absPath, err := filepath.Abs(source)
		if err != nil {
			return types.ServiceVolumeConfig{}, err
		}
		volConfig.Source = absPath
		volConfig.Bind = &types.ServiceVolumeBind{CreateHostPath: true}
	} else {
		volConfig.Type = types.VolumeTypeVolume
		volConfig.Volume = &types.ServiceVolumeVolume{}
		volConfig.Source = source
	}

	return volConfig, nil
}
