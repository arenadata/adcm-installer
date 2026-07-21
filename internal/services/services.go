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

package services

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/arenadata/adcm-installer/internal/services/helpers"
	"github.com/arenadata/adcm-installer/pkg/compose"
	"github.com/arenadata/adcm-installer/pkg/secrets"
	"github.com/arenadata/adcm-installer/pkg/utils"

	"github.com/AlecAivazis/survey/v2"
	composeTypes "github.com/compose-spec/compose-go/v2/types"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

const (
	PgDbName = "db-name"
	PgDbUser = "db-user"
	PgDbPass = "db-pass"

	pgSslModeDisable    = "disable"
	pgSslModeAllow      = "allow"
	pgSslModePrefer     = "prefer"
	pgSslModeRequire    = "require"
	pgSslModeVerifyCA   = "verify-ca"
	pgSslModeVerifyFull = "verify-full"

	PgSslCaKey   = "pg-ssl-ca"
	PgSslCertKey = "pg-ssl-cert"
	PgSslKeyKey  = "pg-ssl-key"

	VaultDeployModeNonHa = "non-ha"
	VaultDeployModeHa    = "ha"
	VaultDeployModeDev   = "dev"
	VaultUnsealData      = "unseal-data"

	SecretStorageFileSystem = "FileSystem"
	SecretStorageVault      = "Vault"

	JobExecEnvLocal  = "local"
	JobExecEnvCelery = "celery"

	VaultTypeEmbedded = "embedded"
	VaultTypeExternal = "external"

	VaultTokenKey      = "vault-token"
	VaultCaKey         = "vault-ca"
	VaultClientCertKey = "vault-client-cert"
	VaultClientKeyKey  = "vault-client-key"

	AdcmUrlEnv                 = "DEFAULT_ADCM_URL"
	JobExecEnvEnv              = "SCHEDULER_JOB_EXECUTION_ENVIRONMENT"
	SecretBackendEnv           = "SECRET_BACKEND"
	SecretBackendEnvFileSystem = "FileSystemBackend"
	SecretBackendEnvVault      = "VaultBackend"
	ConsulUrlEnv               = "CONSUL_URL"
	VaultUrlEnv                = "VAULT_URL"
	VaultMountPointEnv         = "VAULT_MOUNT_POINT"
	VaultTokenFileEnv          = "VAULT_TOKEN_FILE"
	VaultCaFileEnv             = "VAULT_CA_FILE"
	VaultClientCertFileEnv     = "VAULT_CLIENT_CERT_FILE"
	VaultClientKeyFileEnv      = "VAULT_CLIENT_KEY_FILE"

	AdcmName   = "adcm"
	AdpgName   = "adpg"
	ConsulName = "consul"
	VaultName  = "vault"
	PauseName  = "pause"
	WorkerName = "worker"

	ConfigJson = "config.json"
	PemKey     = "key.pem"
	PemCert    = "cert.pem"
	PemCa      = "ca.pem"

	XSecretsKey = "x-secrets"

	InitContainerProfile    = "init"
	PrimaryContainerProfile = "primary"
)

var (
	allowSSLModes = []string{
		pgSslModeDisable,
		pgSslModeAllow,
		pgSslModePrefer,
		pgSslModeRequire,
		pgSslModeVerifyCA,
		pgSslModeVerifyFull,
	}

	allowVaultDeploymentModes = []string{
		VaultDeployModeNonHa,
		VaultDeployModeHa,
		VaultDeployModeDev,
	}

	allowSecretStorages = []string{
		SecretStorageFileSystem,
		SecretStorageVault,
	}

	allowVaultTypes = []string{
		VaultTypeEmbedded,
		VaultTypeExternal,
	}

	allowJobExecEnvs = []string{
		JobExecEnvLocal,
		JobExecEnvCelery,
	}
)

type XSecrets struct {
	AgeRecipient string            `yaml:"age_recipient,omitempty" mapstructure:"age_recipient,omitempty"`
	Key          string            `yaml:"key,omitempty" mapstructure:"key,omitempty"`
	Data         map[string]string `yaml:"data,omitempty" mapstructure:"data,omitempty"`
	UnMapped     map[string]string `yaml:"un-mapped,omitempty" mapstructure:"un-mapped,omitempty"`
}

type InitConfig struct {
	SecretStorage string `yaml:"secret-storage"`
	VaultType     string `yaml:"vault-type"`
	JobExecEnv    string `yaml:"job-execution-environment"`

	Adcm   AdcmConfig   `yaml:",inline"`
	Adpg   AdpgConfig   `yaml:",inline"`
	Consul ConsulConfig `yaml:",inline"`
	Vault  VaultConfig  `yaml:",inline"`
}

type Project struct {
	servicesModHelpers helpers.ModHelpers
	prj                *composeTypes.Project
	config             *InitConfig
	interactive        bool
	crypt              secrets.Secrets

	// adcmVaultStorage records whether the ADCM instances keep their secrets
	// in Vault, settled once for the whole installation
	adcmVaultStorage bool

	// adcmHealthCheck records whether the ADCM release serves the liveness
	// probe, settled once for the whole installation
	adcmHealthCheck bool
}

func New(name string, opts ...ProjectOption) (*Project, error) {
	p := &Project{
		servicesModHelpers: helpers.NewModHelpers(),
		config:             &InitConfig{},
		prj: &composeTypes.Project{
			Name:     name,
			Services: make(composeTypes.Services),
		},
	}

	for _, opt := range opts {
		if err := opt(p); err != nil {
			return nil, err
		}
	}

	setDefaults(p.config)

	return p, nil
}

func (prj *Project) Build() error {
	// settled here rather than in setDefaults: it reads the registry, and
	// constructing a Project must not depend on the network
	prj.resolveAdcmTag()

	if prj.interactive {
		adcmCount := strconv.Itoa(int(prj.config.Adcm.Count))
		checkErr(readValue(&prj.config.Adcm.Count, &prompt{msg: "Number of ADCM instances", def: adcmCount}))
	}

	if err := prj.resolveSecretStorage(); err != nil {
		return err
	}

	if err := prj.resolveJobExecEnv(); err != nil {
		return err
	}

	prj.consul()
	prj.adpg()
	prj.vault()

	prj.adcmInstances()

	for name := range prj.prj.Services {
		prj.AppendHelpers(sharedHelpers(name)...)
	}

	if err := prj.ApplyHelpers(); err != nil {
		return err
	}

	// a worker is a copy of the finished ADCM service -- it takes its image,
	// environment, volumes and secrets -- so it is built once the helpers above
	// have been applied, and writes into the project directly rather than
	// appending helpers of its own
	prj.workerInstances()

	return nil
}

// resolveJobExecEnv settles how ADCM runs its jobs and, with it, how many worker
// services are generated: local runs the jobs inside ADCM and needs none, celery
// hands them to standalone Celery workers.
func (prj *Project) resolveJobExecEnv() error {
	config := prj.config

	if len(config.JobExecEnv) > 0 && !slices.Contains(allowJobExecEnvs, config.JobExecEnv) {
		return fmt.Errorf("unknown job-execution-environment %q, allowed values: %s",
			config.JobExecEnv, strings.Join(allowJobExecEnvs, ", "))
	}

	if prj.interactive && len(config.JobExecEnv) == 0 {
		envPrompt := &prompt{
			msg:  "Select Job Execution Environment:",
			def:  JobExecEnvLocal,
			opts: allowJobExecEnvs,
			help: "local runs jobs inside ADCM, celery runs them in standalone ADCM worker services",
		}
		checkErr(readValue(&config.JobExecEnv, envPrompt))
	}

	if len(config.JobExecEnv) == 0 {
		// an explicitly requested worker count is a request for celery
		config.JobExecEnv = JobExecEnvLocal
		if config.Adcm.WorkerCount != nil && *config.Adcm.WorkerCount > 0 {
			config.JobExecEnv = JobExecEnvCelery
		}
	}

	if config.JobExecEnv != JobExecEnvCelery {
		if config.Adcm.WorkerCount != nil && *config.Adcm.WorkerCount > 0 {
			return fmt.Errorf("adcm-worker-count is only allowed when job-execution-environment is %q",
				JobExecEnvCelery)
		}

		config.Adcm.WorkerCount = utils.Ptr(uint8(0))
		return nil
	}

	if config.Adcm.WorkerCount == nil {
		config.Adcm.WorkerCount = utils.Ptr(uint8(1))
	}

	if prj.interactive {
		workerCount := *config.Adcm.WorkerCount
		checkErr(readValue(&workerCount, &prompt{
			msg: "Number of ADCM worker instances:",
			def: strconv.Itoa(int(workerCount)),
		}))
		config.Adcm.WorkerCount = &workerCount
	}

	if *config.Adcm.WorkerCount == 0 {
		return fmt.Errorf("adcm-worker-count must be greater than 0 when job-execution-environment is %q",
			JobExecEnvCelery)
	}

	return nil
}

func (prj *Project) resolveSecretStorage() error {
	config := prj.config

	if len(config.SecretStorage) > 0 && !slices.Contains(allowSecretStorages, config.SecretStorage) {
		return fmt.Errorf("unknown secret-storage %q, allowed values: %s",
			config.SecretStorage, strings.Join(allowSecretStorages, ", "))
	}
	if len(config.VaultType) > 0 && !slices.Contains(allowVaultTypes, config.VaultType) {
		return fmt.Errorf("unknown vault-type %q, allowed values: %s",
			config.VaultType, strings.Join(allowVaultTypes, ", "))
	}

	vaultRequested := config.Vault.enable

	if prj.interactive {
		if vaultRequested {
			config.SecretStorage = SecretStorageVault
		} else if len(config.SecretStorage) == 0 {
			storagePrompt := &prompt{
				msg:  "Select Secret Storage:",
				def:  SecretStorageFileSystem,
				opts: allowSecretStorages,
			}
			checkErr(readValue(&config.SecretStorage, storagePrompt))
		}

		if config.SecretStorage == SecretStorageVault && len(config.VaultType) == 0 {
			typePrompt := &prompt{
				msg:  "Select Vault Secret Storage type:",
				def:  VaultTypeEmbedded,
				opts: allowVaultTypes,
				help: "embedded runs a managed OpenBao service next to ADCM, external uses an existing Vault/OpenBao server",
			}
			checkErr(readValue(&config.VaultType, typePrompt))
		}
	} else if len(config.SecretStorage) == 0 {
		config.SecretStorage = SecretStorageFileSystem
		if vaultRequested {
			config.SecretStorage = SecretStorageVault
		}
	}

	if config.SecretStorage != SecretStorageVault {
		return nil
	}

	if len(config.VaultType) == 0 {
		config.VaultType = VaultTypeEmbedded
	}

	switch config.VaultType {
	case VaultTypeEmbedded:
		config.Vault.enable = true
	case VaultTypeExternal:
		config.Vault.enable = false
		return prj.externalVaultSettings()
	}

	return nil
}

func (prj *Project) externalVaultSettings() error {
	config := &prj.config.Adcm

	if prj.interactive {
		checkErr(readValue(&config.VaultUrl,
			&prompt{msg: "Vault url:", def: config.VaultUrl, help: "e.g. https://vault.example.com:8200"},
			validVaultUrl))
		checkErr(readValue(&config.VaultTokenFile,
			&prompt{msg: "Vault token file path:",
				help: "Path to a local file with the Vault token that ADCM will use"},
			survey.ComposeValidators(survey.Required, fileExists)))
		checkErr(readValue(&config.VaultCaFile,
			&prompt{msg: "Vault CA file path:",
				help: "Leave blank if the Vault certificate is signed by a well-known CA or TLS is not used"},
			fileExists))
		checkErr(readValue(&config.VaultClientCertFile,
			&prompt{msg: "Vault client certificate file path:",
				help: "Leave blank if Vault does not require client TLS authentication"},
			fileExists))
		if len(config.VaultClientCertFile) > 0 {
			checkErr(readValue(&config.VaultClientKeyFile,
				&prompt{msg: "Vault client private key file path:"},
				survey.ComposeValidators(survey.Required, fileExists)))
		}
	}

	if err := validVaultUrl(config.VaultUrl); err != nil {
		return fmt.Errorf("adcm-vault-url: %v", err)
	}
	if len(config.VaultTokenFile) == 0 {
		return fmt.Errorf("adcm-vault-token-file is required when vault-type is %q", VaultTypeExternal)
	}
	if err := fileExists(config.VaultTokenFile); err != nil {
		return fmt.Errorf("adcm-vault-token-file: %v", err)
	}
	if (len(config.VaultClientCertFile) > 0) != (len(config.VaultClientKeyFile) > 0) {
		return fmt.Errorf("adcm-vault-client-cert-file and adcm-vault-client-key-file must be specified together")
	}

	return nil
}

func validVaultUrl(val interface{}) error {
	rawUrl, _ := val.(string)
	if len(rawUrl) == 0 {
		return fmt.Errorf("url is required")
	}

	u, err := url.Parse(rawUrl)
	if err != nil {
		return fmt.Errorf("invalid url: %v", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || len(u.Host) == 0 {
		return fmt.Errorf("url must be in http(s)://host[:port] format")
	}
	return nil
}

func (prj *Project) AppendHelpers(hlp ...helpers.ModHelper) {
	prj.servicesModHelpers = append(prj.servicesModHelpers, hlp...)
}

func (prj *Project) ApplyHelpers() error {
	return prj.servicesModHelpers.Apply(prj.prj)
}

func (prj *Project) ToYaml(w io.Writer) (err error) {
	enc := yaml.NewEncoder(w)
	defer func() {
		if e := enc.Close(); e != nil {
			err = e
		}
	}()

	enc.SetIndent(2)
	return enc.Encode(prj.prj)
}

func (prj *Project) hostname(name string) string {
	return prj.prj.Name + "-" + name
}

type service struct {
	Name string
	Type string
}

func (prj *Project) Services() []service {
	var out []service
	for k, v := range prj.prj.Services {
		out = append(out, service{
			Name: k,
			Type: v.Labels[compose.ADAppTypeLabelKey],
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func readValue(v any, prompt *prompt, validator ...survey.Validator) error {
	if len(prompt.opts) > 0 {
		return survey.AskOne(prompt.prompt(), v)
	}

	q := &survey.Question{
		Prompt: prompt.prompt(),
	}

	if len(validator) > 0 {
		q.Validate = validator[0]
	}

	return survey.Ask([]*survey.Question{q}, v)
}

type prompt struct {
	msg  string
	opts []string
	def  string
	help string

	secret bool
}

func (p prompt) prompt() survey.Prompt {
	if len(p.opts) > 0 {
		return &survey.Select{
			Message: p.msg,
			Options: p.opts,
			Default: p.def,
			Help:    p.help,
		}
	}

	if p.secret {
		return &survey.Password{Message: p.msg, Help: p.help}
	}

	return &survey.Input{Message: p.msg, Default: p.def, Help: p.help}
}

func addService(name string, prj *composeTypes.Project) {
	if _, ok := prj.Services[name]; ok {
		log.Fatalf("service %s has already been added", name)
	}
	prj.Services[name] = composeTypes.ServiceConfig{Name: name}
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func fileExists(val interface{}) error {
	file := val.(string)
	if len(file) == 0 {
		return nil
	}

	fi, err := os.Stat(file)
	if err != nil {
		return fmt.Errorf("file cannot be read")
	}

	if fi.IsDir() {
		return fmt.Errorf("path cannot be a directory")
	}

	return nil
}

func setDefaults(config *InitConfig) {
	if config.Adcm.Count < 1 {
		config.Adcm.Count = 1
	}
	// Adcm.WorkerCount stays nil here: "not configured" is resolved from the
	// job execution environment in resolveJobExecEnv
	if config.Adcm.DBPort == 0 {
		config.Adcm.DBPort = ADPGPublishPort
	}
	if len(config.Adcm.DBName) == 0 {
		config.Adcm.DBName = "adcm"
	}
	if len(config.Adcm.DBUser) == 0 {
		config.Adcm.DBUser = "adcm"
	}
	if len(config.Adcm.DBSSLMode) == 0 {
		config.Adcm.DBSSLMode = pgSslModeDisable
	}
	if config.Adcm.PublishPort == 0 {
		config.Adcm.PublishPort = ADCMPublishPort
	}
	if config.Adcm.PublishSSLPort == 0 {
		config.Adcm.PublishSSLPort = ADCMPublishSSLPort
	}
	if len(config.Adcm.Image) == 0 {
		config.Adcm.Image = ADCMImage
	}
	if hostIp := utils.HostIp(); len(hostIp) > 0 {
		config.Adcm.ip = hostIp
	}
	if len(config.Adcm.Url) == 0 && config.Adcm.Count == 1 {
		config.Adcm.Url = fmt.Sprintf("http://%s:%d", config.Adcm.ip, config.Adcm.PublishPort)
	}

	if len(config.Adpg.Image) == 0 {
		config.Adpg.Image = ADPGImage
	}
	if len(config.Adpg.Tag) == 0 {
		config.Adpg.Tag = ADPGTag
	}

	if len(config.Consul.Image) == 0 {
		config.Consul.Image = ConsulImage
	}
	if len(config.Consul.Tag) == 0 {
		config.Consul.Tag = ConsulTag
	}
	if config.Consul.PublishPort == 0 {
		config.Consul.PublishPort = ConsulPublishPort
	}

	if len(config.Vault.Image) == 0 {
		config.Vault.Image = VaultImage
	}
	if len(config.Vault.Tag) == 0 {
		config.Vault.Tag = VaultTag
	}
	if config.Vault.PublishPort == 0 {
		config.Vault.PublishPort = VaultPublishPort
	}
	if len(config.Vault.Mode) == 0 {
		config.Vault.Mode = VaultDeployModeNonHa
	}
	if len(config.Vault.DBSSLMode) == 0 {
		config.Vault.DBSSLMode = pgSslModeDisable
	}
	if len(config.Vault.DBName) == 0 {
		config.Vault.DBName = "vault"
	}
	if len(config.Vault.DBUser) == 0 {
		config.Vault.DBUser = "vault"
	}
	if config.Vault.UI == nil {
		config.Vault.UI = utils.Ptr(true)
	}
}

type ProjectOption func(*Project) error

func WithCrypt(crypt secrets.Secrets) ProjectOption {
	return func(p *Project) error {
		p.crypt = crypt
		return nil
	}
}

func WithInteractive(b bool) ProjectOption {
	return func(p *Project) error {
		p.interactive = b
		return nil
	}
}

func WithAdpg(b bool) ProjectOption {
	return func(p *Project) error {
		p.config.Adpg.enable = b
		return nil
	}
}

func WithConsul(b bool) ProjectOption {
	return func(p *Project) error {
		p.config.Consul.enable = b
		return nil
	}
}

func WithVault(b bool) ProjectOption {
	return func(p *Project) error {
		p.config.Vault.enable = b
		return nil
	}
}

func WithAdcmCount(n uint8) ProjectOption {
	return func(p *Project) error {
		if n > p.config.Adcm.Count {
			p.config.Adcm.Count = n
		}
		return nil
	}
}

func WithAdcmWorkerCount(n uint8) ProjectOption {
	return func(p *Project) error {
		p.config.Adcm.WorkerCount = &n
		return nil
	}
}

func WithConfigFile(file string) ProjectOption {
	return func(p *Project) error {
		if len(file) == 0 {
			return nil
		}

		return valuesFromConfigFile(file, p.config)
	}
}

func valuesFromConfigFile(file string, config *InitConfig) error {
	fi, err := os.Open(file)
	if err != nil {
		return err
	}
	defer func() { _ = fi.Close() }()

	dec := yaml.NewDecoder(fi)
	dec.KnownFields(true)
	return dec.Decode(&config)
}

func sharedHelpers(svcName string) helpers.ModHelpers {
	return helpers.ModHelpers{
		helpers.CapDropAll(svcName),
		helpers.RestartPolicy(svcName, composeTypes.RestartPolicyUnlessStopped),
		//compose.PullPolicy(svcName, composeTypes.PullPolicyAlways),
		helpers.Network(svcName, compose.DefaultNetwork, nil),
	}
}

func AdcmFamilyService(appType string) bool {
	return appType == AdcmName || appType == WorkerName
}
