package cmd

import (
	"github.com/arenadata/adcm-installer/compose"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// downCmd represents the down command
var downCmd = &cobra.Command{
	Use:   "down",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		comp, err := compose.NewComposeService()
		if err != nil {
			log.Fatal(err)
		}

		volumes, _ := cmd.Flags().GetBool("volumes")
		if err = comp.Down(cmd.Context(), compose.ProjectName, volumes); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(downCmd)

	downCmd.Flags().Bool("volumes", false, "Remove with volumes")
}
