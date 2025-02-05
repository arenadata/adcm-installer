package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/arenadata/arenadata-installer/internal/api/meta"
	"github.com/arenadata/arenadata-installer/internal/runtime"
	"github.com/arenadata/arenadata-installer/pkg/compose"
	"github.com/arenadata/arenadata-installer/pkg/secrets"

	composeTypes "github.com/compose-spec/compose-go/v2/types"
	composev2 "github.com/docker/compose/v2/pkg/compose"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	roleArgs = `psql -c "SELECT create_role_if_not_exists('%s', '%s');" 1>/dev/null`
	dbArgs   = `createdb -O %s %s 2>/dev/null || true`
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply a configuration by file name",
	Run:   applyProject,
}

func init() {
	rootCmd.AddCommand(applyCmd)

	ageKeyFlags(applyCmd, "age-key", ageKeyFileName)
	configFileFlags(applyCmd)

	applyCmd.Flags().Bool("dry-run", false, "Simulate an apply and generate docker-compose.yaml without secrets")
}

func applyProject(cmd *cobra.Command, _ []string) {
	logger := log.WithField("command", "apply")

	configFilePath, _ := cmd.Flags().GetString("file")
	if len(configFilePath) == 0 {
		logger.Fatal("config file not provided")
	}

	ageKey, err := getAgeKey(cmd, "age-key")
	if err != nil {
		logger.Fatal(err)
	}

	dec, err := secrets.NewAgeCryptFromString(ageKey)
	if err != nil {
		logger.Fatal(err)
	}

	scope := &meta.Scope{
		DryRun: getBool(cmd, "dry-run"),
		AgeKey: dec,
		Project: &composeTypes.Project{
			Services: composeTypes.Services{},
		},
	}

	apps, err := readConfigFile(configFilePath)
	if err != nil {
		logger.Fatal(err)
	}

	for _, app := range apps {
		if err = runtime.Convert(app, new(composeTypes.ServiceConfig), scope); err != nil {
			logger.Fatal(err)
		}
	}

	comp, err := compose.NewComposeService()
	if err != nil {
		logger.Fatal(err)
	}

	if scope.DryRun {
		enc := yaml.NewEncoder(os.Stdout)
		enc.SetIndent(2)
		if err = enc.Encode(scope.Project); err != nil {
			logger.Fatal(err)
		}
		return
	}

	if err = comp.Create(cmd.Context(), scope.Project); err != nil {
		logger.Fatal(err)
	}

	servicesWithDependsOn := scope.Project.ServicesWithDependsOn()
	var servicesWithoutDependsOn []string
	err = composev2.InDependencyOrder(cmd.Context(), scope.Project, func(ctx context.Context, s string) error {
		if !in(servicesWithDependsOn, s) {
			servicesWithoutDependsOn = append(servicesWithoutDependsOn, s)
		}
		return nil
	})
	if err != nil {
		logger.Fatal(err)
	}

	if err = comp.Start(cmd.Context(), scope.Project.Name, servicesWithoutDependsOn...); err != nil {
		logger.Fatal(err)
	}

	err = composev2.InDependencyOrder(cmd.Context(), scope.Project, func(ctx context.Context, s string) error {
		if in(servicesWithDependsOn, s) {
			svc := scope.Project.Services[s]
			for depSvcName := range svc.DependsOn {
				depSvc := scope.Project.Services[depSvcName]
				app := depSvc.CustomLabels[compose.ADLabel]
				if len(app) == 0 {
					return nil
				}

				if app == "postgres" {
					dbname := scope.Project.Environment[compose.PostgresDbNameFilename+"-"+s]
					dbuser := scope.Project.Environment[compose.PostgresUserFilename+"-"+s]
					dbpassword := scope.Project.Environment[compose.PostgresPasswordFilename+"-"+s]
					roleCreate := fmt.Sprintf(roleArgs, dbuser, dbpassword)
					dbCreate := fmt.Sprintf(dbArgs, dbuser, dbname)

					for i, arg := range []string{roleCreate, dbCreate} {
						if err = comp.Exec(cmd.Context(), depSvc.ContainerName, "/bin/sh", "-ce", arg); err != nil {
							logger.Fatalf("prepare DB failed: %d", i)
						}
					}

					if err = comp.Start(cmd.Context(), scope.Project.Name, s); err != nil {
						logger.Fatal(err)
					}
				}
			}
		}
		return err
	})
	if err != nil {
		logger.Fatal(err)
	}
}

func in(a []string, s string) bool {
	for _, i := range a {
		if i == s {
			return true
		}
	}
	return false
}
