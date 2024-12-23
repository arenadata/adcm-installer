package compose

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/arenadata/adcm-installer/models"
	"github.com/arenadata/adcm-installer/utils"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli/command"
	cliflags "github.com/docker/cli/cli/flags"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/docker/compose/v2/pkg/compose"
)

const (
	ProjectName = "adcm-installer"
)

type Compose struct {
	api api.Service
}

func NewComposeService() (*Compose, error) {
	cli, err := command.NewDockerCli()
	if err != nil {
		return nil, err
	}

	if err = cli.Initialize(&cliflags.ClientOptions{}); err != nil {
		return nil, err
	}

	return &Compose{api: compose.NewComposeService(cli)}, nil
}

func (c Compose) Up(ctx context.Context, prj *types.Project) error {
	timeout := 30 * time.Second

	return c.api.Up(ctx, prj, api.UpOptions{
		Create: api.CreateOptions{
			Timeout:   &timeout,
			AssumeYes: true,
		},
		Start: api.StartOptions{
			Project:     prj,
			Wait:        true,
			WaitTimeout: timeout,
			Services:    prj.ServiceNames(),
		},
	})
}

func (c Compose) Down(ctx context.Context, projectName string, volumes bool) error {
	timeout := 30 * time.Second

	return c.api.Down(ctx, projectName, api.DownOptions{
		Timeout: &timeout,
		Volumes: volumes,
	})
}

func NewProject(name string, conf *models.Config) (*types.Project, error) {
	const defaultNetwork = "adcm_default"
	project := &types.Project{
		Name:     name,
		Services: map[string]types.ServiceConfig{},
		Networks: map[string]types.NetworkConfig{
			defaultNetwork: {Name: defaultNetwork},
		},
	}

	adcm := NewService(project.Name, models.ADCMServiceName, conf.ADCM.Image.String())
	//adcm.CapDrop = []string{"ALL"}
	adcm.Networks = map[string]*types.ServiceNetworkConfig{defaultNetwork: {}}
	adcm.Ports = []types.ServicePortConfig{{
		Protocol:  "tcp",
		Target:    8000,
		Published: "8000",
		Mode:      "ingress",
	}}
	adcm.SecurityOpt = []string{"no-new-privileges"}

	envMappings := []string{
		"DB_HOST=" + *conf.Postgres.Connection.Host,
		fmt.Sprintf("DB_PORT=%d", *conf.Postgres.Connection.Port),
		"DB_USER=" + conf.Secrets.Postgres.Login,
		"DB_PASS=" + conf.Secrets.Postgres.Password,
		"DB_NAME=" + *conf.Postgres.Connection.Database,
	}
	if conf.Postgres.Connection.SSL != nil && conf.Postgres.Connection.SSL.SSLMode != models.PostgresSSLMode {
		b, err := json.Marshal(conf.Postgres.Connection.SSL)
		if err != nil {
			return nil, err
		}
		envMappings = append(envMappings, "DB_OPTIONS="+string(b))
	}
	adcm.Environment = types.NewMappingWithEquals(envMappings)

	if !utils.PtrIsEmpty(conf.ADCM.Volume) {
		if project.Volumes == nil {
			project.Volumes = make(types.Volumes)
		}
		volConfig, err := Volume(*conf.ADCM.Volume, models.ADCMVolumeName, models.ADCMVolumeTarget)
		if err != nil {
			return nil, err
		}
		if volConfig.Type == types.VolumeTypeVolume {
			project.Volumes[models.ADCMVolumeName] = types.VolumeConfig{Name: models.ADCMVolumeName}
		}
		adcm.Volumes = append(adcm.Volumes, volConfig)
	}

	if *conf.Postgres.Install {
		pgHost := conf.Postgres.Connection.Host
		pg := NewService(project.Name, *pgHost, conf.Postgres.Image.String())
		pg.CapDrop = []string{"ALL"}
		pg.User = "postgres:postgres"
		pg.Networks = map[string]*types.ServiceNetworkConfig{defaultNetwork: {}}

		pg.Environment = types.NewMappingWithEquals([]string{
			"PGUSER=" + conf.Secrets.Postgres.Login,
			"POSTGRES_DB=" + *conf.Postgres.Connection.Database,
			"POSTGRES_USER=" + conf.Secrets.Postgres.Login,
			"POSTGRES_PASSWORD=" + conf.Secrets.Postgres.Password,
			"POSTGRES_HOST_AUTH_METHOD=md5",
		})

		if !utils.PtrIsEmpty(conf.Postgres.Volume) {
			if project.Volumes == nil {
				project.Volumes = make(types.Volumes)
			}
			volConfig, err := Volume(*conf.Postgres.Volume, models.PostgresVolumeName, models.PostgresVolumeTarget)
			if err != nil {
				return nil, err
			}
			if volConfig.Type == types.VolumeTypeVolume {
				project.Volumes[models.PostgresVolumeName] = types.VolumeConfig{Name: models.PostgresVolumeName}
			}
			pg.Volumes = append(pg.Volumes, volConfig)
		}

		pg.HealthCheck = &types.HealthCheckConfig{
			Test: types.HealthCheckTest{
				"CMD-SHELL", "pg_isready", "--quiet",
			},
			Interval: utils.Ptr(types.Duration(10 * time.Second)),
			Timeout:  utils.Ptr(types.Duration(3 * time.Second)),
			Retries:  utils.Ptr(uint64(3)),
		}

		project.Services[pg.Name] = pg

		d := types.ServiceDependency{
			Condition: types.ServiceConditionHealthy,
			Required:  true,
			Restart:   true,
		}

		adcm.DependsOn = types.DependsOnConfig{
			*pgHost: d,
		}
	}

	project.Services[adcm.Name] = adcm

	return project, nil
}

func NewService(projectName, name, image string) types.ServiceConfig {
	return types.ServiceConfig{
		Name:          name,
		ContainerName: name,
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
