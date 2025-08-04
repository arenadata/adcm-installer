/*
 Copyright (c) 2025 Arenadata Softwer LLC.
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package cmd

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/arenadata/adcm-installer/internal/services"
	"github.com/arenadata/adcm-installer/internal/services/helpers"
	"github.com/arenadata/adcm-installer/pkg/compose"
	"github.com/arenadata/adcm-installer/pkg/secrets"
	"github.com/arenadata/adcm-installer/pkg/utils"

	"github.com/compose-spec/compose-go/v2/cli"
	composeTypes "github.com/compose-spec/compose-go/v2/types"
	dockerCompose "github.com/docker/compose/v2/cmd/compose"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	initCmd = &cobra.Command{
		Use:   "init <name>",
		Short: "Initialize a new configuration",
		Long: `Creates a configuration file with the specified name in docker compose format
with a list of preconfigured services. The private key is used to
encrypt/decrypt secrets in the configuration file, which is looked for in the
current directory in a file named age.key. If the age.key file does not exist,
it will be created automatically. When calling the command without specifying
additional parameters, you will be prompted for parameters for connecting to
the database in interactive mode. Attempting to call the command with an 
existing configuration file, an invalid age.key file format, or without
specifying an installation name will result in the program exiting with an
error. The installation name must be unique within a single server.
- --adpg adds the PostgreSQL service to the configuration file and configures
         ADCM to use it. Interactive mode is not used without specifying
         additional parameters
- --age-key takes the value of the private key in cleartext. Takes precedence
            over --age-key-file
- --age-key-file takes the path to the file with the private key
- --force allows you to overwrite the existing configuration file
- --from-config path to a file in yaml format filled with variables for
                fine-tuning the configuration without using interactive mode
- --interactive fine-tuning each service in interactive mode`,
		PreRunE: cobra.ExactArgs(1),
		Run:     initProject,
	}

	fileNames = []string{"adcm.yaml", "adcm.yml", "ad-app.yaml", "ad-app.yml"}
)

func init() {
	rootCmd.AddCommand(initCmd)

	ageKeyFlags(initCmd, "age-key", ageKeyFileName)
	initCmdFlags(initCmd)
}

func initCmdFlags(cmd *cobra.Command) {
	f := cmd.Flags()

	f.Bool("no-crypt", false, "Don't encrypt data")
	_ = f.MarkHidden("no-crypt")

	f.Uint8("adcm-count", 1, "Set number of ADCM instances")
	f.Bool(services.AdpgName, false, "Use managed ADPG")
	f.Bool(services.ConsulName, false, "Use managed Consul (Alpha)")
	f.Bool(services.VaultName, false, "Use managed Vault")
	f.Bool("force", false, "Force overwrite existing config file")
	f.BoolP("interactive", "i", false, "Interactive mode")

	f.StringP("output", "o", "", "Output filename")
	f.String("from-config", "", "Read variables from config file")
	cmd.MarkFlagsMutuallyExclusive("adcm-count", "from-config", "interactive")
}

func initProject(cmd *cobra.Command, args []string) {
	logger := log.WithField("command", "init")

	if err := isConfigExists(cmd); err != nil {
		logger.Fatal(err)
	}

	opts := []services.ProjectOption{
		services.WithInteractive(getBool(cmd, "interactive")),
		services.WithAdpg(getBool(cmd, services.AdpgName)),
		services.WithConsul(getBool(cmd, services.ConsulName)),
		services.WithVault(getBool(cmd, services.VaultName)),
	}

	configFile, _ := cmd.Flags().GetString("from-config")
	adcmCount, _ := cmd.Flags().GetUint8("adcm-count")

	opts = append(opts,
		services.WithConfigFile(configFile),
		services.WithAdcmCount(adcmCount),
	)

	var isNewAgeKey bool
	var age *secrets.AgeCrypt
	var masterKey *services.XSecrets
	if !getBool(cmd, "no-crypt") {
		key := make([]byte, 32)
		_, err := rand.Read(key)
		if err != nil {
			logger.Fatal(err)
		}

		age, isNewAgeKey, err = readOrCreateNewAgeKey(cmd, "age-key")
		if err != nil {
			logger.Fatal(err)
		}

		mKey, err := age.Encrypt(string(key))
		if err != nil {
			logger.Fatal(err)
		}

		masterKey = &services.XSecrets{
			AgeRecipient: age.Recipient().String(),
			Key:          mKey,
		}

		aes, err := secrets.NewAesCrypt(key)
		if err != nil {
			logger.Fatal(err)
		}

		opts = append(opts, services.WithCrypt(aes))
	}

	prj, err := services.New(args[0], opts...)
	if err != nil {
		logger.Fatal(err)
	}

	if masterKey != nil {
		prj.AppendHelpers(helpers.Extension("", services.XSecretsKey, masterKey))
	}
	prj.AppendHelpers(helpers.ProjectNetwork(compose.DefaultNetwork, nil))

	if err = prj.Build(); err != nil {
		logger.Fatalf("Build project failed: %v", err)
	}

	uid := 10001
	for _, svc := range prj.Services() {
		if svc.Type == services.AdcmName {
			continue
		}

		//// FIXME: can't create secret in read-only containers
		//prj.AppendHelpers(helpers.ReadOnlyRootFilesystem(svc.Name))

		uids := strconv.Itoa(uid)
		uid++
		prj.AppendHelpers(
			helpers.SecurityOptsNoNewPrivileges(svc.Name),
			helpers.User(svc.Name, uids, uids),
		)
	}

	if err = prj.ApplyHelpers(); err != nil {
		logger.Fatal(err)
	}

	closer, err := setOutput(cmd)
	if err != nil {
		logger.Fatalf("Could not set output: %s", err)
	}
	defer func() {
		if e := closer.Close(); e != nil {
			logger.Fatal(e)
		}
		if err != nil {
			logger.Fatal(err)
		}
	}()

	if isNewAgeKey {
		if err = saveAgeKey(ageKeyFileName, age); err != nil {
			logger.Fatal(err)
		}
	}

	out := cmd.OutOrStdout()
	err = prj.ToYaml(out)
}

func isConfigExists(cmd *cobra.Command) error {
	outputPath, _ := cmd.Flags().GetString("output")
	if len(outputPath) == 0 {
		absPath, err := filepath.Abs(outputPath)
		if err != nil {
			return fmt.Errorf("could not determine absolute path: %s", err)
		}

		workingDir := filepath.Dir(absPath)
		fi, err := os.Stat(absPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("could not stat path: %s", err)
			}
		} else if fi.IsDir() {
			workingDir = absPath
		}

		outputPath = filepath.Join(absPath, fileNames[0])
		if configPaths := findFiles(fileNames, workingDir); len(configPaths) > 0 {
			outputPath = configPaths[0]
		}
	} else {
		var err error
		outputPath, err = filepath.Abs(outputPath)
		if err != nil {
			return fmt.Errorf("could not determine absolute path: %s", err)
		}
	}
	_ = cmd.Flags().Set("output", outputPath)

	force, _ := cmd.Flags().GetBool("force")
	if ok, err := utils.FileExists(outputPath); err != nil {
		return err
	} else if ok && !force {
		return fmt.Errorf("config file %q already exists", outputPath)
	}
	return nil
}

func findFiles(names []string, pwd string) []string {
	var candidates []string
	for _, n := range names {
		f := filepath.Join(pwd, n)
		if _, err := os.Stat(f); err == nil {
			candidates = append(candidates, f)
		}
	}
	return candidates
}

func readConfigFile(conf string) (*composeTypes.Project, error) {
	cli.DefaultFileNames = fileNames

	opts := dockerCompose.ProjectOptions{
		Offline: true,
	}

	var sec *services.XSecrets
	projectOpts := []cli.ProjectOptionsFn{
		cli.WithConsistency(false),
		cli.WithExtension(services.XSecretsKey, sec),
	}

	if len(conf) > 0 {
		opts.ConfigPaths = []string{conf}
	} else {
		projectOpts = append(projectOpts, cli.WithDefaultConfigPath)
	}

	prj, _, err := opts.ToProject(context.Background(), nil, nil, projectOpts...)

	return prj, err
}
