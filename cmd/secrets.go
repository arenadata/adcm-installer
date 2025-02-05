package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/arenadata/arenadata-installer/pkg/secrets"
	"github.com/arenadata/arenadata-installer/pkg/utils"

	"github.com/gosimple/slug"
	"github.com/spf13/cobra"
)

const (
	ageKeyFileName = "age.key"
)

var (
	noAgeKeyProvided = errors.New("no age key provided")
)

// secretsCmd represents the secrets command
var secretsCmd = &cobra.Command{
	Aliases: []string{"sec", "secret"},
	Use:     "secrets",
	Short:   "Manage secrets",
}

func init() {
	rootCmd.AddCommand(secretsCmd)
}

func ageEnvKey(key string) string {
	key = slug.Make(key)
	key = strings.ToUpper(key)
	return strings.ReplaceAll(key, "-", "_")
}

func ageKeyFlags(cmd *cobra.Command, key, defaultKeyPath string, markFlags ...func(...string)) {
	if len(key) == 0 {
		panic("age-key must not be empty")
	}

	fileKey := key + "-file"
	envKey := ageEnvKey(key)
	cmd.Flags().String(key, "", "Set private age key. Can be set by "+envKey+" environment variable")
	cmd.Flags().String(fileKey, defaultKeyPath, "Read private age key from file")

	cmd.MarkFlagsMutuallyExclusive(key, fileKey)

	if len(defaultKeyPath) > 0 {
		if isAgeKeyFileExists, err := utils.FileExists(defaultKeyPath); err != nil {
			panic(err)
		} else if isAgeKeyFileExists {
			return
		}
	}

	for _, markFlag := range markFlags {
		markFlag(key, fileKey)
	}
}

func getAgeKey(cmd *cobra.Command, key string) (string, error) {
	ageKey, _ := cmd.Flags().GetString(key)
	if len(ageKey) == 0 {
		ageKey = os.Getenv(ageEnvKey(key))
	}

	fileKey := key + "-file"
	if len(ageKey) == 0 || cmd.Flags().Changed(fileKey) {
		ageKeyFile, _ := cmd.Flags().GetString(fileKey)
		isAgeKeyFileExists, err := utils.FileExists(ageKeyFile)
		if err != nil {
			return "", err
		}
		if isAgeKeyFileExists {
			ageKey, err = readAgeKeyFromFile(ageKeyFile)
			if err != nil {
				return "", fmt.Errorf("read AGE key from file %q failed: %v", ageKeyFile, err)
			}
		}
	}

	if len(ageKey) > 0 {
		return ageKey, nil
	}

	return "", noAgeKeyProvided
}

func readAgeKeyFromFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
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

func readOrCreateNewAgeKey(cmd *cobra.Command, key string) (*secrets.AgeCrypt, bool, error) {
	ageKey, err := getAgeKey(cmd, key)
	if err != nil && !errors.Is(err, noAgeKeyProvided) {
		return nil, false, err
	} else if err == nil {
		cryptKey, err := secrets.NewAgeCryptFromString(ageKey)
		return cryptKey, false, err
	}

	cryptKey, err := secrets.NewAgeCrypt()
	return cryptKey, true, err
}
