package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/arenadata/arenadata-installer/apis/app/v1alpha1"
	"github.com/arenadata/arenadata-installer/internal/services"
	"github.com/arenadata/arenadata-installer/pkg/compose"
	"github.com/arenadata/arenadata-installer/pkg/interactive"
	"github.com/arenadata/arenadata-installer/pkg/secrets"
	"github.com/arenadata/arenadata-installer/pkg/utils"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

const (
	postgresSSLMode = "disable"
)

var initCmd = &cobra.Command{
	Use:   "init <name>",
	Short: "Initialize a new configuration",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(1)(cmd, args); err != nil {
			return err
		}

		if cmd.Flags().Changed("pg-password") && !getBool(cmd, "pg") {
			return fmt.Errorf("--pg flag required for --pg-password")
		}
		return nil
	},
	Run: initProject,
}

func init() {
	rootCmd.AddCommand(initCmd)

	ageKeyFlags(initCmd, "age-key", ageKeyFileName)

	f := initCmd.Flags()
	f.Bool("adcm", false, "ADCM manifest initialization")
	f.Bool("pg", false, "Postgres manifest initialization")
	initCmd.MarkFlagsOneRequired("adcm", "pg")

	f.Bool("force", false, "Force overwrite existing config file")
	f.BoolP("interactive", "i", false, "Interactive mode (set sensitive data)")

	flagWithMutuallyExclusiveInteractive(f, "adcm-db-host", "", "Set host for Postgres connection")
	flagWithMutuallyExclusiveInteractive(f, "adcm-db-port", compose.PostgresPort, "Set port for Postgres connection")
	flagWithMutuallyExclusiveInteractive(f, "adcm-db-name", "adcm", "Set database name for Postgres connection")
	flagWithMutuallyExclusiveInteractive(f, "adcm-db-user", "adcm", "Set user for Postgres connection")
	flagWithMutuallyExclusiveInteractive(f, "adcm-db-password", "", "Set password for Postgres connection")

	f.String("adcm-pg-ssl-mode", postgresSSLMode, "Set SSL mode for Postgres connection")
	flagWithMutuallyExclusiveInteractive(f, "adcm-pg-ssl-ca", "", "Set path to CA (PEM) for Postgres connection")
	flagWithMutuallyExclusiveInteractive(f, "adcm-pg-ssl-crt", "", "Set path to certificate (PEM) for Postgres connection")
	flagWithMutuallyExclusiveInteractive(f, "adcm-pg-ssl-key", "", "Set path to key (PEM) for Postgres connection")
	initCmd.MarkFlagsRequiredTogether("adcm-pg-ssl-mode", "adcm-pg-ssl-ca", "adcm-pg-ssl-crt", "adcm-pg-ssl-key")

	flagWithMutuallyExclusiveInteractive(f, "pg-password", "", "Set password for postgres user. Required --pg")

	f.StringP("output", "o", appFilename, "Manifest output filename")
	f.StringP("namespace", "n", compose.DefaultNamespace, "Namespace name")
}

func flagWithMutuallyExclusiveInteractive(f *pflag.FlagSet, name string, value any, usage string) {
	switch v := value.(type) {
	case string:
		f.String(name, v, usage)
	case uint16:
		f.Uint16(name, v, usage)
	}

	initCmd.MarkFlagsMutuallyExclusive("interactive", name)
}

func initProject(cmd *cobra.Command, args []string) {
	logger := log.WithField("command", "init")

	force, _ := cmd.Flags().GetBool("force")
	outputPath, _ := cmd.Flags().GetString("output")
	if ok, err := utils.FileExists(outputPath); err != nil {
		logger.Fatal(err)
	} else if ok && !force {
		logger.Fatalf("config file %q already exists", outputPath)
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

	output := os.Stdout
	if len(outputPath) > 0 && outputPath != "-" {
		output, err = os.OpenFile(outputPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0664)
		if err != nil {
			logger.Fatal(err)
		}
		defer func() {
			if e := output.Close(); e != nil {
				logger.Fatal(e)
			}
			if err != nil {
				logger.Fatal(err)
			}
		}()
	}

	var out []*v1alpha1.Application
	for _, key := range []string{"adcm", "pg"} {
		if ok := getBool(cmd, key); ok {
			app, err := appendApplication(cmd, args[0], key, crypt)
			if err != nil {
				logger.Errorf("config %s initialization failed: %v", key, err)
				return
			}
			out = append(out, app)
		}
	}

	enc := yaml.NewEncoder(output)
	defer func() {
		if e := enc.Close(); e != nil {
			err = e
		}
	}()

	enc.SetIndent(2)
	for _, app := range out {
		if err = enc.Encode(app); err != nil {
			logger.Error(err)
			return
		}
	}
}

func appendApplication(cmd *cobra.Command, name, key string, crypt *secrets.AgeCrypt) (*v1alpha1.Application, error) {
	var kind string
	switch key {
	case "adcm":
		kind = "ADCM"
	case "app":
		kind = "Application"
	case "pg":
		kind = "Postgres"
	default:
		return nil, fmt.Errorf("unknown kind %q", kind)
	}

	ns, _ := cmd.Flags().GetString("namespace")
	app, err := v1alpha1.New(kind, ns, name)
	if err != nil {
		return nil, fmt.Errorf("initialization %s failed: %v", kind, err)
	}

	if kind == "ADCM" {
		if getBool(cmd, "pg") {
			app.Annotations[v1alpha1.DependsOnKey] = "postgres." + name
		}

		if err = initAdcm(app, cmd, crypt); err != nil {
			return nil, err
		}
	}

	if kind == "Postgres" {
		if err = initPg(app, cmd, crypt); err != nil {
			return nil, err
		}
	}

	return app, nil
}

func initAdcm(app *v1alpha1.Application, cmd *cobra.Command, crypt *secrets.AgeCrypt) error {
	dbHost, _ := cmd.Flags().GetString("adcm-db-host")
	dbPort, _ := cmd.Flags().GetUint16("adcm-db-port")
	dbName, _ := cmd.Flags().GetString("adcm-db-name")
	dbUser, _ := cmd.Flags().GetString("adcm-db-user")
	dbPassword, _ := cmd.Flags().GetString("adcm-db-password")
	if len(dbPassword) == 0 {
		dbPassword = utils.GenerateRandomString(15)
	}

	dbSslMode, _ := cmd.Flags().GetString("adcm-pg-ssl-mode")
	dbSslCa, _ := cmd.Flags().GetString("adcm-pg-ssl-ca")
	dbSslCrt, _ := cmd.Flags().GetString("adcm-pg-ssl-crt")
	dbSslKey, _ := cmd.Flags().GetString("adcm-pg-ssl-key")

	sec := secrets.Secrets{
		Files: make(map[string]*secrets.File),
		Env:   make(map[string]string),
	}

	if getBool(cmd, "interactive") {
		act := interactive.Actions{}

		r := bufio.NewReader(os.Stdin)

		if !getBool(cmd, "pg") {
			act = append(act, interactive.NewAction(
				interactive.String(r),
				"Enter host for Postgres connection",
				dbHost,
				&dbHost,
			))

			act = append(act, interactive.NewAction(
				interactive.String(r),
				"Enter port for Postgres connection",
				dbPort,
				&dbPort,
			))
		}

		act = append(act, interactive.NewAction(
			interactive.String(r),
			"Enter database name for Postgres connection",
			dbName,
			&dbName,
		))

		act = append(act, interactive.NewAction(
			interactive.String(r),
			"Enter user for Postgres connection",
			dbUser,
			&dbUser,
		))

		act = append(act, interactive.NewAction(
			interactive.Password,
			"Enter password for Postgres connection (will be generated if not set)",
			"",
			&dbPassword,
		))

		if len(dbSslMode) > 0 && dbSslMode != postgresSSLMode {
			act = append(act, interactive.NewAction(
				interactive.String(r),
				"Enter path to CA (PEM) file for Postgres connection",
				"",
				&dbSslCa,
			))

			act = append(act, interactive.NewAction(
				interactive.String(r),
				"Enter path to certificate (PEM) file for Postgres connection",
				"",
				&dbSslCrt,
			))

			act = append(act, interactive.NewAction(
				interactive.String(r),
				"Enter path to key (PEM) file for Postgres connection",
				"",
				&dbSslKey,
			))
		}

		if err := act.Run(); err != nil {
			return err
		}
	}

	app.Spec.Env["DB_HOST"] = &dbHost
	if dbPort != compose.PostgresPort {
		val := strconv.Itoa(int(dbPort))
		app.Spec.Env["DB_PORT"] = &val
	}
	app.Spec.Env["DB_NAME"] = &dbName

	sec.Files[compose.PostgresUserFilename] = &secrets.File{
		EnvKey: utils.Ptr("DB_USER_FILE"),
		Data:   dbUser,
	}
	sec.Env["DB_USER"] = dbUser

	sec.Files[compose.PostgresPasswordFilename] = &secrets.File{
		EnvKey: utils.Ptr("DB_PASS_FILE"),
		Data:   dbPassword,
	}
	sec.Env["DB_PASS"] = dbPassword

	if len(dbSslMode) > 0 && dbSslMode != postgresSSLMode {
		const (
			crt = "tls.crt"
			key = "tls.key"
			ca  = "ca.crt"
		)

		b, err := os.ReadFile(dbSslCa)
		if err != nil {
			return err
		}
		sec.Files[ca] = &secrets.File{
			Data: string(b),
		}

		b, err = os.ReadFile(dbSslCrt)
		if err != nil {
			return err
		}
		sec.Files[crt] = &secrets.File{
			Data: string(b),
		}

		b, err = os.ReadFile(dbSslKey)
		if err != nil {
			return err
		}
		sec.Files[key] = &secrets.File{
			Data: string(b),
		}

		dbSslOptions := services.DbSSLOptions{
			SSLMode:     dbSslMode,
			SSLCert:     compose.SecretsPath + crt,
			SSLKey:      compose.SecretsPath + key,
			SSLRootCert: compose.SecretsPath + ca,
		}

		app.Spec.Env["DB_OPTIONS"] = utils.Ptr(dbSslOptions.String())
	}

	return injectSecretToAnnotations(app, sec, crypt)
}

func initPg(app *v1alpha1.Application, cmd *cobra.Command, crypt *secrets.AgeCrypt) error {
	passwd, _ := cmd.Flags().GetString("pg-password")
	if len(passwd) == 0 {
		passwd = utils.GenerateRandomString(15)
	}

	if getBool(cmd, "interactive") {
		act := interactive.Actions{}

		act = append(act, interactive.NewAction(
			interactive.Password,
			"Enter Postgres Password (will be generated if not set)",
			"",
			&passwd,
		))

		if err := act.Run(); err != nil {
			return err
		}
	}

	sec := secrets.Secrets{
		Files: map[string]*secrets.File{
			"postgres-passwd": {
				EnvKey: utils.Ptr("POSTGRES_PASSWORD_FILE"),
				Data:   passwd,
			},
		},
	}

	return injectSecretToAnnotations(app, sec, crypt)
}

func injectSecretToAnnotations(app *v1alpha1.Application, sec secrets.Secrets, crypt *secrets.AgeCrypt) error {
	b, err := json.Marshal(sec)
	if err != nil {
		return err
	}

	encData, err := crypt.Encrypt(string(b))
	if err != nil {
		return err
	}

	app.Annotations[v1alpha1.SecretsAgeRecipientKey] = crypt.Recipient().String()
	app.Annotations[v1alpha1.SecretsAgeKey] = encData

	return nil
}
