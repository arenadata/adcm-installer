package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:     "set <key>",
	Short:   "NOT IMPLEMENTED",
	PreRunE: cobra.ExactArgs(1),
	Run:     secretSetValue,
}

func init() {
	secretsCmd.AddCommand(setCmd)

	ageKeyFlags(setCmd, "age-key", ageKeyFileName)
	configFileFlags(setCmd)
	/*
		--interactive, -i
	*/
}

func secretSetValue(cmd *cobra.Command, args []string) {
	log.Fatal("not implemented")
}
