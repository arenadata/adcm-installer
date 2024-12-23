package models

import (
	"github.com/arenadata/adcm-installer/utils"
)

const (
	ADCMConfigFile = "adcm.yaml"
	AGEKeyFile     = "age.key"

	ADImageRegistry  = "hub.arenadata.io"
	ADCMServiceName  = "adcm"
	ADCMImageName    = "adcm/adcm"
	ADCMImageTag     = "2.4"
	ADCMVolumeName   = "adcm"
	ADCMVolumeTarget = "/adcm/data"

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

func FullConfigWithDefaults() *Config {
	return &Config{
		Registry: utils.Ptr(""),
		ADCM: &ADCMConfig{
			Image: &Image{
				Registry: utils.Ptr(ADImageRegistry),
				Name:     utils.Ptr(ADCMImageName),
				Tag:      utils.Ptr(ADCMImageTag),
			},
			Volume: utils.Ptr(ADCMVolumeName + ":" + ADCMVolumeTarget),
		},
		Postgres: &PostgresConfig{
			Install: utils.Ptr(PostgresInstall),
			Image: &Image{
				Registry: utils.Ptr(""),
				Name:     utils.Ptr(PostgresImageName),
				Tag:      utils.Ptr(PostgresImageTag),
			},
			Connection: &PostgresConnectionConfig{
				Host:     utils.Ptr(PostgresHost),
				Port:     utils.Ptr(PostgresPort),
				Database: utils.Ptr(PostgresDatabase),
				SSL: &PostgresSSLConfig{
					SSLMode: PostgresSSLMode,
				},
			},
			Volume: utils.Ptr(PostgresVolumeName + ":" + PostgresVolumeTarget),
		},
	}
}

func SetDefaultsConfig(in *Config) {
	if in.ADCM == nil {
		in.ADCM = &ADCMConfig{}
	}

	if in.ADCM.Image == nil {
		in.ADCM.Image = &Image{}
	}

	if utils.PtrIsEmpty(in.ADCM.Image.Registry) {
		if utils.PtrIsEmpty(in.Registry) {
			in.ADCM.Image.Registry = utils.Ptr(ADImageRegistry)
		} else {
			in.ADCM.Image.Registry = in.Registry
		}
	}

	if utils.PtrIsEmpty(in.ADCM.Image.Name) {
		in.ADCM.Image.Name = utils.Ptr(ADCMImageName)
	}

	if utils.PtrIsEmpty(in.ADCM.Image.Tag) {
		in.ADCM.Image.Tag = utils.Ptr(ADCMImageTag)
	}

	if utils.PtrIsEmpty(in.ADCM.Volume) {
		in.ADCM.Volume = utils.Ptr(ADCMVolumeName + ":" + ADCMVolumeTarget)
	}

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

	if utils.PtrIsEmpty(in.Postgres.Volume) {
		in.Postgres.Volume = utils.Ptr(PostgresVolumeName + ":" + PostgresVolumeTarget)
	}
}

func SetDefaultSecrets(in *Secrets) {
	if in.Postgres == nil {
		in.Postgres = &Credentials{}
	}

	if len(in.Postgres.Login) == 0 {
		in.Postgres.Login = PostgresLogin
	}

	if len(in.Postgres.Password) == 0 {
		in.Postgres.Password = utils.GenerateRandomString(15)
	}
}
