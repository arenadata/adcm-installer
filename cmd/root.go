package cmd

import (
	"fmt"
	"os"
	"path"
	"runtime"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	version = "1.0.0-dev"
	RootCmd = rootCmd
)

var rootCmd = &cobra.Command{
	Use:   "adcm-installer",
	Short: "Command line tool for installing Arenadata products",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getBool(cmd, "version") {
			cmd.Println(version)
			return nil
		}
		return cmd.Usage()
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	var verbose bool
	cobra.OnInitialize(func() {
		if verbose {
			log.SetLevel(log.DebugLevel)
		}
	})

	log.SetReportCaller(true)
	formatter := &log.TextFormatter{
		TimestampFormat:        "20060102150405",
		FullTimestamp:          true,
		DisableLevelTruncation: true,
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			return "", fmt.Sprintf(" %s:%d", path.Base(f.File), f.Line)
		},
	}
	log.SetFormatter(formatter)

	rootCmd.Flags().Bool("version", false, "Print the version and exit")
}

func configFileFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("file", "f", "", "Application configuration file")
}

func getBool(cmd *cobra.Command, key string) bool {
	ok, _ := cmd.Flags().GetBool(key)
	return ok
}
