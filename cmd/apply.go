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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/arenadata/adcm-installer/assets"
	"github.com/arenadata/adcm-installer/internal/services"
	"github.com/arenadata/adcm-installer/internal/services/helpers"
	"github.com/arenadata/adcm-installer/pkg/compose"
	"github.com/arenadata/adcm-installer/pkg/secrets"
	"github.com/arenadata/adcm-installer/pkg/types"
	"github.com/arenadata/adcm-installer/pkg/utils"
	"github.com/arenadata/adcm-installer/pkg/vault/unseal"
	"github.com/arenadata/adcm-installer/pkg/vault/unseal/image"

	"github.com/Masterminds/semver/v3"
	composeTypes "github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli/command"
	"github.com/docker/compose/v2/pkg/api"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

var (
	mapFlagsToEnv = map[string]string{
		"db-host":       "DB_HOST",
		"db-port":       "DB_PORT",
		"db-name":       "DB_NAME",
		"db-user":       "DB_USER",
		"db-pass":       "DB_PASS",
		"adpg-password": "POSTGRES_PASSWORD_FILE",
	}

	applyCmd = &cobra.Command{
		Use:   "apply",
		Short: "Apply a configuration by file name",
		Long: `Launches containers on the host specified in the configuration file. Before
launching the master list of containers, directory permissions and database
initialization are pre-configured (when calling adi init with the --adpg
argument). Without arguments, the current directory's adcm.yaml
(adcm.yml/ad-app.yml/ad-app.yaml) and age.key files are searched. If either
file is missing or has an unknown format, the application will exit with an error.
- --age-key takes the value of the private key in clear text. Has priority over
            --age-key-file
- --age-key-file takes the value of the path to the file with the private key
- --dry-run terminates the command without starting containers with the output
            of the configuration for docker compose with encrypted secrets
- --file specifies the path to the configuration file
- --output is used together with the --dry-run flag to specify the path of the
		   file to which the output will be written
- --pg-debug enables the output of debugging information in the container logs,
             excluding the output of sensitive data`,
		Run: applyProject,
	}
)

func init() {
	rootCmd.AddCommand(applyCmd)

	ageKeyFlags(applyCmd, "age-key", ageKeyFileName)
	configFileFlags(applyCmd)

	applyCmd.Flags().Bool("dry-run", false, "Simulate an apply command and generate compose files")
	applyCmd.Flags().Bool("debug", false, "Enable debug in containers")
	applyCmd.Flags().Bool("force", false, "Rewrite unseal data in x-secrets")
	applyCmd.MarkFlagsMutuallyExclusive("dry-run", "debug")
	applyCmd.Flags().StringP("output", "o", "", "Output filename")
}

func applyProject(cmd *cobra.Command, _ []string) {
	logger := log.WithField("command", "apply")

	configFilePath, _ := cmd.Flags().GetString("file")
	prj, err := readConfigFile(configFilePath)
	if err != nil {
		logger.Fatal(err)
	}

	dryRunMode := getBool(cmd, "dry-run")
	debugMode := getBool(cmd, "debug")
	force := getBool(cmd, "force")

	var aes secrets.Secrets
	if !dryRunMode {
		aes, err = encoder(cmd, prj)
		if err != nil {
			logger.Fatal(err)
		}
	}

	xSecrets, unMappedxSecrets, err := secretsDecrypt(prj.Services, aes)
	if err != nil {
		logger.Fatal(err)
	}

	execBuf := new(bytes.Buffer)
	comp, err := compose.NewComposeService(command.WithOutputStream(execBuf))
	if err != nil {
		logger.Fatal(err)
	}

	criInfo, err := comp.Info(cmd.Context())
	if err != nil {
		logger.Fatal(err)
	}

	// https://github.com/moby/moby/blob/v27.5.1/daemon/archive_tarcopyoptions_unix.go#L16
	// in dockerd v28.0.0 the bug with secrets copying has been fixed
	serverVersionString := strings.SplitN(criInfo.ServerVersion, "~astra1", 2)[0]
	var needSecretsFix bool
	serverVersion, err := semver.NewVersion(serverVersionString)
	if err != nil {
		logger.Warnf("Cannot parse dockerd Server Version %s: %s", criInfo.ServerVersion, err)
		needSecretsFix = true
	} else {
		needSecretsFix = serverVersion.LessThan(semver.MustParse("v28.0.0"))
	}

	hostOS := criInfo.OperatingSystem
	servicesModHelpers := helpers.NewModHelpers()
	pgInit := types.NewPGInit()
	_, managedAdpg := prj.Services[services.AdpgName]

	for name, svc := range prj.Services {
		if needSecretsFix {
			svc.User = strings.SplitN(svc.User, ":", 2)[0]
		}

		// rename all svc.Secrets[*].Source
		for i, sec := range svc.Secrets {
			svc.Secrets[i].Source = name + "-" + sec.Source
		}
		prj.Services[name] = svc

		servicesModHelpers = append(servicesModHelpers,
			helpers.Profiles(name, services.PrimaryContainerProfile),
		)

		appType := svc.Labels[compose.ADAppTypeLabelKey]
		if appType != services.AdcmName {
			servicesModHelpers = append(servicesModHelpers,
				helpers.ContainerName(name),
			)
		}

		if appType == services.AdcmName {
			sec := xSecrets[name]

			for k, v := range sec {
				envKey := mapFlagsToEnv[k]
				secret := helpers.Secret{
					Source: name + "-" + k,
					Value:  v,
					EnvKey: envKey,
					Target: path.Join(helpers.SecretsPath, k),
				}
				if len(envKey) > 0 {
					secret.EnvFileKey = envKey + "_FILE"
				}
				servicesModHelpers = append(servicesModHelpers,
					helpers.Secrets(name, secret),
					helpers.ProjectSecrets(secret),
				)
			}

			if managedAdpg {
				servicesModHelpers = append(servicesModHelpers,
					helpers.Environment(name,
						helpers.Env{Name: "DB_HOST", Value: utils.Ptr(services.AdpgName)},
						helpers.Env{Name: "DB_PORT", Value: utils.Ptr("5432")},
					),
				)

				fillPgInitFile(pgInit, sec)
			}

		} else if name == services.AdpgName {
			if svc.ReadOnly {
				mntOpts := mountOpt(hostOS, svc.User)
				mntOpts["size"] = "65536"
				mntOpts["mode"] = "1750"

				servicesModHelpers = append(servicesModHelpers, helpers.MountTmpFs(name,
					helpers.TmpFs{Target: "/var/run/postgresql", MountOptions: mntOpts}))
			}

		} else if name == services.VaultName {
			vaultMode := svc.Labels[compose.ADVaultModeLabelKey]
			if len(vaultMode) > 0 && vaultMode != services.VaultDeployModeDev {
				var target string
				for _, sec := range svc.Secrets {
					if sec.Source == name+"-"+services.ConfigJson {
						target = sec.Target
						break
					}
				}

				servicesModHelpers = append(servicesModHelpers,
					helpers.Entrypoint(name, "bao", "server", "-config="+target))

				if managedAdpg {
					sec := xSecrets[name]
					unMap := unMappedxSecrets[name]

					for k, v := range sec {
						if k == services.ConfigJson {
							var configFile services.VaultConfigFile
							if err = json.Unmarshal([]byte(v), &configFile); err != nil {
								logger.Fatal(err)
							}

							u, err := url.Parse(configFile.Storage.Postgresql.ConnectionUrl)
							if err != nil {
								logger.Fatal(err)
							}

							u.Path = unMap[services.PgDbName]
							u.User = url.UserPassword(unMap[services.PgDbUser], unMap[services.PgDbPass])

							configFile.Storage.Postgresql.ConnectionUrl = u.String()
							b, err := json.Marshal(configFile)
							if err != nil {
								logger.Fatal(err)
							}
							v = string(b)
						}

						s := helpers.Secret{
							Source: name + "-" + k,
							Value:  v,
							Target: path.Join(helpers.SecretsPath, k),
						}

						servicesModHelpers = append(servicesModHelpers,
							helpers.ProjectSecrets(s),
						)
					}

					fillPgInitFile(pgInit, unMap)
				}
			}
		}
	}

	if managedAdpg {
		svc := prj.Services[services.AdpgName]

		// TODO: helper addService to project
		chownName := services.ChownContainer(prj, svc)
		initAdpgServiceName := services.InitContainer(prj, svc)

		// set secrets for init-adpg container
		for k, v := range xSecrets[services.AdpgName] {
			source := services.AdpgName + "-" + k
			s := helpers.Secret{
				Source:     source,
				EnvFileKey: mapFlagsToEnv[source],
				Value:      v,
				Target:     path.Join(helpers.SecretsPath, k),
			}
			servicesModHelpers = append(servicesModHelpers,
				helpers.Secrets(initAdpgServiceName, s),
				helpers.ProjectSecrets(s),
			)
		}

		servicesModHelpers = append(servicesModHelpers,
			helpers.Command(initAdpgServiceName, []string{"initdb"}),
			helpers.Environment(initAdpgServiceName,
				helpers.Env{
					Name:  "PG_ENTRYPOINT_LOG_DEBUG",
					Value: utils.Ptr("true"),
				},
				helpers.Env{
					Name:  "POSTGRES_SHUTDOWN_MODE",
					Value: utils.Ptr("smart"),
				},
			),
			helpers.DependsOn(initAdpgServiceName,
				helpers.Depended{
					Service:   chownName,
					Condition: composeTypes.ServiceConditionCompletedSuccessfully,
					Required:  true,
				}),
		)

		// generate init.json for init-adpg
		if len(pgInit.DB) > 0 || len(pgInit.Role) > 0 {
			initJson, err := json.Marshal(pgInit)
			if err != nil {
				logger.Fatal(err)
			}

			secret := helpers.Secret{
				Source:     services.AdpgName + "-init.json",
				Target:     path.Join(helpers.SecretsPath, "init.json"),
				EnvFileKey: "POSTGRES_INITDB_FILE",
				Value:      string(initJson),
			}
			servicesModHelpers = append(servicesModHelpers,
				helpers.Secrets(initAdpgServiceName, secret),
				helpers.ProjectSecrets(secret),
			)
		}
	}

	for name, svc := range prj.Services {
		servicesModHelpers = append(servicesModHelpers,
			helpers.Platform(name, compose.DefaultPlatform),
			helpers.CustomLabels(name, map[string]string{compose.ADLabel: ""}),
			helpers.SecretsPermission(name, parseUidGidFromUser(svc.User)),
		)
	}

	if err = servicesModHelpers.Apply(prj); err != nil {
		logger.Fatal(err)
	}

	services.PauseContainer(prj)

	if dryRunMode {
		closer, err := setOutput(cmd)
		if err != nil {
			logger.Fatal(err)
		}
		defer func() { _ = closer.Close() }()

		enc := yaml.NewEncoder(cmd.OutOrStdout())
		defer func() { _ = enc.Close() }()

		enc.SetIndent(2)
		_ = enc.Encode(prj)
		_ = enc.Encode(prj.Environment)
		return
	}

	initPrj, err := prj.WithProfiles([]string{services.InitContainerProfile})
	if err != nil {
		logger.Fatal(err)
	}

	defer func() {
		if err != nil {
			logger.Fatal(err)
		}
	}()

	if len(initPrj.Services) > 0 {
		if err := assets.LoadBusyboxImage(cmd.Context()); err != nil {
			logger.Fatal(err)
		}
		if err := comp.Up(cmd.Context(), initPrj, true); err != nil {
			logger.Fatal(err)
		}

		defer func() {
			if !debugMode {
				err := comp.Remove(cmd.Context(), initPrj, initPrj.ServiceNames()...)
				if err != nil {
					log.Warnf("Removing init containers failed: %v", err)
				}
			}
		}()
	}

	eg, _ := errgroup.WithContext(cmd.Context())
	if _, ok := prj.Services[services.VaultName]; ok {
		eg.Go(func() error {
			return vaultInit(cmd.Context(), prj, comp, aes, force)
		})
	}

	containerName := getContainerNameIfItIsRunning(cmd.Context(), comp, prj.Name)
	if force && len(containerName) > 0 {
		time.Sleep(5 * time.Second)
	}

	err = comp.Up(cmd.Context(), prj, true)

	if e := eg.Wait(); e != nil {
		if err == nil {
			err = e
		} else {
			err = fmt.Errorf("%v: %v", err, e)
		}
	}
}

func getContainerNameIfItIsRunning(ctx context.Context, comp *compose.Compose, prjName string) string {
	lst, _ := comp.List(ctx, false)
	for _, l := range lst {
		lbl := l.Labels
		if lbl[api.ProjectLabel] == prjName &&
			lbl[api.ServiceLabel] == services.VaultName &&
			l.State == "running" {
			return strings.Trim(l.Names[0], "/")
		}
	}
	return ""
}

func vaultInit(ctx context.Context, prj *composeTypes.Project, comp *compose.Compose, aes secrets.Secrets, force bool) error {
	output := prj.ComposeFiles[0]
	var adcmYaml map[string]any
	b, err := os.ReadFile(output)
	if err != nil {
		return err
	}
	if err = yaml.Unmarshal(b, &adcmYaml); err != nil {
		return err
	}

	var containerName string
	var count int
	tik := time.NewTicker(2 * time.Second)

OUT:
	for {
		select {
		case <-tik.C:
			if count == 15 {
				tik.Stop()
				return fmt.Errorf("vault init timed out")
			}

			count++
			containerName = getContainerNameIfItIsRunning(ctx, comp, prj.Name)
			if len(containerName) > 0 {
				tik.Stop()
				break OUT
			}
		}
	}

	unsealRunner, err := image.New(containerName)
	if err != nil {
		return err
	}

	status, err := unsealRunner.Status(ctx)
	if err != nil {
		return fmt.Errorf("read vault status failed: %v", err)
	}

	if !status.Sealed {
		return nil
	}

	var unsealDataRaw string
	unMappedData := get(adcmYaml, []string{"services", services.VaultName, "x-secrets", "un-mapped"})
	unsealDataEnc, unsealDataIsExists := unMappedData[services.VaultUnsealData]
	if unsealDataIsExists {
		if aes != nil {
			if unsealDataRaw, err = aes.DecryptValue(unsealDataEnc.(string)); err != nil {
				return fmt.Errorf("decrypt vault init data failed: %v", err)
			}
		} else {
			unsealDataRaw = unsealDataEnc.(string)
		}
	}

	var unsealData *unseal.VaultInitData
	if !status.Initialized {
		if unsealDataIsExists && !force {
			return fmt.Errorf("you are trying unseal Vault/Openbao with uninitialized data. "+
				"Remove the services.%s.x-secrets.un-mapped.%s key mannualy before call apply command. "+
				"Or rerun the command with --force flag, then unseal data will be overwritten",
				services.VaultName, services.VaultUnsealData)
		}

		ud, err := unsealRunner.RawInitData(ctx)
		if err != nil {
			return err
		}
		unsealDataRaw = string(ud)

		if aes != nil {
			if unsealDataEnc, err = aes.EncryptValue(unsealDataRaw); err != nil {
				// this shouldn't happen, but https://go.dev/issue/66821
				return fmt.Errorf("encrypt vault init data failed: %v", err)
			}
		}

		unMappedData[services.VaultUnsealData] = unsealDataEnc

		buf := new(bytes.Buffer)
		enc := yaml.NewEncoder(buf)
		enc.SetIndent(2)

		if err = enc.Encode(adcmYaml); err != nil {
			// this shouldn't happen, but if it does, print the unseal data
			log.Warnf("unseal data: %s", unsealDataEnc)
			return fmt.Errorf("marshal compose file failed: %v", err)
		}

		if err = os.WriteFile(output, buf.Bytes(), 0600); err != nil {
			return fmt.Errorf("write vault init data to adcm.yaml file failed: %v", err)
		}
	}

	if err = json.Unmarshal([]byte(unsealDataRaw), &unsealData); err != nil {
		return fmt.Errorf("unmarshal unseal data failed: %v", err)
	}

	if err = unsealRunner.Unseal(ctx, unsealData.UnsealKeysB64); err != nil {
		return err
	}

	return nil
}

func get(m map[string]any, key []string) map[string]any {
	x := m
	for _, k := range key {
		v := x[k]
		x = v.(map[string]any)
	}
	return x
}

func mountOpt(sys, user string) helpers.Mapping {
	opts := helpers.Mapping{}
	// podman
	if sys == "centos" {
		opts["U"] = ""
		return opts
	}

	if len(user) > 0 {
		usr := parseUidGidFromUser(user)
		opts["uid"] = usr.UID
		if len(usr.GID) > 0 {
			opts["gid"] = usr.GID
		}
	}

	return opts
}

func parseUidGidFromUser(u string) helpers.Secret {
	sec := helpers.Secret{}
	userParts := strings.Split(u, ":")
	sec.UID = userParts[0]
	if len(userParts) > 1 {
		sec.GID = userParts[1]
	}
	return sec
}

func fillPgInitFile(pg *types.PGInit, sec map[string]string) {
	dbUser := sec[services.PgDbUser]
	if len(dbUser) > 0 {
		pg.Role[dbUser] = &types.Role{
			Password: sec[services.PgDbPass],
		}
	}

	dbName := sec[services.PgDbName]
	if len(dbName) > 0 {
		pg.DB[dbName] = nil
		if len(dbUser) > 0 {
			pg.DB[dbName] = &types.Database{Owner: dbUser}
		}
	}
}
