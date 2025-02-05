package cmd

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/arenadata/arenadata-installer/pkg/secrets"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var newKeyCmd = &cobra.Command{
	Use:   "new-key",
	Short: "Generate a new private key",
	Run:   secretsNewKey,
}

func init() {
	secretsCmd.AddCommand(newKeyCmd)

	newKeyCmd.Flags().StringP("output", "o", ageKeyFileName, "Key output filename")
}

func secretsNewKey(cmd *cobra.Command, _ []string) {
	logger := log.WithField("command", "secrets-new-key")

	age, err := secrets.NewAgeCrypt()
	if err != nil {
		logger.Fatal(err)
	}

	outputPath, _ := cmd.Flags().GetString("output")
	if len(outputPath) > 0 && outputPath != "-" {
		if err = saveAgeKey(outputPath, age); err != nil {
			logger.Fatal(err)
		}
	}

	_ = fPrintAgeKey(os.Stdout, os.Stderr, age)
}

func saveAgeKey(path string, key *secrets.AgeCrypt) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("file %s already exists", path)
	}
	fi, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0400)
	if err != nil {
		return err
	}
	defer func() {
		if e := fi.Close(); e != nil {
			err = e
		}
	}()

	err = fPrintAgeKey(fi, fi, key)

	return err
}

func fPrintAgeKey(stdout, stderr io.Writer, key *secrets.AgeCrypt) error {
	if _, err := fmt.Fprintf(stderr, "# created: %s\n", time.Now().Format(time.RFC3339)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stderr, "# public key: %s\n", key.Recipient()); err != nil {
		return err
	}

	_, err := fmt.Fprintf(stdout, "%s\n", key)
	return err
}
