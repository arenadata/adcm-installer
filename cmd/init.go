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
	Run: func(cmd *cobra.Command, args []string) {
		configFile, _ := cmd.Flags().GetString("config")

		forceRewrite, _ := cmd.Flags().GetBool("force")
		_, err := os.Stat(configFile)
		if err == nil && !forceRewrite {
			log.Fatalf("config file %s already exists", configFile)
		}

		ageKey, _ := cmd.Flags().GetString("age-key")
		ageKeyFile, _ := cmd.Flags().GetString("age-key-file")
		isAgeKeyFileExists := utils.PathExists(ageKeyFile)
		if len(ageKey) == 0 && len(ageKeyFile) > 0 && isAgeKeyFileExists {
			ageKey, err = readAgeKey(ageKeyFile)
			if err != nil {
				log.Fatalf("read AGE key from %q: %v", ageKeyFile, err)
			}
		}

		ageCrypt, err := crypt.New(ageKey)
		if err != nil {
			log.Fatal(err)
		}

		sec := models.NewSecrets(ageCrypt)
		models.SetDefaultSecrets(sec)

		interactive, _ := cmd.Flags().GetBool("interactive")
		if interactive {
			if err = pgCredentials(sec); err != nil {
				log.Fatal(err)
			}
		}

		var configNode *yaml.Node
		slimConfig, _ := cmd.Flags().GetBool("slim")
		if slimConfig {
			configNode = new(yaml.Node)
			err = configNode.Encode(&models.Config{Secrets: sec})
		} else {
			conf := models.FullConfigWithDefaults()
			conf.Secrets = sec
			configNode, err = models.SetConfigComments(conf)
		}
		if err != nil {
			log.Fatal(err)
		}

		fi, err := os.OpenFile(configFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0640)
		if err != nil {
			log.Fatal(err)
		}
		defer func() {
			if err := fi.Close(); err != nil {
				log.Fatal(err)
			}
		}()

		if !isAgeKeyFileExists {
			if err = saveAgeKey(ageKeyFile, ageCrypt); err != nil {
				log.Fatal(err)
			}
		}

		confEnc := yaml.NewEncoder(fi)
		confEnc.SetIndent(2)
		if err := confEnc.Encode(configNode); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolP("force", "f", false, "force overwrite existing config")
	initCmd.Flags().Bool("slim", false, "create a minimal config file")
	initCmd.Flags().BoolP("interactive", "i", false, "interactive mode (set sensitive data)")
	initCmd.Flags().String("age-key", "", "set specific private age key")
	initCmd.Flags().String("age-key-file", models.AGEKeyFile, "read private age key from file")
	initCmd.MarkFlagsMutuallyExclusive("age-key", "age-key-file")
}

func pgCredentials(sec *models.Secrets) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("Enter PostgreSQL Login (default: %s): ", models.PostgresLogin)
	login, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	fmt.Print("Enter PostgreSQL Password: ")
	bytePassword, err := term.ReadPassword(syscall.Stdin)
	if err != nil {
		return err
	}

	login = strings.TrimSpace(login)
	if len(login) > 0 {
		sec.Postgres.Login = login
	}

	password := strings.TrimSpace(string(bytePassword))
	if len(password) > 0 {
		sec.Postgres.Password = password
	}

	return nil
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
