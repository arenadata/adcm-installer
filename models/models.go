package models

import (
	"path"
	"reflect"
	"strings"

	"github.com/arenadata/adcm-installer/crypt"

	"gopkg.in/yaml.v3"
)

// Config sets the installer configuration
type Config struct {
	// Project set specific project name.
	Project *string `json:"project,omitempty" yaml:"project,omitempty" doc:"Set project name."`
	// Registry set specific global image registry.
	Registry *string `json:"registry,omitempty" yaml:"registry,omitempty" doc:"Set specific image registry."`
	// ADCM provides ADCM configuration options.
	ADCM *ADCMConfig `json:"adcm,omitempty" yaml:"adcm,omitempty" doc:"Provides ADCM configuration options."`
	// Postgres Provides PostgreSQL specific configuration options.
	Postgres *PostgresConfig `json:"postgres,omitempty" yaml:"postgres,omitempty" doc:"Provides PostgreSQL specific configuration options."`
	// Secrets provides sensitive data.
	Secrets *Secrets `json:"-" yaml:"secrets,omitempty" doc:"-"`
}

type ADCMConfig struct {
	// Image provides ADCM image specific options.
	Image *Image `json:"image,omitempty" yaml:"image,omitempty" doc:"Provides ADCM image specific options."`
	// Publish ADCM port
	Publish *uint16 `json:"publish,omitempty" yaml:"publish,omitempty" doc:"Publish ADCM port."`
	// Volume persistent ADCM storage name or path.
	Volume *string `json:"volume,omitempty" yaml:"volume,omitempty" doc:"Persistent ADCM storage name or path."`
}

type PostgresConfig struct {
	// Install run PostgreSQL in Docker.
	Install *bool `json:"install,omitempty" yaml:"install,omitempty" doc:"Install PostgreSQL for ADCM."`
	// Image provides PostgreSQL image specific options.
	Image *Image `json:"image,omitempty" yaml:"image,omitempty" doc:"Provides PostgreSQL image specific options."`
	// Connection provides PostgreSQL connection options. Ignored with postgres.install=true.
	Connection *PostgresConnectionConfig `json:"connection,omitempty" yaml:"connection" doc:"Provides PostgreSQL connection options. Ignored with postgres.install=true."`
	// Volume persistent PostgreSQL storage name or path.
	Volume *string `json:"volume,omitempty" yaml:"volume,omitempty" doc:"Persistent PostgreSQL storage name or path."`
}

type PostgresConnectionConfig struct {
	// Host set specific PostgreSQL host.
	Host *string `json:"host,omitempty" yaml:"host,omitempty" doc:"PostgreSQL host."`
	// Port set specific PostgreSQL port.
	Port *int `json:"port,omitempty" yaml:"port,omitempty" doc:"PostgreSQL port."`
	// Database set specific PostgreSQL database name.
	Database *string `json:"database,omitempty" yaml:"database,omitempty" doc:"PostgreSQL database name."`
	// SSL PostgreSQL SSL settings. Ignored with postgres.install=true.
	SSL *PostgresSSLConfig `json:"ssl,omitempty" yaml:"ssl,omitempty" doc:"PostgreSQL SSL settings. Ignored with postgres.install=true."`
}

type PostgresSSLConfig struct {
	// SSLMode PostgreSQL SSL mode.
	SSLMode string `json:"ssl-mode" yaml:"sslMode" doc:"PostgreSQL SSL mode."`
	// SSLCert PostgreSQL SSL cert.
	SSLCert string `json:"ssl-cert" yaml:"sslCert" doc:"PostgreSQL SSL cert."`
	// SSLKey PostgreSQL SSL key.
	SSLKey string `json:"ssl-key" yaml:"sslKey" doc:"PostgreSQL SSL key."`
	// SSLRootCert PostgreSQL SSL root cert.
	SSLRootCert string `json:"ssl-root-cert" yaml:"sslRootCert" doc:"PostgreSQL SSL root cert."`
}

type Image struct {
	// Registry image registry to use.
	Registry *string `json:"registry,omitempty" yaml:"registry,omitempty" doc:"Image registry to use."`
	// Name image name to use.
	Name *string `json:"name" yaml:"name" doc:"Image name to use."`
	// Tag image tag to use.
	Tag *string `json:"tag,omitempty" yaml:"tag,omitempty" doc:"Image tag to use."`
}

func (i Image) String() string {
	var image string
	if i.Registry != nil && len(*i.Registry) > 0 {
		reg := path.Clean(*i.Registry)
		if reg[0] == '/' {
			reg = reg[1:]
		}
		if reg[0] == '.' {
			reg = ""
		}
		if len(reg) > 0 {
			image = reg + "/"
		}
	}

	imageName := path.Clean(*i.Name)
	if imageName[0] == '/' {
		imageName = imageName[1:]
	}
	if imageName[0] == '.' {
		return ""
	}

	image += imageName

	tag := DefaultImageTag
	if i.Tag != nil && len(*i.Tag) > 0 {
		tag = *i.Tag
	}

	tagSep := ":"
	if strings.HasPrefix(tag, "sha") {
		tagSep = "@"
	}

	return image + tagSep + tag
}

// Secrets sets secrets for applications
type Secrets struct {
	Recipient     string         `json:"recipient" yaml:"recipient"`
	SensitiveData *SensitiveData `json:"enc" yaml:"enc"`
}

func NewSecrets(e *crypt.AgeCrypt) *Secrets {
	return &Secrets{
		Recipient: e.Recipient().String(),
		SensitiveData: &SensitiveData{
			encDec:   e,
			Postgres: &Credentials{},
		},
	}
}

type SensitiveData struct {
	encDec   *crypt.AgeCrypt
	Postgres *Credentials `json:"postgres" yaml:"postgres"`
}

type Credentials struct {
	Login    string `json:"login" yaml:"login"`
	Password string `json:"password" yaml:"password"`
}

func (sd *SensitiveData) MarshalYAML() (any, error) {
	type _SD SensitiveData

	b, err := yaml.Marshal(_SD(*sd))
	if err != nil {
		return nil, err
	}

	return sd.encDec.Encrypt(string(b))
}

func (sd *SensitiveData) UnmarshalYAML(unmarshal func(any) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}

	decData, err := sd.encDec.Decrypt(s)
	if err != nil {
		return err
	}

	type _SD SensitiveData

	var iSD _SD
	if err = yaml.Unmarshal([]byte(decData), &iSD); err != nil {
		return err
	}

	*sd = SensitiveData(iSD)

	return nil
}

func SetConfigComments(conf *Config) (*yaml.Node, error) {
	node := new(yaml.Node)
	if err := node.Encode(conf); err != nil {
		return nil, err
	}

	comments(reflect.ValueOf(conf), node)

	return node, nil
}

func comments(in reflect.Value, out *yaml.Node) {
	if in.Kind() == reflect.Ptr {
		in = in.Elem()
	}
	t := in.Type()

	for i := 0; i < in.NumField(); i++ {
		field := t.Field(i)
		docTag := field.Tag.Get("doc")
		if docTag == "-" || len(docTag) == 0 {
			continue
		}
		out.Content[i*2].HeadComment = docTag

		if field.Type.Kind() == reflect.Ptr {
			v := in.Field(i).Elem()
			if !v.IsValid() {
				continue
			}
			if v.Type().Kind() == reflect.Struct {
				comments(v, out.Content[i*2+1])
			}
		}
	}
}
