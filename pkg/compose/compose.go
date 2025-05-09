package compose

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli/command"
	cliflags "github.com/docker/cli/cli/flags"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/docker/compose/v2/pkg/compose"
	"github.com/docker/compose/v2/pkg/utils"
	moby "github.com/docker/docker/api/types"
	containerTypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/system"
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
		},
		Start: api.StartOptions{
			Wait:        true,
			WaitTimeout: timeout,
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

func (c Compose) Info(ctx context.Context) (system.Info, error) {
	return c.cli.Client().Info(ctx)
}

func hasProjectLabelFilter() filters.KeyValuePair {
	return filters.Arg("label", api.ProjectLabel)
}

func hasConfigHashLabel() filters.KeyValuePair {
	return filters.Arg("label", api.ConfigHashLabel)
}

func hasConfigADLabel() filters.KeyValuePair {
	return filters.Arg("label", ADLabel)
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
