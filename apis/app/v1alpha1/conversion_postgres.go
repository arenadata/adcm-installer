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
			Source: compose.Concat("-", app.Namespace, app.Kind, app.Name),
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
	scope := s.Meta().Context.(*meta.Scope)

	postgresDefaults(app)

	initScripts := map[string]string{"/docker-entrypoint-initdb.d/init.sql": compose.PostgresHelperSQLScript}
	healthCheckConfig := compose.HealthCheckConfig{
		Cmd:      []string{"CMD-SHELL", "pg_isready", "--quiet"},
		Interval: 10 * time.Second,
		Timeout:  3 * time.Second,
		Retries:  3,
	}

	helpers := compose.NewModHelpers()
	// compose.SetServiceName call before compose.Configs/Secrets
	helpers = append(helpers, compose.ServiceName(app.Kind, app.Name))
	helpers = append(helpers, compose.HealthCheck(healthCheckConfig))
	helpers = append(helpers, compose.Environment(compose.Env{Name: "PGUSER", Value: utils.Ptr(compose.PostgresUser)}))
	helpers = append(helpers, compose.Configs(initScripts))
	helpers = append(helpers, compose.User(compose.PostgresUser, ""))

	if err := helpers.Run(scope.Project, svc); err != nil {
		return err
	}

	return applicationToService(app, svc, s)
}
