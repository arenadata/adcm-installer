package compose

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/arenadata/adcm-installer/models"
	"github.com/arenadata/adcm-installer/utils"

	"github.com/compose-spec/compose-go/v2/format"
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
		vol, err := format.ParseVolume(*conf.ADCM.Volume)
		if err != nil {
			return err
		}
		if vol.Type == types.VolumeTypeVolume {
			prj.Volumes[models.ADCMVolumeName] = types.VolumeConfig{Name: models.ADCMVolumeName}
		}

		svc.Volumes = append(svc.Volumes, vol)
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
		vol, err := format.ParseVolume(*conf.Postgres.Volume)
		if err != nil {
			return err
		}
		if vol.Type == types.VolumeTypeVolume {
			prj.Volumes[models.PostgresVolumeName] = types.VolumeConfig{Name: models.PostgresVolumeName}
		}
		svc.Volumes = append(svc.Volumes, vol)
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
