package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/arenadata/arenadata-installer/pkg/compose"

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
	Aliases: []string{"ns"},
	Use:     "namespaces",
	Short:   "List running namespaces",
	Run:     listNamespaces,
}

func init() {
	listCmd.AddCommand(listNamespacesCmd)

	listNamespacesCmd.Flags().BoolP("all", "a", false, "Show all namespaces")
}

func listNamespaces(cmd *cobra.Command, _ []string) {
	logger := log.WithField("command", "list-namespaces")

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
	}, "NAMESPACE", "STATUS", "CONFIG FILES")
}
