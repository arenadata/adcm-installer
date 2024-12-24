package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "github.com/arenadata/adcm-installer",
	Short: "A brief description of your application",
}

func init() {
	var verbose bool
	cobra.OnInitialize(func() {
		if verbose {
			log.SetLevel(log.DebugLevel)
		}
	})
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose mode")
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
