package cmd

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/arenadata/adcm-installer/crypt"
	"github.com/arenadata/adcm-installer/models"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// newKeyCmd represents the newKey command
var newKeyCmd = &cobra.Command{
	Use:   "new-key",
	Short: "Generate new AGE key",
	Run: func(cmd *cobra.Command, args []string) {
		k, err := crypt.New()
		if err != nil {
			log.Fatalf("generate new age key failed: %v", err)
		}

		stdout, stderr := cmd.OutOrStdout(), cmd.ErrOrStderr()
		output, _ := cmd.Flags().GetString("output")
		if len(output) > 0 && output != "-" {
			if err = saveAgeKey(output, k); err != nil {
				log.Fatalf("save age key failed: %v", err)
			}

			return
		}

		if err := printAgeKey(stderr, stdout, k); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	secretsCmd.AddCommand(newKeyCmd)
	newKeyCmd.Flags().StringP("output", "o", models.AGEKeyFile, "Output filename")
}

func saveAgeKey(path string, key *crypt.AgeCrypt) error {
	fi, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0640)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if e := fi.Close(); e != nil {
			err = e
		}
	}()

	if err = printAgeKey(fi, fi, key); err != nil {
		return err
	}

	return err
}

func printAgeKey(stdout, stderr io.Writer, key *crypt.AgeCrypt) error {
	if _, err := fmt.Fprintf(stderr, "# created: %s\n", time.Now().Format(time.RFC3339)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stderr, "# public key: %s\n", key.Recipient()); err != nil {
		return err
	}

	_, err := fmt.Fprintf(stdout, "%s\n", key)
	return err
}
