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
	"strconv"
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

	var adcmFamilyServices []string

	for name, svc := range prj.Services {
		if needSecretsFix {
			svc.User = strings.SplitN(svc.User, ":", 2)[0]
		}

		// rename all svc.Secrets[*].Source
		for i, sec := range svc.Secrets {
			svc.Secrets[i].Source = name + "-" + sec.Source
		}

		appType := svc.Labels[compose.ADAppTypeLabelKey]

		if services.AdcmFamilyService(appType) && !dryRunMode {
			adcmFamilyServices = append(adcmFamilyServices, name)
		}

		prj.Services[name] = svc

		servicesModHelpers = append(servicesModHelpers,
			helpers.Profiles(name, services.PrimaryContainerProfile),
		)

		servicesModHelpers = append(servicesModHelpers,
			helpers.ContainerName(name),
		)

		if services.AdcmFamilyService(appType) {
			sec := xSecrets[adcmSecretsSource(name, svc)]

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
		} else if name == services.ConsulName {
			mntOpts := mountOpt(hostOS, svc.User)
			mntOpts["size"] = "1048576"
			mntOpts["mode"] = "1750"

			servicesModHelpers = append(servicesModHelpers,
				helpers.ReadOnlyRootFilesystem(services.ConsulName),
				helpers.MountTmpFs(name, helpers.TmpFs{Target: "/data", MountOptions: mntOpts}),
			)
		}
	}

	if managedAdpg {
		svc := prj.Services[services.AdpgName]

		// TODO: helper addService to project
		chownName := services.ChownContainer(prj, svc, false)
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

	// ADCM 3.0+ migration: chown the ownership of already-existing data
	// volumes/bind mounts to the unprivileged user the container now runs as.
	// The mounts are scanned first so the image is pulled and inspected just once
	migrated := map[string]struct{}{}
	migrations := make(map[string][]composeTypes.ServiceVolumeConfig, len(adcmFamilyServices))
	needUser := false
	for _, name := range adcmFamilyServices {
		svc := prj.Services[name]

		// the secret files are chowned to the same user, so they need it as well
		needUser = needUser || len(svc.Secrets) > 0

		for _, mnt := range svc.Volumes {
			// the ADCM services and their workers share the data volume
			mountKey := string(mnt.Type) + ":" + mnt.Source
			if _, ok := migrated[mountKey]; ok {
				continue
			}
			migrated[mountKey] = struct{}{}

			needsChown, err := mountNeedsChown(cmd.Context(), comp, prj, mnt)
			if err != nil {
				logger.Fatal(err)
			}
			if needsChown {
				migrations[name] = append(migrations[name], mnt)
				needUser = true
			}
		}
	}

	if needUser {
		imageUsers, err := imageUsers(cmd.Context(), comp, prj, adcmFamilyServices, needSecretsFix)
		if err != nil {
			logger.Fatal(err)
		}

		for _, name := range adcmFamilyServices {
			svc := prj.Services[name]
			user := imageUsers[svc.Image]
			if len(user) > 0 {
				svc.User = user
				prj.Services[name] = svc
			}

			if migrate := migrations[name]; len(migrate) > 0 {
				migrateSvc := svc
				migrateSvc.Volumes = migrate
				if len(user) == 0 {
					migrateSvc.User = "0:0"
				}
				services.ChownContainer(prj, migrateSvc, true)
			}
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

	if _, ok := prj.Services[services.VaultName]; ok {
		if err = startManagedVault(cmd.Context(), prj, comp, aes, force); err != nil {
			return
		}
	}

	err = comp.Up(cmd.Context(), prj, true)
}

func startManagedVault(ctx context.Context, prj *composeTypes.Project, comp *compose.Compose, aes secrets.Secrets, force bool) error {
	vaultPrj, err := prj.WithSelectedServices([]string{services.VaultName})
	if err != nil {
		return err
	}

	watch := replaceWatch{upDone: make(chan struct{})}
	if force {
		_, watch.oldContainerID = runningVaultContainer(ctx, comp, prj.Name)
	}

	var rootToken string
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var e error
		rootToken, e = vaultInit(egCtx, prj, comp, aes, force, watch)
		return e
	})

	eg.Go(func() error {
		defer close(watch.upDone)
		return comp.Up(egCtx, vaultPrj, true)
	})

	if err = eg.Wait(); err != nil {
		return err
	}

	return configureAdcmVaultAccess(ctx, prj, comp, rootToken)
}

type replaceWatch struct {
	oldContainerID string
	upDone         chan struct{}
}

func configureAdcmVaultAccess(ctx context.Context, prj *composeTypes.Project, comp *compose.Compose, rootToken string) error {
	adcmVaultServices := adcmServicesWithVaultBackend(prj)
	if len(adcmVaultServices) == 0 {
		return nil
	}

	if len(rootToken) == 0 {
		return fmt.Errorf("no vault root token available to configure ADCM secret storage")
	}

	containerName, _ := runningVaultContainer(ctx, comp, prj.Name)
	if len(containerName) == 0 {
		return fmt.Errorf("vault container is not running")
	}

	runner, err := image.New(containerName)
	if err != nil {
		return err
	}

	var mountPoints []string
	seenMountPoints := map[string]bool{}
	tokenSecrets := helpers.NewModHelpers()
	for svcName, mountPoint := range adcmVaultServices {
		if len(mountPoint) > 0 && !seenMountPoints[mountPoint] {
			seenMountPoints[mountPoint] = true
			mountPoints = append(mountPoints, mountPoint)
		}

		tokenSecrets = append(tokenSecrets, helpers.ProjectSecrets(helpers.Secret{
			Source: svcName + "-" + services.VaultTokenKey,
			Value:  rootToken,
		}))
	}

	if err = runner.EnsureKV2Mounts(ctx, rootToken, mountPoints); err != nil {
		return fmt.Errorf("create ADCM mount points failed: %v", err)
	}

	return tokenSecrets.Apply(prj)
}

func adcmServicesWithVaultBackend(prj *composeTypes.Project) map[string]string {
	out := map[string]string{}
	for name, svc := range prj.Services {
		if !services.AdcmFamilyService(svc.Labels[compose.ADAppTypeLabelKey]) {
			continue
		}

		backend := svc.Environment[services.SecretBackendEnv]
		if backend == nil || *backend != services.SecretBackendEnvVault {
			continue
		}

		var mountPoint string
		if mp := svc.Environment[services.VaultMountPointEnv]; mp != nil {
			mountPoint = *mp
		}
		out[name] = mountPoint
	}
	return out
}

func runningVaultContainer(ctx context.Context, comp *compose.Compose, prjName string) (string, string) {
	lst, _ := comp.List(ctx, false)
	for _, l := range lst {
		lbl := l.Labels
		if lbl[api.ProjectLabel] == prjName &&
			lbl[api.ServiceLabel] == services.VaultName &&
			l.State == "running" {
			return strings.Trim(l.Names[0], "/"), l.ID
		}
	}
	return "", ""
}

type vaultContainerResolver func(context.Context) (name string, id string)

const (
	vaultWaitPolls       = 15
	vaultAvoidPolls      = 5
	vaultInitMaxAttempts = 5
)

var vaultPollInterval = 2 * time.Second

func vaultInit(ctx context.Context, prj *composeTypes.Project, comp *compose.Compose, aes secrets.Secrets, force bool, watch replaceWatch) (string, error) {
	resolve := func(ctx context.Context) (string, string) {
		return runningVaultContainer(ctx, comp, prj.Name)
	}

	vaultSvc := prj.Services[services.VaultName]
	if vaultSvc.Labels[compose.ADVaultModeLabelKey] == services.VaultDeployModeDev {
		if _, _, err := waitVaultContainer(ctx, resolve, "", nil); err != nil {
			return "", err
		}

		if token := vaultSvc.Environment[services.BaoDevRootTokenEnv]; token != nil {
			return *token, nil
		}
		return "", nil
	}

	var lastErr error
	for attempt := 0; attempt < vaultInitMaxAttempts; attempt++ {
		containerName, containerID, err := waitVaultContainer(ctx, resolve, watch.oldContainerID, watch.upDone)
		if err != nil {
			return "", err
		}

		token, err := vaultInitOnce(ctx, prj, aes, force, containerName)
		if err != nil {
			if !image.IsContainerGone(err) {
				return "", err
			}

			lastErr = err
			log.Warnf("vault container %s went away during initialization, retrying: %v", containerName, err)
			continue
		}

		replaced, err := vaultReplaced(ctx, resolve, containerID, watch.upDone)
		if err != nil {
			return "", err
		}
		if replaced {
			log.Warnf("vault container %s was replaced after initialization, repeating on the new container", containerName)
			continue
		}

		return token, nil
	}

	return "", fmt.Errorf("vault init did not survive container replacement: %v", lastErr)
}

func waitVaultContainer(ctx context.Context, resolve vaultContainerResolver, avoidID string, upDone <-chan struct{}) (string, string, error) {
	tik := time.NewTicker(vaultPollInterval)
	defer tik.Stop()

	for count := 0; ; count++ {
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-tik.C:
			if count == vaultWaitPolls {
				return "", "", fmt.Errorf("vault init timed out")
			}

			name, id := resolve(ctx)
			if len(name) == 0 {
				continue
			}

			if len(avoidID) > 0 && id == avoidID && count < vaultAvoidPolls {
				select {
				case <-upDone:
				default:
					continue
				}
			}

			return name, id, nil
		}
	}
}

func vaultReplaced(ctx context.Context, resolve vaultContainerResolver, usedID string, upDone <-chan struct{}) (bool, error) {
	tik := time.NewTicker(vaultPollInterval)
	defer tik.Stop()

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-upDone:
			_, id := resolve(ctx)
			return len(id) > 0 && id != usedID, nil
		case <-tik.C:
			if _, id := resolve(ctx); len(id) > 0 && id != usedID {
				return true, nil
			}
		}
	}
}

func vaultInitOnce(ctx context.Context, prj *composeTypes.Project, aes secrets.Secrets, force bool, containerName string) (string, error) {
	output := prj.ComposeFiles[0]
	var adcmYaml map[string]any
	b, err := os.ReadFile(output)
	if err != nil {
		return "", err
	}
	if err = yaml.Unmarshal(b, &adcmYaml); err != nil {
		return "", err
	}

	unsealRunner, err := image.New(containerName)
	if err != nil {
		return "", err
	}

	status, err := unsealRunner.Status(ctx)
	if err != nil {
		return "", fmt.Errorf("read vault status failed: %v", err)
	}

	var unsealDataRaw string
	unMappedData, err := get(adcmYaml, []string{"services", services.VaultName, "x-secrets", "un-mapped"})
	if err != nil {
		return "", fmt.Errorf("read vault secrets from %s failed: %v", output, err)
	}

	unsealDataEnc, unsealDataIsExists := unMappedData[services.VaultUnsealData]
	if unsealDataIsExists {
		unsealDataString, ok := unsealDataEnc.(string)
		if !ok {
			return "", fmt.Errorf("services.%s.x-secrets.un-mapped.%s must be a string",
				services.VaultName, services.VaultUnsealData)
		}

		unsealDataRaw = unsealDataString
		if aes != nil {
			if unsealDataRaw, err = aes.DecryptValue(unsealDataString); err != nil {
				return "", fmt.Errorf("decrypt vault init data failed: %v", err)
			}
		}
	}

	if !status.Initialized {
		if unsealDataIsExists && !force {
			return "", fmt.Errorf("you are trying unseal Vault/Openbao with uninitialized data. "+
				"Remove the services.%s.x-secrets.un-mapped.%s key mannualy before call apply command. "+
				"Or rerun the command with --force flag, then unseal data will be overwritten",
				services.VaultName, services.VaultUnsealData)
		}

		ud, err := unsealRunner.RawInitData(ctx)
		if err != nil {
			return "", err
		}
		unsealDataRaw = string(ud)

		if aes != nil {
			if unsealDataEnc, err = aes.EncryptValue(unsealDataRaw); err != nil {
				// this shouldn't happen, but https://go.dev/issue/66821
				return "", fmt.Errorf("encrypt vault init data failed: %v", err)
			}
		} else {
			unsealDataEnc = unsealDataRaw
		}

		unMappedData[services.VaultUnsealData] = unsealDataEnc

		buf := new(bytes.Buffer)
		enc := yaml.NewEncoder(buf)
		enc.SetIndent(2)

		if err = enc.Encode(adcmYaml); err != nil {
			// this shouldn't happen, but if it does, print the unseal data
			log.Warnf("unseal data: %s", unsealDataEnc)
			return "", fmt.Errorf("marshal compose file failed: %v", err)
		}

		if err = os.WriteFile(output, buf.Bytes(), 0600); err != nil {
			return "", fmt.Errorf("write vault init data to adcm.yaml file failed: %v", err)
		}
	} else if !unsealDataIsExists {
		if !status.Sealed {
			return "", nil
		}
		return "", fmt.Errorf("vault is sealed and no %s found in services.%s.x-secrets.un-mapped; "+
			"unseal the vault manually", services.VaultUnsealData, services.VaultName)
	}

	var unsealData *unseal.VaultInitData
	if err = json.Unmarshal([]byte(unsealDataRaw), &unsealData); err != nil {
		return "", fmt.Errorf("unmarshal unseal data failed: %v", err)
	}

	if status.Sealed {
		if err = unsealRunner.Unseal(ctx, unsealData.UnsealKeysB64); err != nil {
			return "", err
		}
	}

	return unsealData.RootToken, nil
}

func get(m map[string]any, key []string) (map[string]any, error) {
	x := m
	for i, k := range key {
		v, ok := x[k].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%q key not found or has unexpected format", strings.Join(key[:i+1], "."))
		}
		x = v
	}
	return x, nil
}

// service mount's ownership must be fixed for the unprivileged user.
//   - Bind mounts always do: a pre-existing host directory carries data written
//     by the old root-based ADCM, and a fresh one is created root-owned by engine
//   - Named volumes only when they already exist
func mountNeedsChown(ctx context.Context, comp *compose.Compose, prj *composeTypes.Project, mnt composeTypes.ServiceVolumeConfig) (bool, error) {
	switch mnt.Type {
	case composeTypes.VolumeTypeBind:
		return true, nil
	case composeTypes.VolumeTypeVolume:
		name := mnt.Source
		if v, ok := prj.Volumes[mnt.Source]; ok && len(v.Name) > 0 {
			name = v.Name
		}
		return comp.VolumeExists(ctx, name)
	default:
		return false, nil
	}
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
		if len(usr.UID) > 0 {
			opts["uid"] = usr.UID
		}
		if len(usr.GID) > 0 {
			opts["gid"] = usr.GID
		}
	}

	return opts
}

// imageUsers pulls the images of the given services, when the daemon does
// not have them, and reports the user each image runs as.
//
// uidOnly cuts the user down to its uid, for dockerd below v28.0.0, which
// mis-copies secrets when the user is uid:gid.
func imageUsers(ctx context.Context, comp *compose.Compose, prj *composeTypes.Project,
	svcNames []string, uidOnly bool) (map[string]string, error) {
	users := map[string]string{}
	for _, name := range svcNames {
		image := prj.Services[name].Image
		if _, ok := users[image]; ok {
			continue
		}

		if err := comp.Pull(ctx, prj, compose.DefaultPlatform, name); err != nil {
			return nil, err
		}

		user, err := comp.ImageUser(ctx, image)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve the user of image %q: %v", image, err)
		}

		if uidOnly {
			user = strings.SplitN(user, ":", 2)[0]
		}

		users[image] = user
	}

	return users, nil
}

// adcmSecretsSource names the service whose x-secrets hold the credentials svc
// runs with. The label names the ADCM instance that owns them -- carried by the
// workers and by every ADCM instance but the primary, all of which share one
// database and secret storage.
func adcmSecretsSource(name string, svc composeTypes.ServiceConfig) string {
	if adcmName := svc.Labels[compose.ADAppAdcmLabelKey]; len(adcmName) > 0 {
		return adcmName
	}

	return name
}

func parseUidGidFromUser(u string) helpers.Secret {
	sec := helpers.Secret{}
	userParts := strings.SplitN(u, ":", 2)
	if !numericID(userParts[0]) {
		return sec
	}

	sec.UID = userParts[0]
	if len(userParts) > 1 && numericID(userParts[1]) {
		sec.GID = userParts[1]
	}
	return sec
}

func numericID(s string) bool {
	if len(s) == 0 {
		return false
	}

	_, err := strconv.Atoi(s)
	return err == nil
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
