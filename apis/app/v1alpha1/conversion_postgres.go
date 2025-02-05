package v1alpha1

import (
	"time"

	"github.com/arenadata/arenadata-installer/internal/api/meta"
	"github.com/arenadata/arenadata-installer/pkg/compose"
	"github.com/arenadata/arenadata-installer/pkg/utils"

	composeTypes "github.com/compose-spec/compose-go/v2/types"
)

func postgresDefaults(app *Application) {
	if len(app.Spec.Image.Name) == 0 {
		app.Spec.Image.Name = compose.PostgresImageName
	}

	if len(app.Spec.Image.Tag) == 0 {
		app.Spec.Image.Tag = compose.PostgresImageTag
	}

	if len(app.Spec.Volumes) == 0 {
		app.Spec.Volumes = append(app.Spec.Volumes, Volume{
			Source: compose.ContainerName(app.Namespace, app.Kind, app.Name),
			Target: compose.PostgresVolumeTarget,
		})
	} else if len(app.Spec.Volumes) == 1 && len(app.Spec.Volumes[0].Target) == 0 {
		app.Spec.Volumes[0].Target = compose.PostgresVolumeTarget
	}

	if app.Spec.Env == nil {
		app.Spec.Env = make(map[string]*string)
	}
}

func postgres(app *Application, svc *composeTypes.ServiceConfig, s meta.ConversionScope) error {
	if err := secretsRequired(app); err != nil {
		return err
	}

	postgresDefaults(app)

	svc.HealthCheck = &composeTypes.HealthCheckConfig{
		Test: composeTypes.HealthCheckTest{
			"CMD-SHELL", "pg_isready", "--quiet",
		},
		Interval: utils.Ptr(composeTypes.Duration(10 * time.Second)),
		Timeout:  utils.Ptr(composeTypes.Duration(3 * time.Second)),
		Retries:  utils.Ptr(uint64(3)),
	}

	if svc.Environment == nil {
		svc.Environment = make(composeTypes.MappingWithEquals)
	}
	svc.Environment["PGUSER"] = utils.Ptr(compose.PostgresUser)

	scope := s.Meta().Context.(*meta.Scope)

	svc.Name = compose.ServiceName(app.Kind, app.Name)
	helpers := compose.NewHelpers()
	initScripts := map[string]string{"/docker-entrypoint-initdb.d/init.sql": compose.PostgresHelperSQLScript}
	helpers = append(helpers, compose.Configs(initScripts))
	if err := helpers.Run(scope.Project, svc); err != nil {
		return err
	}

	return applicationToService(app, svc, s)
}
