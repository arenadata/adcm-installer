package compose

const (
	DefaultPlatform    = "linux/amd64"
	DefaultNetworkName = "app-arenadata"
	DefaultNamespace   = "app-arenadata"

	ADImageRegistry = "hub.arenadata.io"
	ADLabel         = "io.arenadata.app"

	SecretsPath = "/run/secrets/"
	ConfigsPath = "/run/configs/"

	PostgresVolumeTarget        = "/var/lib/postgresql/data"
	PostgresPort         uint16 = 5432
	PostgresImageName           = "postgres"
	PostgresImageTag            = "16-alpine"

	PostgresUser             = "postgres"
	PostgresUserFilename     = "postgres-user"
	PostgresPasswordFilename = "postgres-passwd"
	PostgresDbNameFilename   = "postgres-db-name"

	ADCMVolumeTarget        = "/adcm/data"
	ADCMPort         uint16 = 8000
	ADCMImageName           = "adcm/adcm"
	ADCMImageTag            = "2.5.0"

	PostgresHelperSQLScript = `CREATE OR REPLACE FUNCTION create_role_if_not_exists(name TEXT, password TEXT) RETURNS void AS $$
BEGIN
   IF NOT EXISTS (SELECT FROM pg_roles WHERE pg_roles.rolname = name) THEN
      EXECUTE format('CREATE ROLE %I WITH LOGIN PASSWORD %L', name, password);
   END IF;
END;
$$ LANGUAGE plpgsql;
`
)
