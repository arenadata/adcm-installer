package cmd

import (
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Aliases: []string{"ls"},
	Use:     "list",
	Short:   "A brief description of your command",
}

func init() {
	rootCmd.AddCommand(listCmd)
}
