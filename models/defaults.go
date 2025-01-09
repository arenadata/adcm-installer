package models

import (
	"reflect"

	"github.com/arenadata/adcm-installer/utils"

	"gopkg.in/yaml.v3"
)

const (
	ProjectName = "default"

	ADCMConfigFile = "adcm.yaml"
	AGEKeyFile     = "age.key"

	DefaultImageTag = "latest"

	ADLabel                 = "io.arenadata"
	ADImageRegistry         = "hub.arenadata.io"
	ADCMServiceName         = "adcm"
	ADCMImageName           = "adcm/adcm"
	ADCMImageTag            = "2.4"
	ADCMVolumeName          = "adcm"
	ADCMVolumeTarget        = "/adcm/data"
	ADCMPublishPort  uint16 = 8000
	ADCMPortPattern         = "%d:8000"

	PostgresServiceName  = "postgres"
	PostgresImageName    = "postgres"
	PostgresImageTag     = "16-alpine"
	PostgresInstall      = true
	PostgresHost         = "adcm-postgres"
	PostgresPort         = 5432
	PostgresSSLMode      = "disable"
	PostgresDatabase     = "adcm"
	PostgresLogin        = "adcm"
	PostgresVolumeName   = "adcm-postgres"
	PostgresVolumeTarget = "/var/lib/postgresql/data"
)

func FullConfigWithComments(sec *Secrets) *yaml.Node {
	conf := &Config{
		Registry: utils.Ptr(""),
		ADCM: &ADCMConfig{
			Volume: utils.Ptr(ADCMVolumeName + ":" + ADCMVolumeTarget + ":Z"),
		},
		Postgres: &PostgresConfig{
			Image: &Image{
				Registry: utils.Ptr(""),
			},
			Connection: &PostgresConnectionConfig{
				SSL: &PostgresSSLConfig{
					SSLMode: PostgresSSLMode,
				},
			},
			Volume: utils.Ptr(PostgresVolumeName + ":" + PostgresVolumeTarget + ":Z"),
		},
		Secrets: sec,
	}

	SetDefaultsConfig(conf)
	yamlNode, err := setConfigComments(conf)
	if err != nil {
		panic(err)
	}

	return yamlNode
}

func SetDefaultsConfig(in *Config) {
	if in.Project == nil {
		in.Project = utils.Ptr(ProjectName)
	}

	if in.ADCM == nil {
		in.ADCM = &ADCMConfig{}
	}

	SetDefaultsADCMConfig(in.ADCM, in.Registry)

	if in.Postgres == nil {
		in.Postgres = &PostgresConfig{}
	}

	if in.Postgres.Install == nil {
		in.Postgres.Install = utils.Ptr(PostgresInstall)
	}

	if in.Postgres.Image == nil {
		in.Postgres.Image = &Image{}
	}

	if !utils.PtrIsEmpty(in.Registry) && utils.PtrIsEmpty(in.Postgres.Image.Registry) {
		in.Postgres.Image.Registry = in.Registry
	}

	if utils.PtrIsEmpty(in.Postgres.Image.Name) {
		in.Postgres.Image.Name = utils.Ptr(PostgresImageName)
	}

	if utils.PtrIsEmpty(in.Postgres.Image.Tag) {
		in.Postgres.Image.Tag = utils.Ptr(PostgresImageTag)
	}

	if in.Postgres.Connection == nil {
		in.Postgres.Connection = &PostgresConnectionConfig{}
	}

	if utils.PtrIsEmpty(in.Postgres.Connection.Host) {
		in.Postgres.Connection.Host = utils.Ptr(PostgresHost)
	}

	if utils.PtrIsEmpty(in.Postgres.Connection.Port) {
		in.Postgres.Connection.Port = utils.Ptr(PostgresPort)
	}

	if utils.PtrIsEmpty(in.Postgres.Connection.Database) {
		in.Postgres.Connection.Database = utils.Ptr(PostgresDatabase)
	}
}

func SetDefaultsADCMConfig(in *ADCMConfig, registry *string) {
	if in.Image == nil {
		in.Image = &Image{}
	}

	if utils.PtrIsEmpty(in.Image.Registry) {
		if utils.PtrIsEmpty(registry) {
			in.Image.Registry = utils.Ptr(ADImageRegistry)
		} else {
			in.Image.Registry = registry
		}
	}

	if utils.PtrIsEmpty(in.Image.Name) {
		in.Image.Name = utils.Ptr(ADCMImageName)
	}

	if utils.PtrIsEmpty(in.Image.Tag) {
		in.Image.Tag = utils.Ptr(ADCMImageTag)
	}

	if utils.PtrIsEmpty(in.Publish) {
		in.Publish = utils.Ptr(ADCMPublishPort)
	}

}

func SetDefaultSecrets(in *SensitiveData) {
	if len(in.Postgres.Login) == 0 {
		in.Postgres.Login = PostgresLogin
	}

	if len(in.Postgres.Password) == 0 {
		in.Postgres.Password = utils.GenerateRandomString(15)
	}
}

func setConfigComments(conf *Config) (*yaml.Node, error) {
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
