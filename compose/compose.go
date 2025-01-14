package compose

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/arenadata/adcm-installer/models"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/containerd/platforms"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/command/container"
	cliflags "github.com/docker/cli/cli/flags"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/docker/compose/v2/pkg/compose"
	"github.com/docker/compose/v2/pkg/utils"
	moby "github.com/docker/docker/api/types"
	containerTypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/sirupsen/logrus"
)

type Compose struct {
	cli *command.DockerCli
	svc api.Service
}

func NewComposeService(ops ...command.CLIOption) (*Compose, error) {
	cli, err := command.NewDockerCli(ops...)
	if err != nil {
		return nil, err
	}

	if err = cli.Initialize(&cliflags.ClientOptions{}); err != nil {
		return nil, err
	}

	return &Compose{svc: compose.NewComposeService(cli), cli: cli}, nil
}

func (c Compose) Up(ctx context.Context, prj *types.Project) error {
	timeout := 30 * time.Second

	return c.svc.Up(ctx, prj, api.UpOptions{
		Create: api.CreateOptions{
			Timeout: &timeout,
			//AssumeYes: true,
		},
		Start: api.StartOptions{
			//Project:     prj,
			Wait:        true,
			WaitTimeout: timeout,
			Services:    prj.ServiceNames(),
		},
	})
}

func (c Compose) Down(ctx context.Context, projectName string, volumes bool) error {
	timeout := 30 * time.Second

	return c.svc.Down(ctx, projectName, api.DownOptions{
		Timeout: &timeout,
		Volumes: volumes,
	})
}

func (c Compose) list(ctx context.Context, all bool, filter ...filters.KeyValuePair) ([]moby.Container, error) {
	return c.cli.Client().ContainerList(ctx, containerTypes.ListOptions{
		All:     all,
		Filters: filters.NewArgs(filter...),
	})
}

func (c Compose) GetProject(ctx context.Context, projectName string) (*types.Project, error) {
	return c.svc.Generate(ctx, api.GenerateOptions{ProjectName: projectName})
}

func (c Compose) Exec(ctx context.Context, containerName, cmd string, args ...string) error {
	exec := container.NewExecOptions()
	exec.Command = []string{cmd}
	exec.Command = append(exec.Command, args...)
	exec.Interactive = true
	exec.TTY = true

	return container.RunExec(ctx, c.cli, containerName, exec)
}

func (c Compose) List(ctx context.Context, all bool) ([]api.Stack, error) {
	list, err := c.list(ctx, all,
		hasProjectLabelFilter(),
		hasConfigHashLabel(),
		hasConfigADLabel(),
	)
	if err != nil {
		return nil, err
	}

	return containersToStacks(list)
}

func (c Compose) ContainerRun(
	ctx context.Context,
	platform platforms.Platform,
	hostConfig *containerTypes.HostConfig,
	networkConfig *network.NetworkingConfig,
	containerConfig *containerTypes.Config,
	containerName string,
) error {
	dockerCli := c.cli.Client()
	resp, err := dockerCli.ContainerCreate(ctx,
		containerConfig,
		hostConfig,
		networkConfig,
		&platform,
		containerName,
	)
	if err != nil {
		return err
	}

	// TODO: maybe use docker cli?
	//container.NewRunCommand()

	if err = dockerCli.ContainerStart(ctx, resp.ID, containerTypes.StartOptions{}); err != nil {
		return err
	}

	newContainer, err := dockerCli.ContainerInspect(ctx, resp.ID)
	if err != nil {
		return err
	}

	var count int
	for newContainer.State.Status != "running" {
		if count == 10 {
			return fmt.Errorf("timeout waiting for container to start")
		}

		newContainer, err = dockerCli.ContainerInspect(ctx, resp.ID)
		if err != nil {
			return err
		}

		count++
		time.Sleep(3 * time.Second)
	}

	return nil
}

func (c Compose) ContainerRemove(ctx context.Context, containerName string) error {
	return c.cli.Client().ContainerRemove(ctx, containerName, containerTypes.RemoveOptions{
		Force: true,
	})
}

func (c Compose) Stop(ctx context.Context, projectName string, services ...string) error {
	return c.svc.Stop(ctx, projectName, api.StopOptions{Services: services})
}

func (c Compose) Start(ctx context.Context, projectName string, services ...string) error {
	return c.svc.Start(ctx, projectName, api.StartOptions{Services: services})
}

func (c Compose) Pause(ctx context.Context, projectName string, services ...string) error {
	return c.svc.Pause(ctx, projectName, api.PauseOptions{Services: services})
}

func (c Compose) UnPause(ctx context.Context, projectName string, services ...string) error {
	return c.svc.UnPause(ctx, projectName, api.PauseOptions{Services: services})
}

func NewProject(conf *models.Config) *types.Project {
	return &types.Project{
		Name:     *conf.DeploymentID,
		Services: make(types.Services),
		Volumes:  make(types.Volumes),
	}
}

func NewADCMProject(conf *models.Config, configFilePath string) (*types.Project, error) {
	project := NewProject(conf)
	networkName := *conf.DeploymentID
	if networkName == models.DeploymentId {
		networkName = models.DeploymentId + "_adcm"
	}
	project.Networks = map[string]types.NetworkConfig{networkName: {Name: networkName}}
	project.ComposeFiles = []string{configFilePath}

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

func hasProjectLabelFilter() filters.KeyValuePair {
	return filters.Arg("label", api.ProjectLabel)
}

func hasConfigHashLabel() filters.KeyValuePair {
	return filters.Arg("label", api.ConfigHashLabel)
}

func hasConfigADLabel() filters.KeyValuePair {
	return filters.Arg("label", models.ADLabel)
}

func containersToStacks(containers []moby.Container) ([]api.Stack, error) {
	containersByLabel, keys, err := groupContainerByLabel(containers, api.ProjectLabel)
	if err != nil {
		return nil, err
	}
	var projects []api.Stack
	for _, project := range keys {
		configFiles, err := combinedConfigFiles(containersByLabel[project])
		if err != nil {
			logrus.Warn(err.Error())
			configFiles = "N/A"
		}

		projects = append(projects, api.Stack{
			ID:          project,
			Name:        project,
			Status:      combinedStatus(containerToState(containersByLabel[project])),
			ConfigFiles: configFiles,
		})
	}
	return projects, nil
}

func combinedConfigFiles(containers []moby.Container) (string, error) {
	var configFiles []string

	for _, c := range containers {
		files, ok := c.Labels[api.ConfigFilesLabel]
		if !ok {
			return "", fmt.Errorf("no label %q set on container %q of compose project", api.ConfigFilesLabel, c.ID)
		}

		for _, f := range strings.Split(files, ",") {
			if !utils.StringContains(configFiles, f) {
				configFiles = append(configFiles, f)
			}
		}
	}

	return strings.Join(configFiles, ","), nil
}

func containerToState(containers []moby.Container) []string {
	var statuses []string
	for _, c := range containers {
		statuses = append(statuses, c.State)
	}
	return statuses
}

func combinedStatus(statuses []string) string {
	nbByStatus := map[string]int{}
	var keys []string
	for _, status := range statuses {
		nb, ok := nbByStatus[status]
		if !ok {
			nb = 0
			keys = append(keys, status)
		}
		nbByStatus[status] = nb + 1
	}
	sort.Strings(keys)
	result := ""
	for _, status := range keys {
		nb := nbByStatus[status]
		if result != "" {
			result += ", "
		}
		result += fmt.Sprintf("%s(%d)", status, nb)
	}
	return result
}

func groupContainerByLabel(containers []moby.Container, labelName string) (map[string][]moby.Container, []string, error) {
	containersByLabel := map[string][]moby.Container{}
	var keys []string
	for _, c := range containers {
		label, ok := c.Labels[labelName]
		if !ok {
			return nil, nil, fmt.Errorf("no label %q set on container %q of compose project", labelName, c.ID)
		}
		labelContainers, ok := containersByLabel[label]
		if !ok {
			labelContainers = []moby.Container{}
			keys = append(keys, label)
		}
		labelContainers = append(labelContainers, c)
		containersByLabel[label] = labelContainers
	}
	sort.Strings(keys)
	return containersByLabel, keys, nil
}
