package cmd

import (
	"fmt"
	"io"
	"os"
	"path"
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
	"github.com/gosimple/slug"
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

type initConfig struct {
	ADCMDBHost        string `yaml:"adcm-db-host"`
	ADCMDBPort        uint16 `yaml:"adcm-db-port"`
	ADCMDBName        string `yaml:"adcm-db-name"`
	ADCMDBUser        string `yaml:"adcm-db-user"`
	ADCMDBPassword    string `yaml:"adcm-db-pass"`
	ADCMDBSSLMode     string `yaml:"adcm-db-ssl-mode"`
	ADCMDBSSLCaFile   string `yaml:"adcm-db-ssl-ca-file"`
	ADCMDBSSLCertFile string `yaml:"adcm-db-ssl-cert-file"`
	ADCMDBSSLKeyFile  string `yaml:"adcm-db-ssl-key-file"`
	ADCMImage         string `yaml:"adcm-image"`
	ADCMTag           string `yaml:"adcm-tag"`
	ADCMPublishPort   uint16 `yaml:"adcm-publish-port"`
	ADCMUrl           string `yaml:"adcm-url"`
	ADCMVolume        string `yaml:"adcm-volume"`

	ADPGPassword    string `yaml:"adpg-pass"`
	ADPGImage       string `yaml:"adpg-image"`
	ADPGTag         string `yaml:"adpg-tag"`
	ADPGPublishPort uint16 `yaml:"adpg-publish-port"`

	ConsulImage       string `yaml:"consul-image"`
	ConsulTag         string `yaml:"consul-tag"`
	ConsulPublishPort uint16 `yaml:"consul-publish-port"`

	VaultImage       string `yaml:"vault-image"`
	VaultTag         string `yaml:"vault-tag"`
	VaultPublishPort uint16 `yaml:"vault-publish-port"`
}

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

	// TODO: merge with initConfigDefaults
	services = map[string]service{
		svcNameAdcm:   {name: "ADCM", image: compose.ADCMImage, tag: "2.6.0", port: 8000, mounts: []string{"/adcm/data"}},
		svcNameAdpg:   {name: "ADPG", image: compose.ADPGImage, tag: "v16.3.1", port: 5432, mounts: []string{"/data"}},
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
	f.String("from-config", "", "Read variables from config file")
	initCmd.MarkFlagsMutuallyExclusive("from-config", "interactive")
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

	var isValuesReadFromFile bool
	config := &initConfig{}
	if configFile, _ := cmd.Flags().GetString("from-config"); len(configFile) > 0 {
		config, err = valuesFromConfigFile(configFile)
		if err != nil {
			logger.Fatal(err)
		}
		isValuesReadFromFile = true
	}
	initConfigDefaults(config)

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

	if !isValuesReadFromFile && (!managedADPG || interactive) {
		if !managedADPG {
			portStr := strconv.Itoa(int(config.ADCMDBPort))
			wrap(&config.ADCMDBHost, "ADCM database host:", "", true, false)
			wrap(&config.ADCMDBPort, "ADCM database port:", portStr, false, false)
		}

		wrap(&config.ADCMDBName, "ADCM database name:", config.ADCMDBName, false, false)
		wrap(&config.ADCMDBUser, "ADCM database user:", config.ADCMDBUser, false, false)

		if managedADPG {
			wrap(&config.ADCMDBPassword, "ADCM database password (random generated):", "", false, true)
		} else {
			wrap(&config.ADCMDBPassword, "ADCM database password:", "", true, true)
			config.ADCMDBSSLMode, err = selectValue("Select Postgres SSL mode:", postgresSSLMode, allowSSLModes)
			if err != nil {
				logger.Fatal(err)
			}

			if config.ADCMDBSSLMode != postgresSSLMode {
				wrap(&config.ADCMDBSSLCaFile, "ADCM database SSL CA file path:", "", false, false)
				wrap(&config.ADCMDBSSLCertFile, "ADCM database SSL certificate file path:", "", false, false)
				wrap(&config.ADCMDBSSLKeyFile, "ADCM database SSL private key file path:", "", false, false)
			}
		}

		wrap(&config.ADCMVolume, "ADCM volume name or path:", config.ADCMVolume, false, false)
	}

	if len(config.ADCMDBPassword) == 0 {
		config.ADCMDBPassword = utils.GenerateRandomString(16)
	}

	if !managedADPG {
		portStr := strconv.Itoa(int(config.ADCMDBPort))
		helpers = append(helpers, compose.Environment(svcNameAdcm,
			compose.Env{Name: "DB_HOST", Value: &config.ADCMDBHost},
			compose.Env{Name: "DB_PORT", Value: &portStr},
		))
	}

	if config.ADCMDBSSLMode != postgresSSLMode {
		sslOpts := types.DbSSLOptions{SSLMode: config.ADCMDBSSLMode}

		optStr := sslOpts.String()
		helpers = append(helpers, compose.Environment(svcNameAdcm,
			compose.Env{Name: pgSslOptEnvKey, Value: &optStr},
		))

		if len(config.ADCMDBSSLCaFile) > 0 {
			b, err := os.ReadFile(config.ADCMDBSSLCaFile)
			if err != nil {
				logger.Fatal(err)
			}
			xsecretsData[pgSslCaKey] = string(b)
		}
		if len(config.ADCMDBSSLCertFile) > 0 {
			b, err := os.ReadFile(config.ADCMDBSSLCertFile)
			if err != nil {
				logger.Fatal(err)
			}
			xsecretsData[pgSslCertKey] = string(b)
		}
		if len(config.ADCMDBSSLKeyFile) > 0 {
			b, err := os.ReadFile(config.ADCMDBSSLKeyFile)
			if err != nil {
				logger.Fatal(err)
			}
			xsecretsData[pgSslKeyKey] = string(b)
		}
	}

	xsecretsData["adcm-db-name"] = config.ADCMDBName
	xsecretsData["adcm-db-user"] = config.ADCMDBUser
	xsecretsData["adcm-db-pass"] = config.ADCMDBPassword

	if interactive {
		adcmPublishPortDefault := strconv.Itoa(int(config.ADCMPublishPort))
		wrap(&config.ADCMImage, "ADCM image", config.ADCMImage, false, false)
		wrap(&config.ADCMTag, "ADCM image tag", config.ADCMTag, false, false)
		wrap(&config.ADCMPublishPort, "ADCM publish port", adcmPublishPortDefault, false, false)
	}

	helpers = append(helpers,
		compose.Image(svcNameAdcm, config.ADCMImage+":"+config.ADCMTag),
	)
	if len(config.ADCMUrl) == 0 && config.ADCMPublishPort > 0 {
		var adcmUrl string
		if hostIp := utils.HostIp(); len(hostIp) > 0 {
			// TODO: automatically switch http to https
			adcmUrl = fmt.Sprintf("http://%s:%d", hostIp, config.ADCMPublishPort)
		}
		if interactive {
			wrap(&config.ADCMUrl, "ADCM url", adcmUrl, false, false)
		}

		helpers = append(helpers,
			compose.Environment(svcNameAdcm, compose.Env{Name: "ADCM_URL", Value: &adcmUrl}),
			compose.PublishPort(svcNameAdcm, config.ADCMPublishPort, services[svcNameAdcm].port),
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

		if !isValuesReadFromFile && interactive {
			wrap(&config.ADPGPassword, "ADPG superuser password (random generated):", "", false, true)

			wrap(&config.ADPGImage, "ADPG image", config.ADPGImage, false, false)
			wrap(&config.ADPGTag, "ADPG image tag", config.ADPGTag, false, false)
			wrap(&config.ADPGPublishPort, "ADPG publish port", "0", false, false)
		}

		if len(config.ADPGPassword) == 0 {
			config.ADPGPassword = utils.GenerateRandomString(16)
		}
		xsecretsData["adpg-password"] = config.ADPGPassword

		helpers = append(helpers,
			compose.Image(svcNameAdpg, config.ADPGImage+":"+config.ADPGTag),
		)
		if config.ADPGPublishPort > 0 {
			helpers = append(helpers, compose.PublishPort(svcNameAdpg, config.ADPGPublishPort, services[svcNameAdpg].port))
		}
	}

	if managedConsul {
		// TODO: healthcheck
		addService(svcNameConsul, prj)
		helpers = append(helpers,
			compose.Labels(svcNameConsul, map[string]string{compose.ADAppTypeLabelKey: "consul"}),
			compose.Command(svcNameConsul, []string{"agent", "-dev", "-bind=0.0.0.0"}),
		)

		if !isValuesReadFromFile && interactive {
			portStr := strconv.Itoa(int(config.ConsulPublishPort))
			wrap(&config.ConsulImage, "Consul image", config.ConsulImage, false, false)
			wrap(&config.ConsulTag, "Consul image tag", config.ConsulTag, false, false)
			wrap(&config.ConsulPublishPort, "Consul publish port", portStr, false, false)
		}

		helpers = append(helpers,
			compose.Image(svcNameConsul, config.ConsulImage+":"+config.ConsulTag),
		)
		if config.ConsulPublishPort > 0 {
			helpers = append(helpers, compose.PublishPort(svcNameConsul, config.ConsulPublishPort, services[svcNameConsul].port))
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

		if !isValuesReadFromFile && interactive {
			wrap(&config.VaultImage, "Vault image", config.VaultImage, false, false)
			wrap(&config.VaultTag, "Vault image tag", config.VaultTag, false, false)
			wrap(&config.VaultPublishPort, "Vault publish port", "0", false, false)
		}

		helpers = append(helpers,
			compose.Image(svcNameVault, config.VaultImage+":"+config.VaultTag),
		)
		if config.VaultPublishPort > 0 {
			helpers = append(helpers, compose.PublishPort(svcNameVault, config.VaultPublishPort, services[svcNameVault].port))
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
		hostname := prj.Name + "-" + svcName
		helpers = append(helpers,
			compose.CapDropAll(svcName),
			compose.Hostname(svcName, hostname),
			compose.RestartPolicy(svcName, composeTypes.RestartPolicyUnlessStopped),
			compose.PullPolicy(svcName, composeTypes.PullPolicyAlways),
		)

		svcMounts := services[svcName].mounts
		volumeName := hostname

		if svcName == svcNameAdcm {
			volumeName = config.ADCMVolume
		} else {
			uids := strconv.Itoa(uid)
			uid++
			helpers = append(helpers,
				compose.ReadOnlyRootFilesystem(svcName),
				compose.SecurityOptsNoNewPrivileges(svcName),
				compose.User(svcName, uids, uids),
			)
		}

		for _, mount := range svcMounts {
			mount = strings.TrimRight(mount, "/")
			if len(svcMounts) > 1 {
				volumeName += "-" + slug.Make(strings.TrimLeft(mount, "/"))
			}
			helpers = append(helpers, compose.Volumes(svcName, volumeName+":"+mount))
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
			svc.Secrets = append(svc.Secrets, composeTypes.ServiceSecretConfig{
				Source: k,
				Target: path.Join(compose.SecretsPath, k),
			})
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

func valuesFromConfigFile(configFile string) (*initConfig, error) {
	fi, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer fi.Close()

	var config *initConfig
	dec := yaml.NewDecoder(fi)
	dec.KnownFields(true)
	if err = dec.Decode(&config); err != nil {
		return nil, err
	}

	return config, nil
}

func initConfigDefaults(config *initConfig) {
	if config.ADCMDBPort == 0 {
		config.ADCMDBPort = 5432
	}
	if len(config.ADCMDBName) == 0 {
		config.ADCMDBName = "adcm"
	}
	if len(config.ADCMDBUser) == 0 {
		config.ADCMDBUser = "adcm"
	}
	if len(config.ADCMDBSSLMode) == 0 {
		config.ADCMDBSSLMode = postgresSSLMode
	}
	if config.ADCMPublishPort == 0 {
		config.ADCMPublishPort = 8000
	}
	if len(config.ADCMImage) == 0 {
		config.ADCMImage = compose.ADCMImage
	}
	if len(config.ADCMTag) == 0 {
		config.ADCMTag = "2.6.0"
	}
	if config.ADCMPublishPort == 0 {
		config.ADCMPublishPort = 8000
	}
	if len(config.ADCMVolume) == 0 {
		config.ADCMVolume = "adcm"
	}

	if len(config.ADPGImage) == 0 {
		config.ADPGImage = compose.ADPGImage
	}
	if len(config.ADPGTag) == 0 {
		config.ADPGTag = "v16.3.1"
	}

	if len(config.ConsulImage) == 0 {
		config.ConsulImage = compose.ConsulImage
	}
	if len(config.ConsulTag) == 0 {
		config.ConsulTag = "v0.0.0"
	}
	if config.ConsulPublishPort == 0 {
		config.ConsulPublishPort = 8500
	}

	if len(config.VaultImage) == 0 {
		config.VaultImage = compose.VaultImage
	}
	if len(config.VaultTag) == 0 {
		config.VaultTag = "2.2.0"
	}
	if config.VaultPublishPort == 0 {
		config.VaultPublishPort = 8200
	}
}
