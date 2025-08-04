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
	"os"
	"sort"
	"strconv"

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

	AdcmName   = "adcm"
	AdpgName   = "adpg"
	ConsulName = "consul"
	VaultName  = "vault"
	PauseName  = "pause"

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
)

type XSecrets struct {
	AgeRecipient string            `yaml:"age_recipient,omitempty" mapstructure:"age_recipient,omitempty"`
	Key          string            `yaml:"key,omitempty" mapstructure:"key,omitempty"`
	Data         map[string]string `yaml:"data,omitempty" mapstructure:"data,omitempty"`
	UnMapped     map[string]string `yaml:"un-mapped,omitempty" mapstructure:"un-mapped,omitempty"`
}

type InitConfig struct {
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
	if prj.interactive {
		adcmCount := strconv.Itoa(int(prj.config.Adcm.Count))
		checkErr(readValue(&prj.config.Adcm.Count, &prompt{msg: "Number of ADCM instances", def: adcmCount}))
	}

	if prj.config.Adcm.Count > 1 {
		for i := uint8(1); i <= prj.config.Adcm.Count; i++ {
			prj.adcm(fmt.Sprintf("adcm-%d", i))
		}
	} else {
		prj.adcm("")
	}

	prj.consul()
	prj.adpg()
	prj.vault()

	for name := range prj.prj.Services {
		prj.AppendHelpers(sharedHelpers(name)...)
	}

	return prj.ApplyHelpers()
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
	if len(config.Adcm.Tag) == 0 {
		config.Adcm.Tag = ADCMTag
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
