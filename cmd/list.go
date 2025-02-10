package cmd

import (
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Aliases: []string{"ls"},
	Use:     "list",
	Short:   "Lists resources",
}

func init() {
	rootCmd.AddCommand(listCmd)
}
