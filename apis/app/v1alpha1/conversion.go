package v1alpha1

import (
	"fmt"
	"strings"

	"github.com/arenadata/arenadata-installer/internal/api/meta"
	"github.com/arenadata/arenadata-installer/pkg/compose"
	"github.com/arenadata/arenadata-installer/pkg/secrets"

	composeTypes "github.com/compose-spec/compose-go/v2/types"
)

func applicationToService(app *Application, svc *composeTypes.ServiceConfig, s meta.ConversionScope) error {
	scope := s.Meta().Context.(*meta.Scope)
	prj := scope.Project

	networkName := compose.DefaultNetworkName
	if v := app.Annotations[NetworkKey]; len(v) > 0 {
		networkName = v
	}

	if len(app.Namespace) == 0 {
		app.Namespace = compose.DefaultNamespace
	}

	svc.Restart = composeTypes.RestartPolicyUnlessStopped
	svc.Platform = compose.DefaultPlatform

	helpers := compose.NewModHelpers()
	helpers = append(helpers,
		compose.ServiceName(app.Kind, app.Name),
		compose.ContainerName(app.Namespace, app.Kind, app.Name),
		compose.Image(app.Spec.Image.String()),
		compose.ProjectName(app.Namespace),
		compose.BaseCustomLabels(app.Kind, app.Namespace),
		compose.CapDropAll(),
		compose.Network(networkName),
	)

	//if prj.Name != app.Namespace {
	//	return fmt.Errorf("project %q and %s/%s %q namespaces mismatch", prj.Name, kind, app.Name, app.Namespace)
	//}

	svcRef := strings.ToLower(app.Kind) + "/" + strings.ToLower(app.Name)

	secretData := app.Annotations[SecretsAgeKey]
	if !scope.DryRun && len(secretData) > 0 {
		recipient := app.Annotations[SecretsAgeRecipientKey]
		decryptData, err := secrets.DecryptData(scope.AgeKey, secretData, recipient)
		if err != nil {
			return fmt.Errorf("%s: %v", svcRef, err)
		}

		if dbName := app.Annotations[DatabaseKey]; len(dbName) > 0 {
			decryptData.Files[compose.PostgresDbNameFilename] = &secrets.File{Data: dbName}
		}

		helpers = append(helpers, compose.Secrets(decryptData))
	}

	if v, ok := app.Annotations[DependsOnKey]; ok {
		helpers = append(helpers, compose.DependsOn(v))
	}

	helpers = append(helpers, compose.Environment(compose.ToEnv(app.Spec.Env)...))
	helpers = append(helpers, volumes(app))
	helpers = append(helpers, ingress(app))

	if err := helpers.Run(prj, svc); err != nil {
		return fmt.Errorf("%s: %v", svcRef, err)
	}

	prj.Services[svc.Name] = *svc

	return nil
}

func volumes(app *Application) compose.ModHelper {
	var vols []string
	for _, volume := range app.Spec.Volumes {
		vols = append(vols, volume.String())
	}
	return compose.Volumes(vols)
}

func ingress(app *Application) compose.ModHelper {
	var ports []string
	if app.Spec.Ingress != nil {
		for _, pub := range *app.Spec.Ingress {
			ports = append(ports, pub.String())
		}
	}
	return compose.Ports(ports)
}

func secretsRequired(app *Application) error {
	if len(app.Annotations[SecretsAgeKey]) == 0 {
		svcRef := strings.ToLower(app.Kind + "/" + app.Name)
		return fmt.Errorf("%s: secrets not provided", svcRef)
	}

	return nil
}
