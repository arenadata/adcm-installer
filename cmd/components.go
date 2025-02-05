package cmd

import (
	"github.com/spf13/cobra"
)

// componentsCmd represents the components command
var componentsCmd = &cobra.Command{
	Aliases: []string{"c", "comp"},
	Use:     "components",
	Short:   "Manage installed components",
}

func init() {
	rootCmd.AddCommand(componentsCmd)
}
