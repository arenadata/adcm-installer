package compose

const (
	DefaultPlatform    = "linux/amd64"
	DefaultProjectName = "app-arenadata"

	ADLabel           = "app.arenadata.io"
	ADAppTypeLabelKey = ADLabel + "/type"

	SecretsPath = "/run/secrets"

	ADCMImage   = "hub.arenadata.io/adcm/adcm"
	ADPGImage   = "mybackspace/adpg"
	VaultImage  = "openbao/openbao"
	ConsulImage = "mybackspace/yellow-pages"
)
