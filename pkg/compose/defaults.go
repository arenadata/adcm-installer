package compose

const (
	DefaultPlatform    = "linux/amd64"
	DefaultProjectName = "app-arenadata"

	ADLabel           = "app.arenadata.io"
	ADAppTypeLabelKey = ADLabel + "/type"

	SecretsPath = "/run/csecrets"

	ADCMImage   = "hub.arenadata.io/adcm/adcm"
	ADPGImage   = "hub.arenadata.io/adcm/postgres"
	VaultImage  = "openbao/openbao"
	ConsulImage = "hub.arenadata.io/adcm/consul"
)
