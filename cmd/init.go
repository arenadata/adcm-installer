package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/arenadata/adcm-installer/crypt"
	"github.com/arenadata/adcm-installer/models"
	"github.com/arenadata/adcm-installer/utils"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "A brief description of your command",
	Run:   initProject,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringP("config", "c", models.ADCMConfigFile, "Path to save configuration file")
	initCmd.Flags().BoolP("force", "f", false, "Force overwrite existing config")
	initCmd.Flags().BoolP("interactive", "i", false, "Interactive mode (set sensitive data)")
	initCmd.Flags().String("age-key", "", "Set specific private age key. Can be set by AGE_KEY environment variable")
	initCmd.Flags().String("age-key-file", models.AGEKeyFile, "Read private age key from file")
	initCmd.MarkFlagsMutuallyExclusive("age-key", "age-key-file")
}

func initProject(cmd *cobra.Command, _ []string) {
	logger := log.WithField("command", "init")

	configFile, _ := cmd.Flags().GetString("config")
	isConfigFileExists, err := utils.FileExists(configFile)
	if err != nil {
		logger.Fatal(err)
	}

	forceRewrite, _ := cmd.Flags().GetBool("force")
	if isConfigFileExists && !forceRewrite {
		logger.Fatalf("config file %s already exists", configFile)
	} else if isConfigFileExists {
		logger.Warnf("config file %s will be rewriten", configFile)
	}

	logger.Debug("Get AGE key")
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

		ageKeyFile, _ := cmd.Flags().GetString("age-key-file")
		logger.Debugf("Create new AGE key file: %q", ageKeyFile)
		if err = saveAgeKey(ageKeyFile, ageCrypt); err != nil {
			logger.Fatal(err)
		}
	}

	sec := models.NewSecrets(ageCrypt)
	models.SetDefaultSecrets(sec.SensitiveData)

	interactive, _ := cmd.Flags().GetBool("interactive")
	if interactive {
		logger.Debug("Interactive mode enabled")
		if err = pgCredentials(sec); err != nil {
			logger.Fatal(err)
		}
	}

	conf := models.FullConfigWithDefaults()
	conf.Secrets = sec

	logger.Debug("Set comments to config")
	configNode, err := models.SetConfigComments(conf)
	if err != nil {
		logger.Fatal(err)
	}

	fi, err := os.OpenFile(configFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0640)
	if err != nil {
		logger.Fatal(err)
	}
	defer func() {
		if err = fi.Close(); err != nil {
			logger.Fatal(err)
		}
	}()

	logger.Debugf("Write config file %q", configFile)
	confEnc := yaml.NewEncoder(fi)
	confEnc.SetIndent(2)
	if err = confEnc.Encode(configNode); err != nil {
		logger.Fatal(err)
	}
}

func pgCredentials(sec *models.Secrets) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("Enter PostgreSQL Login (default: %s): ", models.PostgresLogin)
	login, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	fmt.Print("Enter PostgreSQL Password (default: random generated): ")
	bytePassword, err := term.ReadPassword(syscall.Stdin)
	if err != nil {
		return err
	}

	login = strings.TrimSpace(login)
	if len(login) > 0 {
		sec.SensitiveData.Postgres.Login = login
	}

	password := strings.TrimSpace(string(bytePassword))
	if len(password) > 0 {
		sec.SensitiveData.Postgres.Password = password
	}

	return nil
}

func getAgeKey(cmd *cobra.Command, logger *log.Entry) (*crypt.AgeCrypt, error) {
	ageKey, _ := cmd.Flags().GetString("age-key")
	if len(ageKey) == 0 {
		ageKey = os.Getenv("AGE_KEY")
	}
	if len(ageKey) > 0 {
		log.Debug("AGE key provided")
		ageCrypt, err := crypt.FromString(ageKey)
		return ageCrypt, err
	}

	ageKeyFile, _ := cmd.Flags().GetString("age-key-file")
	isAgeKeyFileExists, err := utils.FileExists(ageKeyFile)
	if err != nil {
		return nil, err
	}
	if isAgeKeyFileExists {
		logger.Debugf("Using AGE key file %q", ageKeyFile)
		ageKey, err = readAgeKey(ageKeyFile)
		if err != nil {
			return nil, fmt.Errorf("read AGE key from file %q failed: %v", ageKeyFile, err)
		}

		ageCrypt, err := crypt.FromString(ageKey)
		return ageCrypt, err
	}

	return nil, nil
}

func readAgeKey(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		return strings.TrimSpace(line), nil
	}

	return "", fmt.Errorf("no age key found")
}
