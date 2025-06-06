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

package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/arenadata/adcm-installer/pkg/compose"

	"github.com/docker/compose/v2/cmd/formatter"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type stackView struct {
	Name        string
	Status      string
	ConfigFiles string
}

var listNamespacesCmd = &cobra.Command{
	Use:   "list",
	Short: "List running ADCM installation",
	Long: `Displays a list of running ADCM installations on the current host
- --all includes stopped ADCM installations in the output`,
	Run: listNamespaces,
}

func init() {
	rootCmd.AddCommand(listNamespacesCmd)

	listNamespacesCmd.Flags().BoolP("all", "a", false, "Show all")
}

func listNamespaces(cmd *cobra.Command, _ []string) {
	logger := log.WithField("command", "list")

	comp, err := compose.NewComposeService()
	if err != nil {
		logger.Fatal(err)
	}

	all, _ := cmd.Flags().GetBool("all")
	stacks, err := comp.List(cmd.Context(), all)
	if err != nil {
		logger.Fatal(err)
	}

	view := make([]stackView, len(stacks))
	for i, s := range stacks {
		configFile := s.ConfigFiles
		if len(configFile) == 0 {
			configFile = "N/A"
		}
		view[i] = stackView{
			Name:        s.Name,
			Status:      strings.TrimSpace(fmt.Sprintf("%s %s", s.Status, s.Reason)),
			ConfigFiles: configFile,
		}
	}

	err = formatter.Print(view, formatter.TABLE, cmd.OutOrStdout(), func(w io.Writer) {
		for _, stack := range view {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", stack.Name, stack.Status, stack.ConfigFiles)
		}
	}, "NAMES", "STATUS", "CONFIG FILES")
}
