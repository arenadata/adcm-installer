package compose

import (
	"context"
	"time"

	"github.com/arenadata/adcm-installer/models"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli/command"
	cliflags "github.com/docker/cli/cli/flags"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/docker/compose/v2/pkg/compose"
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
			Timeout: &timeout,
			//AssumeYes: true,
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

func NewProject(conf *models.Config) *types.Project {
	return &types.Project{
		Name:     *conf.Project,
		Services: make(types.Services),
		Volumes:  make(types.Volumes),
	}
}

func NewADCMProject(conf *models.Config) (*types.Project, error) {
	project := NewProject(conf)
	networkName := *conf.Project
	if networkName == models.ProjectName {
		networkName = models.ProjectName + "_adcm"
	}
	project.Networks = map[string]types.NetworkConfig{networkName: {Name: networkName}}

	if *conf.Postgres.Install {
		if err := addServicePG(project, conf); err != nil {
			return nil, err
		}
	}

	if err := addServiceADCM(project, conf); err != nil {
		return nil, err
	}

	return project, nil
}
