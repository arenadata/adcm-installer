package cmd

import (
	"os"
	"path/filepath"

	"github.com/arenadata/adcm-installer/compose"
	"github.com/arenadata/adcm-installer/crypt"
	"github.com/arenadata/adcm-installer/models"
	"github.com/arenadata/adcm-installer/utils"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// upCmd represents the install command
var upCmd = &cobra.Command{
	Use:   "up",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, _ []string) {
		logger := log.WithField("command", "up")

		if initConfig, _ := cmd.Flags().GetBool("init"); initConfig {
			initCmd.Run(cmd, nil)
		}

		configFile, _ := cmd.Flags().GetString("config")
		isConfigFileExists, err := utils.FileExists(configFile)
		if err != nil {
			logger.Fatal(err)
		}

		logger.Debug("GetProject AGE key")
		ageCrypt, err := getAgeKey(cmd, logger)
		if err != nil {
			logger.Fatal(err)
		}
		if ageCrypt == nil {
			logger.Debug("Create new AGE key")
			ageCrypt, err = crypt.New()
			if err != nil {
				logger.Fatal(err)
			}
		}

		config := &models.Config{
			Secrets: models.NewSecrets(ageCrypt),
		}

		var configFilePath string
		if isConfigFileExists {
			logger.Debugf("Using config file %q", configFile)
			if err = readYamlFile(configFile, config); err != nil {
				logger.Fatal(err)
			}
			configFilePath, err = filepath.Abs(configFile)
			if err != nil {
				logger.Warningf("get absolute path to config failed: %v", err)
			}
		}

		models.SetDefaultSecrets(config.Secrets.SensitiveData)
		models.SetDefaultsConfig(config)

		logger.Debug("Initialize ADCM project from config")
		prj, err := compose.NewADCMProject(config, configFilePath)
		if err != nil {
			logger.Fatal(err)
		}

		comp, err := compose.NewComposeService()
		if err != nil {
			logger.Fatal(err)
		}

		if err = comp.Up(cmd.Context(), prj); err != nil {
			logger.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(upCmd)
	upCmd.Flags().StringP("config", "c", models.ADCMConfigFile, "Path to configuration file")
	upCmd.Flags().String("age-key", "", "Set specific private age key. Can be set by AGE_KEY environment variable")
	upCmd.Flags().String("age-key-file", models.AGEKeyFile, "Private AGE key file")
	upCmd.Flags().Bool("init", false, "Initialize new project")
	upCmd.MarkFlagsMutuallyExclusive("age-key", "age-key-file")

	// TODO: implement
	upCmd.Flags().String("with-backup", "", "Run ADCM from backup data")
	upCmd.MarkFlagsMutuallyExclusive("init", "with-backup")
	upCmd.MarkFlagsMutuallyExclusive("config", "with-backup")
}

func readYamlFile(file string, out any) error {
	fi, err := os.Open(file)
	if err != nil {
		return err
	}
	defer func() { _ = fi.Close() }()

	dec := yaml.NewDecoder(fi)
	dec.KnownFields(true)
	return dec.Decode(out)
}
