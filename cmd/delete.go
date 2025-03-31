package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/arenadata/adcm-installer/pkg/compose"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Aliases: []string{"del", "rm"},
	Use:     "delete [name]",
	Short:   "Delete resources by file name or installation name",
	PreRunE: cobra.MaximumNArgs(1),
	Run:     deleteProject,
}

func init() {
	rootCmd.AddCommand(deleteCmd)

	configFileFlags(deleteCmd)
	deleteCmd.Flags().Bool("volumes", false, "Remove all volumes")
	deleteCmd.Flags().Bool("yes", false, "Remove all volumes without asking for confirmation")
}

func deleteProject(cmd *cobra.Command, args []string) {
	logger := log.WithField("command", "delete")

	comp, err := compose.NewComposeService()
	if err != nil {
		logger.Fatal(err)
	}

	deleteVolumes := getBool(cmd, "volumes")
	if deleteVolumes && !getBool(cmd, "yes") {
		fmt.Print("Are you sure you want to delete all volumes: [y/N] ")
		resp, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			logger.Fatal(err)
		}
		if strings.TrimSpace(resp) != "y" {
			fmt.Println("Aborting...")
			return
		}
	}

	var name string
	if len(args) > 0 {
		name = args[0]
	} else {
		configFilePath, _ := cmd.Flags().GetString("file")
		prj, err := readConfigFile(configFilePath)
		if err != nil {
			logger.Fatal(err)
		}
		name = prj.Name
	}

	if err = comp.Down(cmd.Context(), name, deleteVolumes); err != nil {
		logger.Fatal(err)
	}
}
