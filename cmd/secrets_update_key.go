package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var updateKeyCmd = &cobra.Command{
	Use:   "update-key",
	Short: "NOT IMPLEMENTED",
	Run:   secretsUpdateKey,
}

func init() {
	secretsCmd.AddCommand(updateKeyCmd)

	ageKeyFlags(updateKeyCmd, "old-age-key", ageKeyFileName, updateKeyCmd.MarkFlagsOneRequired)
	ageKeyFlags(updateKeyCmd, "new-age-key", "", updateKeyCmd.MarkFlagsOneRequired)

	configFileFlags(updateKeyCmd)
}

func secretsUpdateKey(cmd *cobra.Command, _ []string) {
	log.Fatal("not implemented")
}
