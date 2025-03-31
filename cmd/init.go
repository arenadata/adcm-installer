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
	"github.com/arenadata/adcm-installer/pkg/utils"

	"github.com/AlecAivazis/survey/v2"
	composeTypes "github.com/compose-spec/compose-go/v2/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

const (
	postgresSSLMode = "disable"
)

type xsecrets struct {
	AgeRecipient string            `yaml:"age_recipient" mapstructure:"age_recipient"`
	Data         map[string]string `yaml:"data" mapstructure:"data"`
}

type notSet struct {
	key string
}

func (n *notSet) Error() string {
	return fmt.Sprintf("flag --%s required an argument", n.key)
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
		Use:   "init",
		Short: "Initialize a new configuration",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := validateRequiredFlag(cmd, "adpg-password", "adpg"); err != nil {
				return err
			}

			return nil
		},
		Run: initProject,
	}

	services = map[string]service{
		"adcm":   {name: "ADCM", image: compose.ADCMImage, tag: "2.5.0", port: 8000, mounts: []string{"/adcm/data"}},
		"adpg":   {name: "ADPG", image: compose.ADPGImage, tag: "16.4", port: 5432, mounts: []string{"/data"}},
		"vault":  {name: "Vault", image: compose.VaultImage, tag: "2.2.0", port: 8200, mounts: []string{"/openbao/file", "/openbao/logs"}},
		"consul": {name: "Consul", image: compose.ConsulImage, tag: "v0.0.0", port: 8500, mounts: []string{"/data"}},
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

	for key := range services {
		serviceFlags(f, key)
	}

	f.Bool("force", false, "Force overwrite existing config file")
	f.BoolP("interactive", "i", false, "Interactive mode")

	f.String("adpg-password", "", "Set password for ADPG superuser. Required --adpg")

	f.String("adcm-db-host", "", "Set host for Postgres connection")
	f.Uint16("adcm-db-port", services["adpg"].port, "Set port for Postgres connection")
	f.String("adcm-db-name", "adcm", "Set database name for Postgres connection")
	f.String("adcm-db-user", "adcm", "Set user for Postgres connection")
	f.String("adcm-db-pass", "", "Set password for Postgres connection")

	f.String("adcm-pg-ssl-mode", postgresSSLMode, "Set SSL mode for Postgres connection")
	f.String("adcm-pg-ssl-ca", "", "Set path to CA (PEM) for Postgres connection")
	f.String("adcm-pg-ssl-crt", "", "Set path to certificate (PEM) for Postgres connection")
	f.String("adcm-pg-ssl-key", "", "Set path to key (PEM) for Postgres connection")

	f.String("vault-db-name", "vault", "Set database name for Postgres connection")
	f.String("vault-db-user", "vault", "Set user for Postgres connection")
	f.String("vault-db-pass", "", "Set password for Postgres connection")

	f.StringP("output", "o", "", "Output filename")
	f.StringP("name", "n", compose.DefaultProjectName, "Installation instance name")
}

func serviceFlags(f *pflag.FlagSet, key string) {
	svc := services[key]

	if key != "adcm" {
		f.Bool(key, false, fmt.Sprintf("Add %s to compose file", svc.name))
	}
	f.String(key+"-image", svc.image, fmt.Sprintf("Use specific %s image", svc.name))
	f.String(key+"-version", svc.tag, fmt.Sprintf("Use specific %s version", svc.name))

	port := svc.port
	if key == "adpg" {
		port = 0
	}
	f.Uint16(key+"-publish-port", port, "Use host port to connect to "+svc.name)
}

func validateRequiredFlag(cmd *cobra.Command, keyS, keyB string) error {
	if cmd.Flags().Changed(keyS) && !getBool(cmd, keyB) {
		return fmt.Errorf("--%s flag required for --%s", keyB, keyS)
	}
	return nil
}

func initProject(cmd *cobra.Command, _ []string) {
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

	name, _ := cmd.Flags().GetString("name")
	prj := &composeTypes.Project{
		Name:     name,
		Services: composeTypes.Services{},
	}

	var svcList []string
	for k := range services {
		svcList = append(svcList, k)
	}
	sort.Strings(svcList)

	enabledServices := defineServicesFromFlag(cmd, logger, svcList)
	svcList = []string{}
	for svcName, svc := range enabledServices {
		svcList = append(svcList, svcName)
		port := services[svcName].port
		var ports []composeTypes.ServicePortConfig
		if svc.port > 0 {
			ports = append(ports, composeTypes.ServicePortConfig{
				Mode:      "ingress",
				Target:    uint32(port),
				Published: strconv.FormatUint(uint64(svc.port), 10),
			})
		}
		prj.Services[svcName] = composeTypes.ServiceConfig{Image: svc.Image(), Ports: ports}
	}
	sort.Strings(svcList)

	xsecretsData := readSecretsFromFlag(cmd, logger)
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

	helpers := compose.NewModHelpers()

	uid := 10001
	for _, svcName := range svcList {
		hostname := prj.Name + "-" + svcName
		helpers = append(helpers,
			compose.CapDropAll(svcName),
			compose.Hostname(svcName, hostname),
			compose.RestartPolicy(svcName, composeTypes.RestartPolicyUnlessStopped),
			compose.PullPolicy(svcName, composeTypes.PullPolicyAlways),
		)

		if svcName != "adcm" {
			uids := strconv.Itoa(uid)
			uid++
			helpers = append(helpers,
				compose.ReadOnlyRootFilesystem(svcName),
				compose.SecurityOptsNoNewPrivileges(svcName),
				compose.User(svcName, uids, uids),
			)
		}

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

	dbHost, _ := cmd.Flags().GetString("adcm-db-host")
	if len(dbHost) == 0 {
		dbHost = "adpg"
	}
	port, _ := cmd.Flags().GetUint16("adcm-db-port")
	dbPort := strconv.FormatUint(uint64(port), 10)

	// TODO: user, healthcheck, read_only, security_opt
	helpers = append(helpers,
		compose.CapAdd("adcm", "CAP_CHOWN", "CAP_SETUID", "CAP_SETGID"), //FIXME: nginx run with root privileges
		compose.Environment("adcm",
			compose.Env{Name: "DB_HOST", Value: &dbHost},
			compose.Env{Name: "DB_PORT", Value: &dbPort},
		),
		compose.Labels("adcm", map[string]string{compose.ADAppTypeLabelKey: "adcm"}),
	)

	if _, ok := enabledServices["adpg"]; ok {
		helpers = append(helpers,
			compose.DependsOn("adcm", compose.Depended{Service: "adpg"}),
			compose.HealthCheck("adpg", compose.HealthCheckConfig{
				Cmd:      []string{"CMD-SHELL", "pg-entrypoint isready postgres"},
				Interval: 10 * time.Second,
				Timeout:  3 * time.Second,
				Retries:  3,
			}),
			compose.Labels("adpg", map[string]string{compose.ADAppTypeLabelKey: "adpg"}),
		)
	}

	if _, ok := enabledServices["consul"]; ok {
		// TODO: healthcheck
		//compose.HealthCheck("consul", compose.HealthCheckConfig{
		//	Cmd:         []string{"CMD-SHELL", "curl", "http://127.0.0.1:8500/v1/status/leader"},
		//	Interval:    3 * time.Second,
		//	Timeout:     5 * time.Second,
		//	Retries:     2,
		//	StartPeriod: 5 * time.Second,
		//}),
		helpers = append(helpers,
			compose.Labels("consul", map[string]string{compose.ADAppTypeLabelKey: "consul"}),
			compose.Command("consul", []string{"agent", "-dev", "-bind=0.0.0.0"}),
		)
	}

	if _, ok := enabledServices["vault"]; ok {
		helpers = append(helpers,
			compose.HealthCheck("consul", compose.HealthCheckConfig{
				Cmd:         []string{"CMD-SHELL", "wget", "-q", "-O", "-", "http://vault:8200/v1/sys/health"},
				Interval:    3 * time.Second,
				Timeout:     5 * time.Second,
				StartPeriod: 5 * time.Second,
			}),
			compose.Labels("vault", map[string]string{compose.ADAppTypeLabelKey: "vault"}),
		)
		if _, ok := enabledServices["adpg"]; ok {
			helpers = append(helpers,
				compose.DependsOn("vault", compose.Depended{Service: "adpg"}),
			)
		}
	}

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
	tmp, err = os.CreateTemp(".", "adcm.tmp")
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

func defineServicesFromFlag(cmd *cobra.Command, logger *log.Entry, svcList []string) map[string]service {
	interactive := getBool(cmd, "interactive")
	wrap := func(key, prompt string, secret, required bool) string {
		s, err := getStringValue(cmd, key, prompt, secret, required, interactive)
		if err != nil {
			logger.Fatal(err)
		}
		return s
	}

	out := map[string]service{}
	for _, key := range svcList {
		if key != "adcm" && !getBool(cmd, key) {
			continue
		}

		image := wrap(key+"-image", services[key].name+" image", false, true)
		tag := wrap(key+"-version", services[key].name+" version", false, true)
		port, err := getPortValue(cmd, key+"-publish-port", services[key].name+" publish port", false, interactive)
		if err != nil {
			logger.Fatal(err)
		}

		out[key] = service{image: image, tag: tag, port: port}
	}
	return out
}

func readSecretsFromFlag(cmd *cobra.Command, logger *log.Entry) map[string]string {
	out := map[string]string{}
	managedAdpg := getBool(cmd, "adpg")

	adcmInteractive := getBool(cmd, "interactive") || !managedAdpg
	wrap := func(key, prompt string, secret, required bool) {
		interactive := strings.HasPrefix(key, "adcm-") && adcmInteractive
		s, err := getStringValue(cmd, key, prompt, secret, required, interactive)
		if err != nil {
			logger.Fatal(err)
		}

		out[key] = s
	}

	if managedAdpg {
		wrap("adpg-password", "Postgres superuser password (random generated):", true, false)
	}

	wrap("adcm-db-host", "ADCM database hostname/ip:", false, true)
	port, err := getPortValue(cmd, "adcm-db-port", "ADCM database:", false, adcmInteractive)
	if err != nil {
		logger.Fatal(err)
	}
	out["adcm-db-port"] = strconv.FormatUint(uint64(port), 10)
	wrap("adcm-db-name", "ADCM database name:", false, true)
	wrap("adcm-db-user", "ADCM database user:", false, true)
	wrap("adcm-db-pass", "ADCM database password (random generated):", true, false)

	sslMode, err := getStringFromSelect(cmd, "adcm-pg-ssl-mode", "Select PostgreSQL SSL mode:", allowSSLModes, true)
	if err != nil {
		logger.Fatal(err)
	}

	if !managedAdpg && sslMode != postgresSSLMode {
		wrap("adcm-pg-ssl-ca", "ADCM database SSL CA file path:", false, false)
		wrap("adcm-pg-ssl-crt", "ADCM database SSL certificate file path:", false, false)
		wrap("adcm-pg-ssl-key", "ADCM database SSL private key file path:", false, false)
	}

	managedVault := getBool(cmd, "vault")
	if managedVault {
		wrap("vault-db-name", "Vault database name:", false, true)
		wrap("vault-db-user", "Vault database user:", false, true)
		wrap("vault-db-pass", "Vault database pass (random generated):", true, false)
	}

	return out
}

func getStringValue(cmd *cobra.Command, key, prompt string, secret, required, interactive bool) (string, error) {
	v, _ := cmd.Flags().GetString(key)
	v = strings.TrimSpace(v)

	ok := cmd.Flags().Changed(key)
	if ok {
		if len(v) == 0 && required {
			return "", &notSet{key}
		}
		return v, nil
	}

	if interactive {
		if err := question(prompt, v, secret, required, &v); err != nil {
			return "", err
		}
	}

	if secret && len(v) == 0 {
		v = utils.GenerateRandomString(16)
	}

	return v, nil
}

func getPortValue(cmd *cobra.Command, key, prompt string, required, interactive bool) (uint16, error) {
	v, _ := cmd.Flags().GetUint16(key)
	ok := cmd.Flags().Changed(key)
	if ok || !interactive {
		if v == 0 && required {
			return 0, &notSet{key}
		}

		return v, nil
	}

	var port uint16
	if err := question(prompt, strconv.FormatUint(uint64(v), 10), false, required, &port); err != nil {
		return 0, err
	}

	return port, nil
}

func getStringFromSelect(cmd *cobra.Command, key, prompt string, options []string, required bool) (string, error) {
	v, _ := cmd.Flags().GetString(key)
	ok := cmd.Flags().Changed(key)
	if ok || !getBool(cmd, "interactive") {
		if len(v) == 0 && required {
			return "", &notSet{key}
		}
		return v, nil
	}

	if err := survey.AskOne(
		&survey.Select{
			Message: prompt,
			Options: options,
			Default: v,
		},
		&v,
	); err != nil {
		return "", fmt.Errorf("%s, failed UserInput", err)
	}

	return v, nil
}

func question(prompt, def string, secret, required bool, v any) error {
	var promptType survey.Prompt
	if secret {
		promptType = &survey.Password{
			Message: prompt,
		}
	} else {
		promptType = &survey.Input{
			Message: prompt,
			Default: def,
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

type flag struct {
	v       pflag.Value
	changed bool
}

func getFlagsWithValues(cmd *cobra.Command, values map[string]flag) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		values[f.Name] = flag{f.Value, f.Changed}
	})
}
