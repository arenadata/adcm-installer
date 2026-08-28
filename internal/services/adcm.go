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
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/arenadata/adcm-installer/internal/services/helpers"
	"github.com/arenadata/adcm-installer/pkg/compose"
	"github.com/arenadata/adcm-installer/pkg/types"
	"github.com/arenadata/adcm-installer/pkg/utils"

	"github.com/AlecAivazis/survey/v2"
	"github.com/Masterminds/semver/v3"
	composeTypes "github.com/compose-spec/compose-go/v2/types"
	log "github.com/sirupsen/logrus"
)

type AdcmConfig struct {
	Count          uint8  `yaml:"adcm-count"`
	WorkerCount    *uint8 `yaml:"adcm-worker-count"`
	DBHost         string `yaml:"adcm-db-host"`
	DBPort         uint16 `yaml:"adcm-db-port"`
	DBName         string `yaml:"adcm-db-name"`
	DBUser         string `yaml:"adcm-db-user"`
	DBPassword     string `yaml:"adcm-db-pass"`
	DBSSLMode      string `yaml:"adcm-db-ssl-mode"`
	DBSSLCaFile    string `yaml:"adcm-db-ssl-ca-file"`
	DBSSLCertFile  string `yaml:"adcm-db-ssl-cert-file"`
	DBSSLKeyFile   string `yaml:"adcm-db-ssl-key-file"`
	SSLKeyFile     string `yaml:"adcm-ssl-key-file"`
	SSLCertFile    string `yaml:"adcm-ssl-cert-file"`
	Image          string `yaml:"adcm-image"`
	Tag            string `yaml:"adcm-tag"`
	PublishPort    uint16 `yaml:"adcm-publish-port"`
	PublishSSLPort uint16 `yaml:"adcm-publish-ssl-port"`
	Url            string `yaml:"adcm-url"`
	Volume         string `yaml:"adcm-volume"`

	VaultUrl            string `yaml:"adcm-vault-url"`
	VaultTokenFile      string `yaml:"adcm-vault-token-file"`
	VaultMountPoint     string `yaml:"adcm-vault-mount-point"`
	VaultCaFile         string `yaml:"adcm-vault-ca-file"`
	VaultClientCertFile string `yaml:"adcm-vault-client-cert-file"`
	VaultClientKeyFile  string `yaml:"adcm-vault-client-key-file"`

	ip string
}

// adcmInstances settles the configuration the ADCM instances have in common
// and then generates them.
func (prj *Project) adcmInstances() {
	prj.adcmSharedConfig()

	if prj.config.Adcm.Count > 1 {
		for i := uint8(1); i <= prj.config.Adcm.Count; i++ {
			prj.adcm(fmt.Sprintf("%s-%d", AdcmName, i), i-1)
		}
		prj.adcmSerializeMigrations()
		return
	}

	prj.adcm(AdcmName, 0)
}

// adcmPrimary names the ADCM instance the others follow: it runs the database
// migrations alone and owns the credentials the whole installation shares.
func (prj *Project) adcmPrimary() string {
	if prj.config.Adcm.Count > 1 {
		return fmt.Sprintf("%s-1", AdcmName)
	}

	return AdcmName
}

// adcmSerializeMigrations makes every ADCM instance but the first one wait for it.
// First instance runs them alone and the rest join an already migrated database.
func (prj *Project) adcmSerializeMigrations() {
	first := prj.adcmPrimary()

	// a started container has not necessarily finished migrating yet, so the
	// health check is what the rest wait on wherever the release serves one
	condition := composeTypes.ServiceConditionStarted
	if prj.adcmHealthCheck {
		condition = composeTypes.ServiceConditionHealthy
	}

	for i := uint8(2); i <= prj.config.Adcm.Count; i++ {
		prj.AppendHelpers(helpers.DependsOn(fmt.Sprintf("%s-%d", AdcmName, i),
			helpers.Depended{Service: first, Condition: condition, Required: true}))
	}
}

// resolveAdcmTag settles the ADCM image tag: an unset one is the latest release
// the registry offers, and the built-in default whenever the registry cannot be
// reached.
func (prj *Project) resolveAdcmTag() {
	config := &prj.config.Adcm
	if len(config.Tag) > 0 {
		return
	}

	tag, err := LatestReleasedTag(config.Image)
	if err != nil {
		log.Warnf("cannot resolve the latest released ADCM version from %q (%v), "+
			"falling back to %s", config.Image, err, ADCMTag)
		config.Tag = ADCMTag
		return
	}

	config.Tag = tag
}

// adcmSharedConfig reads the settings every ADCM instance has in common.
func (prj *Project) adcmSharedConfig() {
	config := &prj.config.Adcm

	if len(config.Volume) == 0 {
		config.Volume = prj.hostname(AdcmName)
	}

	if prj.interactive {
		checkErr(readValue(&config.Image, &prompt{msg: "ADCM image:", def: config.Image}))
		checkErr(readValue(&config.Tag, &prompt{msg: "ADCM image tag:", def: config.Tag}))
	}

	managedADPG := prj.config.Adpg.enable
	if prj.interactive || !managedADPG {
		if !managedADPG {
			checkErr(readValue(&config.DBHost, &prompt{msg: "ADCM database host:"}, survey.Required))

			portStr := strconv.Itoa(int(config.DBPort))
			checkErr(readValue(&config.DBPort, &prompt{msg: "ADCM database port:", def: portStr}))
		}

		checkErr(readValue(&config.DBName, &prompt{msg: "ADCM database name:", def: config.DBName}))
		checkErr(readValue(&config.DBUser, &prompt{msg: "ADCM database user:", def: config.DBUser}))

		passwdPrompt := &prompt{msg: "ADCM database password:", secret: true}
		if managedADPG {
			passwdPrompt.help = "If not set, a random password will be generated"
			checkErr(readValue(&config.DBPassword, passwdPrompt))
		} else {
			checkErr(readValue(&config.DBPassword, passwdPrompt, survey.Required))

			sslPrompt := &prompt{msg: "Select Postgres SSL mode:", def: config.DBSSLMode, opts: allowSSLModes}
			checkErr(readValue(&config.DBSSLMode, sslPrompt, survey.Required))

			if config.DBSSLMode != pgSslModeDisable {
				checkErr(readValue(&config.DBSSLCaFile,
					&prompt{msg: "ADCM database SSL CA file path:"}, fileExists))
				checkErr(readValue(&config.DBSSLCertFile,
					&prompt{msg: "ADCM database SSL certificate file path:"}, fileExists))
				checkErr(readValue(&config.DBSSLKeyFile,
					&prompt{msg: "ADCM database SSL private key file path:"}, fileExists))
			}
		}
	}

	if len(config.DBPassword) == 0 {
		config.DBPassword = utils.GenerateRandomString(16)
	}

	if prj.interactive {
		checkErr(readValue(&config.Volume,
			&prompt{msg: "ADCM volume name or path:", def: config.Volume}))

		p := &prompt{msg: "ADCM SSL Private Key file path:",
			help: "Leave blank if you do not enable HTTPS"}
		checkErr(readValue(&config.SSLKeyFile, p, fileExists))
		if len(config.SSLKeyFile) > 0 {
			checkErr(readValue(&config.SSLCertFile,
				&prompt{msg: "ADCM SSL Certificate file path:"}, fileExists))
		}
	}

	prj.adcmHealthCheck = adcmHealthCheckSupported(config.Tag)

	prj.adcmSharedVaultConfig()
}

// adcmSharedVaultConfig settles whether the ADCM instances keep their secrets
// in Vault and, if they do, which KV v2 mount point they share.
func (prj *Project) adcmSharedVaultConfig() {
	config := &prj.config.Adcm

	prj.adcmVaultStorage = prj.config.SecretStorage == SecretStorageVault && adcmVaultSupported(config.Tag)
	if !prj.adcmVaultStorage {
		return
	}

	if len(config.VaultMountPoint) == 0 {
		config.VaultMountPoint = AdcmName
	}

	if prj.interactive {
		checkErr(readValue(&config.VaultMountPoint,
			&prompt{msg: "ADCM Vault mount point:", def: config.VaultMountPoint,
				help: "KV v2 secrets engine mount point shared by the ADCM instances"}, survey.Required))
	}
}

func (prj *Project) adcm(name string, index uint8) {
	config := prj.config.Adcm
	config.PublishPort += uint16(index)
	config.PublishSSLPort += uint16(index)
	if index > 0 || len(config.Url) == 0 {
		config.Url = fmt.Sprintf("http://%s:%d", config.ip, config.PublishPort)
	}

	addService(name, prj.prj)

	hostname := prj.hostname(name)
	managedADPG := prj.config.Adpg.enable

	if prj.interactive {
		checkErr(readValue(&config.PublishPort,
			&prompt{msg: fmt.Sprintf("%s: ADCM publish port:", name),
				def: strconv.Itoa(int(config.PublishPort))}))

		config.Url = fmt.Sprintf("http://%s:%d", config.ip, config.PublishPort)
		checkErr(readValue(&config.Url,
			&prompt{msg: fmt.Sprintf("%s: ADCM url:", name), def: config.Url}))

		if len(config.SSLKeyFile) > 0 {
			checkErr(readValue(&config.PublishSSLPort,
				&prompt{msg: fmt.Sprintf("%s: ADCM publish SSL port:", name),
					def: strconv.Itoa(int(config.PublishSSLPort))}))
		}
	}

	if len(config.SSLKeyFile) > 0 {
		prj.AppendHelpers(
			helpers.Secrets(name,
				helpers.Secret{
					Source:   PemKey,
					Target:   path.Join(ADCMMountPath, "conf/ssl/key.pem"),
					FileMode: 0o400,
				},
				helpers.Secret{
					Source:   PemCert,
					Target:   path.Join(ADCMMountPath, "conf/ssl/cert.pem"),
					FileMode: 0o440,
				},
			),
			helpers.PublishPort(name, config.PublishSSLPort, ADCMPublishSSLPort),
		)
	}

	if managedADPG {
		prj.AppendHelpers(
			helpers.DependsOn(name,
				helpers.Depended{
					Service:  AdpgName,
					Required: true,
				}),
		)
	} else {
		portStr := strconv.Itoa(int(config.DBPort))
		prj.AppendHelpers(
			helpers.Environment(name,
				helpers.Env{Name: "DB_HOST", Value: &config.DBHost},
				helpers.Env{Name: "DB_PORT", Value: &portStr},
			))
	}

	xsecretsData := map[string]string{
		PgDbName: config.DBName,
		PgDbUser: config.DBUser,
		PgDbPass: config.DBPassword,
	}

	if config.DBSSLMode != pgSslModeDisable {
		sslOpts := types.DbSSLOptions{SSLMode: config.DBSSLMode}

		if len(config.DBSSLCaFile) > 0 {
			b, err := os.ReadFile(config.DBSSLCaFile)
			checkErr(err)
			xsecretsData[PgSslCaKey] = string(b)

			target := path.Join(helpers.SecretsPath, PgSslCaKey)
			sslOpts.SSLRootCert = target
			prj.AppendHelpers(
				helpers.Secrets(name, helpers.Secret{
					Source:   PgSslCaKey,
					Target:   target,
					FileMode: 0o440,
				}),
			)
		}
		if len(config.DBSSLCertFile) > 0 {
			b, err := os.ReadFile(config.DBSSLCertFile)
			checkErr(err)
			xsecretsData[PgSslCertKey] = string(b)

			target := path.Join(helpers.SecretsPath, PgSslCertKey)
			sslOpts.SSLCert = target
			prj.AppendHelpers(
				helpers.Secrets(name, helpers.Secret{
					Source:   PgSslCertKey,
					Target:   target,
					FileMode: 0o440,
				}),
			)
		}
		if len(config.DBSSLKeyFile) > 0 {
			b, err := os.ReadFile(config.DBSSLKeyFile)
			checkErr(err)
			xsecretsData[PgSslKeyKey] = string(b)
			target := path.Join(helpers.SecretsPath, PgSslKeyKey)
			sslOpts.SSLKey = target
			prj.AppendHelpers(
				helpers.Secrets(name, helpers.Secret{
					Source:   PgSslKeyKey,
					Target:   target,
					FileMode: 0o400,
				}),
			)
		}

		optStr := sslOpts.String()
		prj.AppendHelpers(helpers.Environment(name, helpers.Env{Name: "DB_OPTIONS", Value: &optStr}))
	}

	if len(config.SSLKeyFile) > 0 {
		b, err := os.ReadFile(config.SSLKeyFile)
		checkErr(err)
		xsecretsData[PemKey] = string(b)
	}
	if len(config.SSLCertFile) > 0 {
		b, err := os.ReadFile(config.SSLCertFile)
		checkErr(err)
		xsecretsData[PemCert] = string(b)
	}

	prj.adcmSecretStorage(name, &config, xsecretsData)

	labels := map[string]string{compose.ADAppTypeLabelKey: AdcmName}
	primary := prj.adcmPrimary()
	if name != primary {
		// the instances share one database, one secret storage and one data
		// volume, so they share the credentials as well: they are kept once, on
		// the primary, and the label is what leads apply to them
		labels[compose.ADAppAdcmLabelKey] = primary
	}

	prj.AppendHelpers(
		helpers.Hostname(name, hostname),
		helpers.Labels(name, labels),
		helpers.Image(name, config.Image+":"+config.Tag),
		helpers.Environment(name, helpers.Env{Name: AdcmUrlEnv, Value: &config.Url}),
		helpers.Volumes(name, config.Volume+":"+ADCMMountPath),
	)

	if name == primary {
		xsecretsDataEncrypted := xsecretsData
		if prj.crypt != nil {
			var err error
			for k, v := range xsecretsData {
				v, err = prj.crypt.EncryptValue(v)
				checkErr(err)
				xsecretsDataEncrypted[k] = v
			}
		}

		prj.AppendHelpers(helpers.Extension(name, XSecretsKey, &XSecrets{Data: xsecretsDataEncrypted}))
	}

	if prj.adcmHealthCheck {
		// ADCM always serves its API over plain HTTP inside the container, even
		// when HTTPS is published as well, so the probe stays on the HTTP port.
		healthCheckCommand := fmt.Sprintf("wget -q -O - http://127.0.0.1:%d%s",
			ADCMPublishPort, ADCMLivenessProbePath)
		prj.AppendHelpers(
			helpers.HealthCheck(name, helpers.HealthCheckConfig{
				Cmd:      []string{"CMD-SHELL", healthCheckCommand},
				Interval: 10 * time.Second,
				Timeout:  5 * time.Second,
				Retries:  5,
				// ADCM runs the database migrations before it serves anything,
				// so give it room to start without being reported unhealthy
				StartPeriod: 30 * time.Second,
			}),
		)
	}

	prj.adcmConsul(name, config.Url)

	if prj.config.JobExecEnv == JobExecEnvCelery {
		// ADCM hands the jobs to its workers, which inherit the variable
		// together with the rest of the ADCM environment
		prj.AppendHelpers(helpers.Environment(name,
			helpers.Env{Name: JobExecEnvEnv, Value: utils.Ptr(JobExecEnvCelery)}))
	}

	if adcmUnprivilegedSupported(config.Tag) {
		// ADCM 3.0+ runs unprivileged, so harden it like the other services
		// (read-only root and tmpfs are added at apply time, where the host OS
		// and the image-resolved user are known).
		prj.AppendHelpers(helpers.SecurityOptsNoNewPrivileges(name))
	} else {
		prj.AppendHelpers(helpers.CapAdd(name, "CAP_CHOWN", "CAP_SETUID", "CAP_SETGID"))
	}

	if config.PublishPort > 0 {
		prj.AppendHelpers(helpers.PublishPort(name, config.PublishPort, ADCMPublishPort))
	}
}

func (prj *Project) adcmConsul(name, adcmUrl string) {
	if !prj.config.Consul.enable {
		return
	}

	if !urlHasHostPort(adcmUrl) {
		log.Warnf("%s: %s is %q, which has no scheme, host and port; ADCM does not start "+
			"with Consul configured unless all three are set", name, AdcmUrlEnv, adcmUrl)
	}

	consulUrl := fmt.Sprintf("http://%s:%d", ConsulName, ConsulPublishPort)
	prj.AppendHelpers(
		helpers.Environment(name, helpers.Env{Name: ConsulUrlEnv, Value: &consulUrl}),
		helpers.DependsOn(name, helpers.Depended{
			Service:   ConsulName,
			Condition: composeTypes.ServiceConditionStarted,
			Required:  true,
		}),
	)
}

func urlHasHostPort(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}

	return len(u.Scheme) > 0 && len(u.Hostname()) > 0 && len(u.Port()) > 0
}

func (prj *Project) adcmSecretStorage(name string, config *AdcmConfig, xsecretsData map[string]string) {
	if !prj.adcmVaultStorage {
		prj.AppendHelpers(helpers.Environment(name,
			helpers.Env{Name: SecretBackendEnv, Value: utils.Ptr(SecretBackendEnvFileSystem)}))
		return
	}

	tokenTarget := path.Join(helpers.SecretsPath, VaultTokenKey)
	prj.AppendHelpers(
		helpers.Environment(name,
			helpers.Env{Name: SecretBackendEnv, Value: utils.Ptr(SecretBackendEnvVault)},
			helpers.Env{Name: VaultMountPointEnv, Value: &config.VaultMountPoint},
			helpers.Env{Name: VaultTokenFileEnv, Value: &tokenTarget},
		),
		helpers.Secrets(name, helpers.Secret{
			Source:   VaultTokenKey,
			Target:   tokenTarget,
			FileMode: 0o400,
		}),
	)

	if prj.config.VaultType == VaultTypeEmbedded {
		prj.adcmEmbeddedVault(name, xsecretsData)
		return
	}
	prj.adcmExternalVault(name, config, xsecretsData)
}

// adcmVaultSupported reports whether the ADCM release identified by tag can
// keep its secrets in Vault.
func adcmVaultSupported(tag string) bool {
	v, err := semver.NewVersion(tag)
	if err != nil {
		// Non-semver rolling tags (e.g. "develop") track the current release, which keeps its secrets in Vault.
		log.Debugf("ADCM tag %q is not a semver version; assuming the Vault secret "+
			"storage is supported (ADCM %s or newer)", tag, ADCMVaultMinTag)
		return true
	}

	if v.LessThan(semver.MustParse(ADCMVaultMinTag)) {
		log.Warnf("ADCM %s does not support the Vault secret storage (requires %s or newer), "+
			"falling back to the FileSystem secret storage", tag, ADCMVaultMinTag)
		return false
	}

	return true
}

// adcmHealthCheckSupported reports whether the ADCM release identified by tag
// serves the liveness probe used by the container health check.
func adcmHealthCheckSupported(tag string) bool {
	v, err := semver.NewVersion(tag)
	if err != nil {
		// Non-semver rolling tags (e.g. "develop") track the current release, which serves the liveness probe.
		log.Debugf("ADCM tag %q is not a semver version; assuming the liveness "+
			"probe is served (ADCM %s or newer)", tag, ADCMHealthCheckMinTag)
		return true
	}

	if v.LessThan(semver.MustParse(ADCMHealthCheckMinTag)) {
		log.Warnf("ADCM %s does not serve the liveness probe (requires %s or newer), "+
			"the container health check is disabled", tag, ADCMHealthCheckMinTag)
		return false
	}

	return true
}

// adcmUnprivilegedSupported reports whether the ADCM release identified by tag
// runs as its own unprivileged user rather than as root.
func adcmUnprivilegedSupported(tag string) bool {
	v, err := semver.NewVersion(tag)
	if err != nil {
		// Non-semver rolling tags (e.g. "develop") track the current release, which runs under the unprivileged user.
		log.Debugf("ADCM tag %q is not a semver version; assuming unprivileged-user "+
			"support (ADCM %s or newer)", tag, ADCMUnprivilegedMinTag)
		return true
	}

	return !v.LessThan(semver.MustParse(ADCMUnprivilegedMinTag))
}

func (prj *Project) adcmEmbeddedVault(name string, xsecretsData map[string]string) {
	vaultConfig := prj.config.Vault

	scheme := "http"
	if len(vaultConfig.SSLKeyFile) > 0 {
		scheme = "https"

		if len(vaultConfig.SSLCertFile) > 0 {
			b, err := os.ReadFile(vaultConfig.SSLCertFile)
			checkErr(err)
			xsecretsData[VaultCaKey] = string(b)

			caTarget := path.Join(helpers.SecretsPath, VaultCaKey)
			prj.AppendHelpers(
				helpers.Environment(name, helpers.Env{Name: VaultCaFileEnv, Value: utils.Ptr(caTarget)}),
				helpers.Secrets(name, helpers.Secret{
					Source:   VaultCaKey,
					Target:   caTarget,
					FileMode: 0o440,
				}),
			)
		}
	}

	vaultUrl := fmt.Sprintf("%s://%s:%d", scheme, VaultName, VaultPublishPort)
	prj.AppendHelpers(
		helpers.Environment(name, helpers.Env{Name: VaultUrlEnv, Value: &vaultUrl}),
		helpers.DependsOn(name, helpers.Depended{Service: VaultName, Required: true}),
	)
}

func (prj *Project) adcmExternalVault(name string, config *AdcmConfig, xsecretsData map[string]string) {
	prj.AppendHelpers(helpers.Environment(name, helpers.Env{Name: VaultUrlEnv, Value: &config.VaultUrl}))

	b, err := os.ReadFile(config.VaultTokenFile)
	checkErr(err)
	xsecretsData[VaultTokenKey] = strings.TrimSpace(string(b))

	tlsFiles := []struct {
		path     string
		key      string
		env      string
		fileMode int64
	}{
		{path: config.VaultCaFile, key: VaultCaKey, env: VaultCaFileEnv, fileMode: 0o440},
		{path: config.VaultClientCertFile, key: VaultClientCertKey, env: VaultClientCertFileEnv, fileMode: 0o440},
		{path: config.VaultClientKeyFile, key: VaultClientKeyKey, env: VaultClientKeyFileEnv, fileMode: 0o400},
	}
	for _, f := range tlsFiles {
		if len(f.path) == 0 {
			continue
		}

		b, err := os.ReadFile(f.path)
		checkErr(err)
		xsecretsData[f.key] = string(b)

		target := path.Join(helpers.SecretsPath, f.key)
		prj.AppendHelpers(
			helpers.Environment(name, helpers.Env{Name: f.env, Value: utils.Ptr(target)}),
			helpers.Secrets(name, helpers.Secret{Source: f.key, Target: target, FileMode: f.fileMode}),
		)
	}
}
