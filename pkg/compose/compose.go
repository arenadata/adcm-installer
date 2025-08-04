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
	composeUtils "github.com/docker/compose/v2/pkg/utils"
	"github.com/docker/docker/api/types/container"
	containerTypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/system"
	"github.com/sirupsen/logrus"
)

const (
	ADLabel             = "app.arenadata.io"
	ADAppTypeLabelKey   = ADLabel + "/type"
	ADVaultModeLabelKey = ADLabel + "/vault-mode"

	DefaultNetwork  = "default"
	DefaultPlatform = "linux/amd64"
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

func (c Compose) Cli() *command.DockerCli {
	return c.cli
}

func (c Compose) Exec(ctx context.Context, prj *types.Project, serviceName string, name string, args ...string) error {
	_, err := c.svc.Exec(ctx, prj.Name, api.RunOptions{
		Project: prj,
		Service: serviceName,
		Command: append([]string{name}, args...),
		Tty:     true,
	})
	return err
}

func (c Compose) Remove(ctx context.Context, prj *types.Project, services ...string) error {
	return c.svc.Remove(ctx, prj.Name, api.RemoveOptions{
		Project:  prj,
		Stop:     true,
		Force:    true,
		Services: services,
	})
}

func (c Compose) Up(ctx context.Context, prj *types.Project, wait bool) error {
	timeout := 30 * time.Second

	return c.svc.Up(ctx, prj, api.UpOptions{
		Create: api.CreateOptions{
			Timeout:              &timeout,
			Recreate:             "diverged",
			RecreateDependencies: "diverged",
		},
		Start: api.StartOptions{
			Project:     prj,
			Wait:        wait,
			WaitTimeout: timeout,
		},
	})
}

func (c Compose) Down(ctx context.Context, prj *types.Project, volumes bool) error {
	timeout := 30 * time.Second

	var project *types.Project
	if len(prj.Services) > 0 {
		project = prj
	}

	return c.svc.Down(ctx, prj.Name, api.DownOptions{
		RemoveOrphans: true,
		Project:       project,
		Timeout:       &timeout,
		Volumes:       volumes,
	})
}

func (c Compose) Info(ctx context.Context) (system.Info, error) {
	return c.cli.Client().Info(ctx)
}

func (c Compose) list(ctx context.Context, all bool, filter ...filters.KeyValuePair) ([]container.Summary, error) {
	return c.cli.Client().ContainerList(ctx, containerTypes.ListOptions{
		All:     all,
		Filters: filters.NewArgs(filter...),
	})
}

func (c Compose) List(ctx context.Context, all bool) ([]container.Summary, error) {
	return c.list(ctx, all,
		filters.Arg("label", api.ProjectLabel),
		filters.Arg("label", api.ConfigHashLabel),
		filters.Arg("label", ADLabel),
	)
}

func (c Compose) ListProjects(ctx context.Context, all bool) ([]api.Stack, error) {
	list, err := c.List(ctx, all)
	if err != nil {
		return nil, err
	}

	return containersToStacks(list)
}

func containersToStacks(containers []container.Summary) ([]api.Stack, error) {
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

func combinedConfigFiles(containers []container.Summary) (string, error) {
	var configFiles []string

	for _, c := range containers {
		files, ok := c.Labels[api.ConfigFilesLabel]
		if !ok {
			return "", fmt.Errorf("no label %q set on container %q of compose project", api.ConfigFilesLabel, c.ID)
		}

		for _, f := range strings.Split(files, ",") {
			if !composeUtils.StringContains(configFiles, f) {
				configFiles = append(configFiles, f)
			}
		}
	}

	return strings.Join(configFiles, ","), nil
}

func containerToState(containers []container.Summary) []string {
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

func groupContainerByLabel(containers []container.Summary, labelName string) (map[string][]container.Summary, []string, error) {
	containersByLabel := map[string][]container.Summary{}
	var keys []string
	for _, c := range containers {
		label, ok := c.Labels[labelName]
		if !ok {
			return nil, nil, fmt.Errorf("no label %q set on container %q of compose project", labelName, c.ID)
		}
		labelContainers, ok := containersByLabel[label]
		if !ok {
			labelContainers = []container.Summary{}
			keys = append(keys, label)
		}
		labelContainers = append(labelContainers, c)
		containersByLabel[label] = labelContainers
	}
	sort.Strings(keys)
	return containersByLabel, keys, nil
}
