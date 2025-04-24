package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/arenadata/adcm-installer/assets"
	"github.com/arenadata/adcm-installer/pkg/compose"
	"github.com/arenadata/adcm-installer/pkg/secrets"
	"github.com/arenadata/adcm-installer/pkg/types"
	"github.com/arenadata/adcm-installer/pkg/utils"

	"github.com/compose-spec/compose-go/v2/cli"
	composeTypes "github.com/compose-spec/compose-go/v2/types"
	dockerCompose "github.com/docker/compose/v2/cmd/compose"
	"github.com/docker/compose/v2/pkg/api"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	PathSeparator         = "ARENADATA_APP_PATH_SEPARATOR"
	FilePath              = "ARENADATA_APP_FILE"
	DisableDefaultEnvFile = "ARENADATA_APP_DISABLE_ENV_FILE"

	xsecretsKey = "x-secrets"
)

var (
	fileNames = []string{"adcm.yaml", "adcm.yml", "ad-app.yaml", "ad-app.yml"}

	applyCmd = &cobra.Command{
		Use:   "apply",
		Short: "Apply a configuration by file name",
		Run:   applyProject,
	}
)

func init() {
	rootCmd.AddCommand(applyCmd)

	ageKeyFlags(applyCmd, "age-key", ageKeyFileName)
	configFileFlags(applyCmd)

	applyCmd.Flags().Bool("dry-run", false, "Simulate an apply command and generate docker-compose.yaml without secrets")
	applyCmd.Flags().Bool("pg-debug", false, "Enable debugging adpg-init container")
	applyCmd.MarkFlagsMutuallyExclusive("dry-run", "pg-debug")
	applyCmd.Flags().StringP("output", "o", "", "Output filename")
}

func applyProject(cmd *cobra.Command, _ []string) {
	logger := log.WithField("command", "apply")

	configFilePath, _ := cmd.Flags().GetString("file")
	prj, err := readConfigFile(configFilePath)
	if err != nil {
		logger.Fatal(err)
	}
	fillADCMLabels(prj)

	dryRunMode := getBool(cmd, "dry-run")

	ageKey, err := getAgeKey(cmd, "age-key")
	if err != nil {
		logger.Fatal(err)
	}

	dec, err := secrets.NewAgeCryptFromString(ageKey)
	if err != nil {
		logger.Fatal(err)
	}

	if xSecrets, ok := prj.Extensions[xsecretsKey].(*xsecrets); ok {
		if xSecrets.AgeRecipient != dec.Recipient().String() {
			logger.Fatal("age_recipient not match")
		}

		for k, v := range xSecrets.Data {
			if !dryRunMode {
				v, err = dec.DecryptValue(v)
				if err != nil {
					logger.Fatal(err)
				}
			}

			prj.Environment[k] = v
		}
	}

	var hasManagedServices bool
	var adpgServiceName string
	for svcName, svc := range prj.Services {
		if svc.Labels[compose.ADAppTypeLabelKey] == "adpg" {
			hasManagedServices = true
			adpgServiceName = svcName
		}
	}

	comp, err := compose.NewComposeService()
	if err != nil {
		logger.Fatal(err)
	}

	criInfo, err := comp.Info(cmd.Context())
	if err != nil {
		logger.Fatal(err)
	}

	helpers := compose.NewModHelpers()
	for svcName, svc := range prj.Services {
		svcType := svc.Labels[compose.ADAppTypeLabelKey]
		if svcType == "adpg" && svc.ReadOnly {
			mountOpt := "U"
			if criInfo.OperatingSystem != "centos" {
				uid, gid := "0", "0"
				if len(svc.User) > 0 {
					uidGid := strings.Split(svc.User, ":")
					uid = uidGid[0]
					if len(uidGid) > 1 {
						gid = uidGid[1]
					}
				}

				mountOpt = fmt.Sprintf("uid=%s,gid=%s", uid, gid)
			}

			mnt := fmt.Sprintf("/var/run/postgresql:%s", mountOpt)
			helpers = append(helpers, compose.TmpFs(svcName, mnt))
		}

		var sslOpts *types.DbSSLOptions
		if svcType == "adcm" {
			if len(adpgServiceName) > 0 {
				helpers = append(helpers,
					compose.Environment(svcName,
						compose.Env{Name: "DB_HOST", Value: &adpgServiceName},
						compose.Env{Name: "DB_PORT", Value: utils.Ptr("5432")},
					))
			}

			if optStr, ok := svc.Environment[pgSslOptEnvKey]; ok {
				if err = json.Unmarshal([]byte(*optStr), &sslOpts); err != nil {
					logger.Fatal(err)
				}
			}
		}

		var serviceSecrets []compose.Secret
		for _, sec := range svc.Secrets {
			envKey, ok := mapFlagsToEnv[sec.Source]
			if !ok {
				// TODO
			}

			composeSec := compose.Secret{
				Source: sec.Source,
				EnvKey: envKey,
				Value:  prj.Environment[sec.Source],
				ENV:    len(envKey) > 0,
			}

			if sslOpts != nil {
				if sec.Source == pgSslCaKey {
					sslOpts.SSLRootCert = sec.Target
				} else if sec.Source == pgSslCertKey {
					sslOpts.SSLCert = sec.Target
				} else if sec.Source == pgSslKeyKey {
					var mode uint32 = 0o400
					composeSec.FileMode = &mode
					sslOpts.SSLKey = sec.Target
				}
			}

			serviceSecrets = append(serviceSecrets, composeSec)
		}

		if sslOpts != nil {
			optStr := sslOpts.String()
			helpers = append(helpers,
				compose.Environment(svcName, compose.Env{Name: pgSslOptEnvKey, Value: &optStr}),
			)
		}

		helpers = append(helpers,
			compose.Secrets(svcName, serviceSecrets...),
			compose.Platform(svcName, compose.DefaultPlatform),
		)
	}

	if err = helpers.Run(prj); err != nil {
		logger.Fatal(err)
	}

	pgDebug := getBool(cmd, "pg-debug")
	var projectInit *composeTypes.Project
	if hasManagedServices {
		projectInit, err = newInitProject(prj, pgDebug)
		if err != nil {
			logger.Fatal(err)
		}
		fillADCMLabels(projectInit)
	}

	// compose.Up(projectInit)
	// chown/init
	// init vault (vault operator init -key-shares=1 -key-threshold=1 -format=json > keys.json)
	// write keys.json to x-secrets
	// wait for all containers stopped
	// compose.Down(projectInit) w/o delete volumes
	// compose.Up(prj)

	if err = assets.LoadBusybox(cmd.Context()); err != nil {
		logger.Fatal(err)
	}

	if dryRunMode {
		closer, err := setOutput(cmd)
		if err != nil {
			logger.Fatal(err)
		}
		defer func() { _ = closer.Close() }()

		enc := yaml.NewEncoder(cmd.OutOrStdout())
		defer func() { _ = enc.Close() }()

		enc.SetIndent(2)
		if projectInit != nil {
			_ = enc.Encode(projectInit)
			_ = enc.Encode(projectInit.Environment)
		}
		_ = enc.Encode(prj)
		_ = enc.Encode(prj.Environment)
		return
	}

	if projectInit != nil {
		if err = comp.Up(cmd.Context(), projectInit); err != nil {
			logger.Fatal(err)
		}

		if pgDebug {
			return
		}

		if err = comp.Down(cmd.Context(), projectInit.Name, false); err != nil {
			logger.Fatal(err)
		}
	}

	if err = comp.Up(cmd.Context(), prj); err != nil {
		logger.Fatal(err)
	}
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
	opts := dockerCompose.ProjectOptions{
		Offline: true,
	}
	var sec *xsecrets
	projectOpts := []cli.ProjectOptionsFn{
		cli.WithConsistency(false),
		cli.WithDotEnv,
		WithConfigFileEnv,
		WithEnvFiles(opts.EnvFiles...),
		cli.WithExtension(xsecretsKey, sec),
	}

	if len(conf) > 0 {
		opts.ConfigPaths = []string{conf}
	} else {
		projectOpts = append(projectOpts, WithDefaultConfigPath)
	}

	prj, _, err := opts.ToProject(context.Background(), nil, nil, projectOpts...)

	return prj, err
}

// WithConfigFileEnv allow to set compose config file paths by ARENADATA_APP_FILE environment variable
func WithConfigFileEnv(o *cli.ProjectOptions) error {
	if len(o.ConfigPaths) > 0 {
		return nil
	}
	sep := o.Environment[PathSeparator]
	if sep == "" {
		sep = string(os.PathListSeparator)
	}
	f, ok := o.Environment[FilePath]
	if ok {
		paths, err := absolutePaths(strings.Split(f, sep))
		o.ConfigPaths = paths
		return err
	}
	return nil
}

func absolutePaths(p []string) ([]string, error) {
	var paths []string
	for _, f := range p {
		if f == "-" {
			paths = append(paths, f)
			continue
		}
		abs, err := filepath.Abs(f)
		if err != nil {
			return nil, err
		}
		f = abs
		if _, err := os.Stat(f); err != nil {
			return nil, err
		}
		paths = append(paths, f)
	}
	return paths, nil
}

// WithDefaultConfigPath searches for default config files from working directory
func WithDefaultConfigPath(o *cli.ProjectOptions) error {
	if len(o.ConfigPaths) > 0 {
		return nil
	}
	pwd, err := o.GetWorkingDir()
	if err != nil {
		return err
	}
	for {
		candidates := findFiles(fileNames, pwd)
		if len(candidates) > 0 {
			winner := candidates[0]
			if len(candidates) > 1 {
				log.Warnf("Found multiple config files with supported names: %s", strings.Join(candidates, ", "))
				log.Warnf("Using %s", winner)
			}
			o.ConfigPaths = append(o.ConfigPaths, winner)

			return nil
		}
		parent := filepath.Dir(pwd)
		if parent == pwd {
			// no config file found, but that's not a blocker if caller only needs project name
			return nil
		}
		pwd = parent
	}
}

// WithEnvFiles set env file(s) to be loaded to set project environment.
// defaults to local .env file if no explicit file is selected, until ARENADATA_APP_DISABLE_ENV_FILE is set
func WithEnvFiles(file ...string) cli.ProjectOptionsFn {
	return func(o *cli.ProjectOptions) error {
		if len(file) > 0 {
			o.EnvFiles = file
			return nil
		}
		if v, ok := os.LookupEnv(DisableDefaultEnvFile); ok {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return err
			}
			if b {
				return nil
			}
		}

		wd, err := o.GetWorkingDir()
		if err != nil {
			return err
		}
		defaultDotEnv := filepath.Join(wd, ".env")

		s, err := os.Stat(defaultDotEnv)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if !s.IsDir() {
			o.EnvFiles = []string{defaultDotEnv}
		}
		return nil
	}
}

func newInitProject(project *composeTypes.Project, pgDebug bool) (*composeTypes.Project, error) {
	helpers := compose.NewModHelpers()
	projectInit := &composeTypes.Project{
		Name:        project.Name,
		Services:    composeTypes.Services{},
		Environment: project.Environment,
		Secrets:     project.Secrets,
		Volumes:     project.Volumes,
	}

	var adpgInitServiceName string
	pgInit := types.NewPGInit()
	for svcName, svc := range project.Services {
		svcType := svc.Labels[compose.ADAppTypeLabelKey]

		var chownServiceName string
		if len(svc.User) > 0 && len(svc.Volumes) > 0 {
			var mounts []string
			for _, mnt := range svc.Volumes {
				mounts = append(mounts, mnt.Target)
			}

			chownServiceName = "chown-" + svcName
			dirs := strings.Join(mounts, " ")
			projectInit.Services[chownServiceName] = composeTypes.ServiceConfig{
				User:       "0:0",
				Image:      assets.ImageName,
				Entrypoint: composeTypes.ShellCommand{"/bin/sh"},
				Command: []string{
					"-cex",
					fmt.Sprintf("chown %s %s", svc.User, dirs),
				},
				Volumes: svc.Volumes,
			}
		}

		switch svcType {
		case "adcm":
			dbName := projectInit.Environment["adcm-db-name"]
			dbOwner := projectInit.Environment["adcm-db-user"]
			pgInit.DB[dbName] = &types.Database{
				Owner: dbOwner,
			}
			pgInit.Role[dbOwner] = &types.Role{
				Password: projectInit.Environment["adcm-db-pass"],
			}
		case "adpg":
			adpgInitServiceName = "init-" + svcName
			svcConf := composeTypes.ServiceConfig{
				User:    svc.User,
				Image:   svc.Image,
				Command: []string{"initdb"},
				Volumes: svc.Volumes,
				Secrets: svc.Secrets,
				Environment: composeTypes.MappingWithEquals{
					"PG_ENTRYPOINT_LOG_DEBUG": utils.Ptr("true"),
				},
			}
			if pgDebug {
				svcConf.Environment["PG_ENTRYPOINT_INIT_DEBUG"] = utils.Ptr("true")
			}
			projectInit.Services[adpgInitServiceName] = svcConf

			helpers = append(helpers, compose.Secrets(adpgInitServiceName, compose.Secret{
				Source: "adpg-password",
				EnvKey: "POSTGRES_PASSWORD",
				ENV:    false,
			}))

			if len(chownServiceName) > 0 && !pgDebug {
				helpers = append(helpers,
					compose.DependsOn(adpgInitServiceName,
						compose.Depended{
							Service:   chownServiceName,
							Condition: composeTypes.ServiceConditionCompletedSuccessfully,
						}),
				)
			}
		//case "consul":
		case "vault":
			//dbName := project.Environment["vault-db-name"]
			//dbOwner := project.Environment["vault-db-user"]
			//pgInit.DB[dbName] = &types.Database{
			//	Owner: dbOwner,
			//}
			//pgInit.Role[dbOwner] = &types.Role{
			//	Password: project.Environment["vault-db-pass"],
			//}
		}
	}

	if len(pgInit.DB) > 0 || len(pgInit.Role) > 0 {
		initJson, err := json.Marshal(pgInit)
		if err != nil {
			return nil, err
		}

		helpers = append(helpers, compose.Secrets(adpgInitServiceName, compose.Secret{
			Source: "init.json",
			EnvKey: "POSTGRES_INITDB",
			Value:  string(initJson),
			ENV:    false,
		}))
	}

	const pauseContainerName = "pause"
	var deps []compose.Depended
	for svcName := range projectInit.Services {
		deps = append(deps, compose.Depended{Service: svcName, Condition: composeTypes.ServiceConditionCompletedSuccessfully})
	}
	helpers = append(helpers, compose.DependsOn(pauseContainerName, deps...))

	projectInit.Services[pauseContainerName] = composeTypes.ServiceConfig{
		Name:       pauseContainerName,
		Image:      assets.ImageName,
		Command:    []string{"sleep", "120"},
		StopSignal: "SIGKILL",
	}

	if err := helpers.Run(projectInit); err != nil {
		return nil, err
	}

	fillProjectFields(projectInit)

	return projectInit, nil
}

func fillProjectFields(project *composeTypes.Project) {
	for name, s := range project.Services {
		s.Name = name

		if s.CustomLabels == nil {
			s.CustomLabels = map[string]string{}
		}

		s.CustomLabels[api.ProjectLabel] = project.Name
		s.CustomLabels[api.ServiceLabel] = name
		s.CustomLabels[api.VersionLabel] = api.ComposeVersion
		s.CustomLabels[api.WorkingDirLabel] = project.WorkingDir
		s.CustomLabels[api.ConfigFilesLabel] = strings.Join(project.ComposeFiles, ",")
		s.CustomLabels[api.OneoffLabel] = "False"

		project.Services[name] = s
	}
}

func fillADCMLabels(project *composeTypes.Project) {
	for name, s := range project.Services {
		s.Name = name

		if s.CustomLabels == nil {
			s.CustomLabels = map[string]string{}
		}
		s.CustomLabels[compose.ADLabel] = ""

		project.Services[name] = s
	}
}
