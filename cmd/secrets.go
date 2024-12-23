package cmd

import "github.com/spf13/cobra"

// secretsCmd represents the secrets command
var secretsCmd = &cobra.Command{
	Aliases: []string{"sec", "s"},
	Use:     "secrets",
	Short:   "A brief description of your command",
}

func init() {
	rootCmd.AddCommand(secretsCmd)
}
