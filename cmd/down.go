package cmd

import (
	"github.com/arenadata/adcm-installer/compose"
	"github.com/arenadata/adcm-installer/models"
	"github.com/arenadata/adcm-installer/utils"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// downCmd represents the down command
var downCmd = &cobra.Command{
	Use:   "down",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, _ []string) {
		logger := log.WithField("command", "down")

		configFile, _ := cmd.Flags().GetString("config")
		isConfigFileExists, err := utils.FileExists(configFile)
		if err != nil {
			logger.Fatal(err)
		}

		var deployId string
		if isConfigFileExists {
			config := make(map[string]any)
			logger.Debugf("Using config file %q", configFile)
			if err = readYamlFile(configFile, config); err != nil {
				logger.Fatal(err)
			}
			deployId = config["deployment-id"].(string)
		}

		if cmd.Flags().Changed("deployment-id") || len(deployId) == 0 {
			deployId, _ = cmd.Flags().GetString("deployment-id")
		}

		volumes, _ := cmd.Flags().GetBool("volumes")
		if volumes {
			logger.Warn("Volumes will be deleted")
		}
		logger.Debugf("DeploymentID %q will be down ...", deployId)

		comp, err := compose.NewComposeService()
		if err != nil {
			logger.Fatal(err)
		}

		if err = comp.Down(cmd.Context(), deployId, volumes); err != nil {
			logger.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(downCmd)

	downCmd.Flags().StringP("config", "c", models.ADCMConfigFile, "Path to configuration file")
	downCmd.Flags().StringP("deployment-id", "d", models.DeploymentId, "DeploymentID name")
	downCmd.MarkFlagsMutuallyExclusive("config", "deployment-id")
	downCmd.Flags().Bool("volumes", false, "Remove with volumes")
}
