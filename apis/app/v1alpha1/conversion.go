package v1alpha1

import (
	"fmt"
	"strings"

	"github.com/arenadata/arenadata-installer/internal/api/meta"
	"github.com/arenadata/arenadata-installer/pkg/compose"
	"github.com/arenadata/arenadata-installer/pkg/secrets"

	composeTypes "github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/compose/v2/pkg/api"
)

func applicationToService(app *Application, svc *composeTypes.ServiceConfig, s meta.ConversionScope) error {
	scope := s.Meta().Context.(*meta.Scope)
	prj := scope.Project

	if len(app.Namespace) == 0 {
		app.Namespace = compose.DefaultNamespace
	}

	if len(prj.Name) == 0 {
		prj.Name = app.Namespace
	}

	kind := strings.ToLower(app.Kind)
	if prj.Name != app.Namespace {
		return fmt.Errorf("project %q and %s/%s %q namespaces mismatch", prj.Name, kind, app.Name, app.Namespace)
	}

	if svc.Environment == nil {
		svc.Environment = make(composeTypes.MappingWithEquals)
	}

	if len(svc.Name) == 0 {
		svc.Name = compose.ServiceName(app.Kind, app.Name)
	}
	svc.ContainerName = compose.ContainerName(app.Namespace, app.Kind, app.Name)
	svc.Image = app.Spec.Image.String()
	svc.Restart = composeTypes.RestartPolicyUnlessStopped
	svc.Platform = compose.DefaultPlatform
	svc.CustomLabels = map[string]string{
		api.ProjectLabel:     app.Namespace,
		api.ServiceLabel:     svc.Name,
		api.VersionLabel:     api.ComposeVersion,
		api.OneoffLabel:      "False",
		api.ConfigFilesLabel: "",
		compose.ADLabel:      kind,
	}

	svcRef := kind + "/" + strings.ToLower(app.Name)

	networkName := compose.DefaultNetworkName
	if v := app.Annotations[NetworkKey]; len(v) > 0 {
		networkName = v
	}

	helpers := compose.NewHelpers()
	helpers = append(helpers, compose.Network(networkName))

	if v, ok := app.Annotations[DependsOnKey]; ok {
		helpers = append(helpers, compose.DependsOn(v))
	}

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

	if len(app.Spec.Env) > 0 {
		if svc.Environment == nil {
			svc.Environment = make(composeTypes.MappingWithEquals)
		}

		helpers = append(helpers, compose.Environment(app.Spec.Env))
	}

	if len(app.Spec.Volumes) > 0 {
		if prj.Volumes == nil {
			prj.Volumes = make(composeTypes.Volumes)
		}

		helpers = append(helpers, volumes(app))
	}

	if app.Spec.Ingress != nil && len(*app.Spec.Ingress) > 0 {
		helpers = append(helpers, ingress(app))
	}

	if err := helpers.Run(prj, svc); err != nil {
		return fmt.Errorf("%s: %v", svcRef, err)
	}

	prj.Services[svc.Name] = *svc

	return nil
}

func volumes(app *Application) compose.Helper {
	var vols []string
	for _, volume := range app.Spec.Volumes {
		vols = append(vols, volume.String())
	}
	return compose.Volumes(vols)
}

func ingress(app *Application) compose.Helper {
	var ports []string
	for _, pub := range *app.Spec.Ingress {
		ports = append(ports, pub.String())
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
