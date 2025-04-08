package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/arenadata/adcm-installer/pkg/compose"
	"github.com/arenadata/adcm-installer/pkg/types"
	"github.com/arenadata/adcm-installer/pkg/utils"

	"github.com/AlecAivazis/survey/v2"
	composeTypes "github.com/compose-spec/compose-go/v2/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	postgresSSLMode = "disable"

	svcNameAdcm   = "adcm"
	svcNameAdpg   = "adpg"
	svcNameConsul = "consul"
	svcNameVault  = "vault"

	pgSslCaKey     = "adcm-pg-ssl-ca"
	pgSslCertKey   = "adcm-pg-ssl-cert"
	pgSslKeyKey    = "adcm-pg-ssl-key"
	pgSslOptEnvKey = "DB_OPTIONS"
)

type xsecrets struct {
	AgeRecipient string            `yaml:"age_recipient" mapstructure:"age_recipient"`
	Data         map[string]string `yaml:"data" mapstructure:"data"`
}

type service struct {
	name   string
	image  string
	tag    string
	port   uint16
	mounts []string
}

func (img service) Image() string {
	return fmt.Sprintf("%s:%s", img.image, img.tag)
}

var (
	initCmd = &cobra.Command{
		Use:     "init <name>",
		Short:   "Initialize a new configuration",
		PreRunE: cobra.ExactArgs(1),
		Run:     initProject,
	}

	services = map[string]service{
		svcNameAdcm:   {name: "ADCM", image: compose.ADCMImage, tag: "2.5.0", port: 8000, mounts: []string{"/adcm/data"}},
		svcNameAdpg:   {name: "ADPG", image: compose.ADPGImage, tag: "16.4", port: 5432, mounts: []string{"/data"}},
		svcNameVault:  {name: "Vault", image: compose.VaultImage, tag: "2.2.0", port: 8200, mounts: []string{"/openbao/file", "/openbao/logs"}},
		svcNameConsul: {name: "Consul", image: compose.ConsulImage, tag: "v0.0.0", port: 8500, mounts: []string{"/data"}},
	}

	allowSSLModes = []string{postgresSSLMode, "allow", "prefer", "require", "verify-ca", "verify-full"}
	mapFlagsToEnv = map[string]string{
		"adcm-db-host": "DB_HOST",
		"adcm-db-port": "DB_PORT",
		"adcm-db-name": "DB_NAME",
		"adcm-db-user": "DB_USER",
		"adcm-db-pass": "DB_PASS",
	}
)

func init() {
	rootCmd.AddCommand(initCmd)

	ageKeyFlags(initCmd, "age-key", ageKeyFileName)

	f := initCmd.Flags()

	f.Bool(svcNameAdpg, false, "Use managed ADPG")
	f.Bool(svcNameConsul, false, "Use managed Consul")
	f.Bool(svcNameVault, false, "Use managed Vault")
	f.Bool("force", false, "Force overwrite existing config file")
	f.BoolP("interactive", "i", false, "Interactive mode")

	f.StringP("output", "o", "", "Output filename")
}

func initProject(cmd *cobra.Command, args []string) {
	logger := log.WithField("command", "init")

	if err := isConfigExists(cmd); err != nil {
		logger.Fatal(err)
	}

	crypt, ok, err := readOrCreateNewAgeKey(cmd, "age-key")
	if err != nil {
		logger.Fatal(err)
	}
	if ok {
		if err = saveAgeKey(ageKeyFileName, crypt); err != nil {
			logger.Fatal(err)
		}
	}

	interactive := getBool(cmd, "interactive")
	managedADPG := getBool(cmd, svcNameAdpg)
	managedConsul := getBool(cmd, svcNameConsul)
	managedVault := getBool(cmd, svcNameVault)

	prj := &composeTypes.Project{
		Name:     args[0],
		Services: composeTypes.Services{},
	}

	wrap := func(v any, p, d string, r, s bool) {
		if err = readValue(v, p, d, r, s); err != nil {
			logger.Fatal(err)
		}
	}

	helpers := compose.NewModHelpers()

	// TODO: user, healthcheck, read_only, security_opt
	addService(svcNameAdcm, prj)
	helpers = append(helpers,
		compose.CapAdd(svcNameAdcm, "CAP_CHOWN", "CAP_SETUID", "CAP_SETGID"),
		compose.Labels(svcNameAdcm, map[string]string{compose.ADAppTypeLabelKey: "adcm"}),
	)

	xsecretsData := map[string]string{}

	dbName := "adcm"
	dbUser := "adcm"
	var dbPass string
	if !managedADPG || interactive {
		if !managedADPG {
			var dbHost string
			var dbPort uint16
			dbPortDefault := strconv.Itoa(int(services[svcNameAdpg].port))
			wrap(&dbHost, "ADCM database host:", "", true, false)
			wrap(&dbPort, "ADCM database port:", dbPortDefault, false, false)

			portStr := strconv.Itoa(int(dbPort))
			helpers = append(helpers, compose.Environment(svcNameAdcm,
				compose.Env{Name: "DB_HOST", Value: &dbHost},
				compose.Env{Name: "DB_PORT", Value: &portStr},
			))
		}

		wrap(&dbName, "ADCM database name:", dbName, false, false)
		wrap(&dbUser, "ADCM database user:", dbName, false, false)

		if managedADPG {
			wrap(&dbPass, "ADCM database password (random generated):", "", false, true)
		} else {
			wrap(&dbPass, "ADCM database password:", "", true, true)
			sslMode, err := selectValue("Select Postgres SSL mode:", postgresSSLMode, allowSSLModes)
			if err != nil {
				logger.Fatal(err)
			}

			if sslMode != postgresSSLMode {
				sslOpts := types.DbSSLOptions{SSLMode: sslMode}

				optStr := sslOpts.String()
				helpers = append(helpers, compose.Environment(svcNameAdcm,
					compose.Env{Name: pgSslOptEnvKey, Value: &optStr},
				))

				var sslCa, sslCert, sslKey string
				wrap(&sslCa, "ADCM database SSL CA file path:", "", false, false)
				wrap(&sslCert, "ADCM database SSL certificate file path:", "", false, false)
				wrap(&sslKey, "ADCM database SSL private key file path:", "", false, false)

				if len(sslCa) > 0 {
					b, err := os.ReadFile(sslCa)
					if err != nil {
						logger.Fatal(err)
					}
					xsecretsData[pgSslCaKey] = string(b)
				}
				if len(sslCert) > 0 {
					b, err := os.ReadFile(sslCert)
					if err != nil {
						logger.Fatal(err)
					}
					xsecretsData[pgSslCertKey] = string(b)
				}
				if len(sslKey) > 0 {
					b, err := os.ReadFile(sslKey)
					if err != nil {
						logger.Fatal(err)
					}
					xsecretsData[pgSslKeyKey] = string(b)
				}
			}
		}
	}

	if len(dbPass) == 0 {
		dbPass = utils.GenerateRandomString(16)
	}

	xsecretsData["adcm-db-name"] = dbName
	xsecretsData["adcm-db-user"] = dbUser
	xsecretsData["adcm-db-pass"] = dbPass

	adcmImage := services[svcNameAdcm].image
	adcmTag := services[svcNameAdcm].tag
	adcmPublishPort := services[svcNameAdcm].port

	if interactive {
		adcmPublishPortDefault := strconv.Itoa(int(adcmPublishPort))
		wrap(&adcmImage, "ADCM image", adcmImage, false, false)
		wrap(&adcmTag, "ADCM image tag", adcmTag, false, false)
		wrap(&adcmPublishPort, "ADCM publish port", adcmPublishPortDefault, false, false)
	}

	helpers = append(helpers,
		compose.Image(svcNameAdcm, adcmImage+":"+adcmTag),
	)
	if adcmPublishPort > 0 {
		helpers = append(helpers,
			compose.PublishPort(svcNameAdcm, adcmPublishPort, services[svcNameAdcm].port),
		)
	}

	if managedADPG {
		addService(svcNameAdpg, prj)

		helpers = append(helpers,
			compose.DependsOn(svcNameAdcm, compose.Depended{Service: svcNameAdpg}),
			compose.Labels(svcNameAdpg, map[string]string{compose.ADAppTypeLabelKey: "adpg"}),
			compose.HealthCheck(svcNameAdpg, compose.HealthCheckConfig{
				Cmd:      []string{"CMD-SHELL", "pg-entrypoint isready postgres"},
				Interval: 10 * time.Second,
				Timeout:  3 * time.Second,
				Retries:  3,
			}),
		)

		var postgresPassword string
		var port uint16
		image := services[svcNameAdpg].image
		tag := services[svcNameAdpg].tag

		if interactive {
			wrap(&postgresPassword, "ADCM superuser password (random generated):", "", false, true)

			wrap(&image, "ADPG image", image, false, false)
			wrap(&tag, "ADPG image tag", tag, false, false)
			wrap(&port, "ADPG publish port", "0", false, false)
		}

		if len(postgresPassword) == 0 {
			postgresPassword = utils.GenerateRandomString(16)
		}
		xsecretsData["adpg-password"] = postgresPassword

		helpers = append(helpers,
			compose.Image(svcNameAdpg, image+":"+tag),
		)
		if port > 0 {
			helpers = append(helpers, compose.PublishPort(svcNameAdpg, port, services[svcNameAdpg].port))
		}
	}

	if managedConsul {
		// TODO: healthcheck
		addService(svcNameConsul, prj)
		helpers = append(helpers,
			compose.Labels(svcNameConsul, map[string]string{compose.ADAppTypeLabelKey: "consul"}),
			compose.Command(svcNameConsul, []string{"agent", "-dev", "-bind=0.0.0.0"}),
		)

		image := services[svcNameConsul].image
		tag := services[svcNameConsul].tag
		port := services[svcNameConsul].port

		if interactive {
			wrap(&image, "Consul image", image, false, false)
			wrap(&tag, "Consul image tag", tag, false, false)
			wrap(&port, "Consul publish port", strconv.Itoa(int(port)), false, false)
		}

		helpers = append(helpers,
			compose.Image(svcNameConsul, image+":"+tag),
		)
		if port > 0 {
			helpers = append(helpers, compose.PublishPort(svcNameConsul, port, services[svcNameConsul].port))
		}
	} else {
		//"Consul url: (required)"
		//"Consul token: (required)"
	}

	if managedVault {
		addService(svcNameVault, prj)
		helpers = append(helpers,
			compose.Labels(svcNameVault, map[string]string{compose.ADAppTypeLabelKey: "vault"}),
			compose.HealthCheck(svcNameVault, compose.HealthCheckConfig{
				Cmd:         []string{"CMD-SHELL", "wget -q -O - http://vault:8200/v1/sys/health"},
				Interval:    3 * time.Second,
				Timeout:     5 * time.Second,
				StartPeriod: 5 * time.Second,
			}),
			compose.Environment(svcNameVault,
				compose.Env{Name: "BAO_DEV_ROOT_TOKEN_ID", Value: utils.Ptr("openbao_secret")},
				compose.Env{Name: "BAO_DEV_LISTEN_ADDRESS", Value: utils.Ptr("0.0.0.0:8200")},
			),
		)

		if managedADPG {
			helpers = append(helpers,
				compose.DependsOn(svcNameVault, compose.Depended{Service: svcNameAdpg}),
			)
		}

		image := services[svcNameVault].image
		tag := services[svcNameVault].tag
		port := services[svcNameVault].port

		if interactive {
			wrap(&image, "Vault image", image, false, false)
			wrap(&tag, "Vault image tag", tag, false, false)
			wrap(&port, "Vault publish port", "0", false, false)
		}

		helpers = append(helpers,
			compose.Image(svcNameVault, image+":"+tag),
		)
		if port > 0 {
			helpers = append(helpers, compose.PublishPort(svcNameVault, port, services[svcNameVault].port))
		}
	} else {
		//"Vault url: (required)"
		//"Vault token: (required)"
	}

	var svcList []string
	for k := range prj.Services {
		svcList = append(svcList, k)
	}
	sort.Strings(svcList)

	uid := 10001
	for _, svcName := range svcList {
		if svcName != "adcm" {
			uids := strconv.Itoa(uid)
			uid++
			helpers = append(helpers,
				compose.ReadOnlyRootFilesystem(svcName),
				compose.SecurityOptsNoNewPrivileges(svcName),
				compose.User(svcName, uids, uids),
			)
		}

		hostname := prj.Name + "-" + svcName
		helpers = append(helpers,
			compose.CapDropAll(svcName),
			compose.Hostname(svcName, hostname),
			compose.RestartPolicy(svcName, composeTypes.RestartPolicyUnlessStopped),
			compose.PullPolicy(svcName, composeTypes.PullPolicyAlways),
		)
		svcMounts := services[svcName].mounts
		for _, mount := range svcMounts {
			mount = strings.TrimRight(mount, "/")
			src := hostname
			if len(svcMounts) > 1 {
				idx := strings.LastIndex(mount, "/")
				src += "-" + mount[idx+1:]
			}
			helpers = append(helpers, compose.Volumes(svcName, src+":"+mount))
		}
	}

	for k, v := range xsecretsData {
		v, err = crypt.EncryptValue(v)
		if err != nil {
			logger.Fatal(err)
		}
		xsecretsData[k] = v
		if k == "adpg-password" {
			continue
		}

		keyParts := strings.SplitN(k, "-", 2)
		svcName := keyParts[0]
		if svc, ok := prj.Services[svcName]; ok {
			svc.Secrets = append(svc.Secrets, composeTypes.ServiceSecretConfig{Source: k})
			prj.Services[svcName] = svc
		}
	}

	prj.Extensions = map[string]any{"x-secrets": &xsecrets{
		AgeRecipient: crypt.Recipient().String(),
		Data:         xsecretsData,
	}}

	if err = helpers.Run(prj); err != nil {
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

	var tmp *os.File
	tmp, err = os.CreateTemp("", "adcm.tmp")
	if err != nil {
		return
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()
	if err = toYaml(tmp, prj); err != nil {
		return
	}
	if err = tmp.Close(); err != nil {
		return
	}

	prj, err = readConfigFile(tmp.Name())
	if err != nil {
		return
	}
	err = toYaml(cmd.OutOrStdout(), prj)
}

func toYaml(output io.Writer, v any) (err error) {
	enc := yaml.NewEncoder(output)
	defer func() {
		if e := enc.Close(); e != nil {
			err = e
		}
	}()

	enc.SetIndent(2)
	return enc.Encode(v)
}

func isConfigExists(cmd *cobra.Command) error {
	outputPath, _ := cmd.Flags().GetString("output")
	if len(outputPath) == 0 {
		absPath, err := filepath.Abs(outputPath)
		if err != nil {
			return fmt.Errorf("could not determine absolute path: %s", err)
		}
		workingDir := filepath.Dir(absPath)

		outputPath = fileNames[0]
		if candidates := findFiles(fileNames, workingDir); len(candidates) > 0 {
			outputPath = candidates[0]
		}
		_ = cmd.Flags().Set("output", outputPath)
	}

	force, _ := cmd.Flags().GetBool("force")
	if ok, err := utils.FileExists(outputPath); err != nil {
		return err
	} else if ok && !force {
		return fmt.Errorf("config file %q already exists", outputPath)
	}
	return nil
}

func setOutput(cmd *cobra.Command) (io.Closer, error) {
	outputPath, _ := cmd.Flags().GetString("output")
	if len(outputPath) == 0 || outputPath == "-" {
		return os.Stdout, nil
	}

	output, err := os.OpenFile(outputPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0640)
	if err != nil {
		return nil, err
	}
	cmd.SetOut(output)
	return output, nil
}

func addService(name string, prj *composeTypes.Project) {
	prj.Services[name] = composeTypes.ServiceConfig{Name: name}
}

func readValue(v any, prompt, defaultValue string, required, secret bool) error {
	var promptType survey.Prompt
	if secret {
		promptType = &survey.Password{
			Message: prompt,
		}
	} else {
		promptType = &survey.Input{
			Message: prompt,
			Default: defaultValue,
		}
	}

	q := &survey.Question{
		Prompt: promptType,
	}

	if required {
		q.Validate = survey.Required
	}

	return survey.Ask([]*survey.Question{q}, v)
}

func selectValue(prompt, defaultValue string, options []string) (string, error) {
	var v string
	if err := survey.AskOne(
		&survey.Select{
			Message: prompt,
			Options: options,
			Default: defaultValue,
		},
		&v,
	); err != nil {
		return "", fmt.Errorf("%s, failed UserInput", err)
	}

	return v, nil
}
