package cmd

import (
	"fmt"
	"github.com/arenadata/arenadata-installer/apis/app/v1alpha1"
	"github.com/arenadata/arenadata-installer/internal/api/meta"
	"github.com/arenadata/arenadata-installer/internal/runtime"
	"github.com/arenadata/arenadata-installer/pkg/utils"
	"os"
	"path"
	goruntime "runtime"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	appFilename = "ad-app.yaml"
)

var version = "1.0.0-dev"

var rootCmd = &cobra.Command{
	Use:   "arenadata-installer",
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
		CallerPrettyfier: func(f *goruntime.Frame) (string, string) {
			return "", fmt.Sprintf(" %s:%d", path.Base(f.File), f.Line)
		},
	}
	log.SetFormatter(formatter)

	//rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose mode")
	rootCmd.Flags().Bool("version", false, "Print the version and exit")
}

func configFileFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("file", "f", appFilename, "Application configuration file")
}

func getBool(cmd *cobra.Command, key string) bool {
	ok, _ := cmd.Flags().GetBool(key)
	return ok
}

func readConfigFile(configFilePath string) ([]*v1alpha1.Application, error) {
	docs, err := utils.SplitYamlFile(configFilePath)
	if err != nil {
		return nil, err
	}

	var apps []*v1alpha1.Application
	for _, doc := range docs {
		obj, err := runtime.Decode(doc)
		if err != nil {
			return nil, err
		}

		app, ok := obj.(*v1alpha1.Application)
		if !ok {
			return nil, fmt.Errorf("object is not Application type: %s", doc)
		}

		apps = append(apps, app)
	}

	return apps, nil
}

func readConfigMeta(configFilePath string) (meta.ObjectMeta, error) {
	fi, err := os.Open(configFilePath)
	if err != nil {
		return meta.ObjectMeta{}, err
	}
	defer fi.Close()

	var data struct {
		Metadata meta.ObjectMeta `yaml:"metadata"`
	}

	dec := yaml.NewDecoder(fi)
	return data.Metadata, dec.Decode(&data)
}
