package v1alpha1

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/arenadata/arenadata-installer/internal/api/meta"
	"github.com/arenadata/arenadata-installer/pkg/compose"

	composeTypes "github.com/compose-spec/compose-go/v2/types"
)

func adcmDefaults(app *Application) {
	if len(app.Spec.Image.Registry) == 0 {
		app.Spec.Image.Registry = compose.ADImageRegistry
	}

	if len(app.Spec.Image.Name) == 0 {
		app.Spec.Image.Name = compose.ADCMImageName
	}

	if len(app.Spec.Image.Tag) == 0 {
		app.Spec.Image.Tag = compose.ADCMImageTag
	}

	if app.Spec.Ingress == nil {
		app.Spec.Ingress = &Ingress{{
			Port:       compose.ADCMPort,
			TargetPort: compose.ADCMPort,
		}}
	} else {
		if len(*app.Spec.Ingress) == 1 && (*app.Spec.Ingress)[0].TargetPort == 0 {
			(*app.Spec.Ingress)[0].TargetPort = compose.ADCMPort
		}
	}

	if len(app.Spec.Volumes) == 0 {
		app.Spec.Volumes = append(app.Spec.Volumes, Volume{
			Source: compose.ContainerName(app.Namespace, app.Kind, app.Name),
			Target: compose.ADCMVolumeTarget,
		})
	} else if len(app.Spec.Volumes) == 1 && len(app.Spec.Volumes[0].Target) == 0 {
		app.Spec.Volumes[0].Target = compose.ADCMVolumeTarget
	}

	if app.Spec.Env == nil {
		app.Spec.Env = make(map[string]*string)
	}
}

func adcm(app *Application, svc *composeTypes.ServiceConfig, s meta.ConversionScope) error {
	if err := secretsRequired(app); err != nil {
		return err
	}

	adcmDefaults(app)

	if v, ok := app.Annotations[DependsOnKey]; ok {
		kn := strings.Split(v, ".")
		if len(kn) != 2 {
			return fmt.Errorf("adcm: invalid depends-on key format: %s", v)
		}

		deps := compose.ServiceName(kn[0], kn[1])
		app.Spec.Env["DB_HOST"] = &deps

		if _, ok := app.Spec.Env["DB_PORT"]; !ok {
			def := strconv.Itoa(int(compose.PostgresPort))
			app.Spec.Env["DB_PORT"] = &def
		}
	}

	annoDB := app.Annotations[DatabaseKey]
	envDB := app.Spec.Env["DB_NAME"]
	if envDB == nil {
		if len(annoDB) == 0 {
			// generate new DB name
			annoDB = fmt.Sprintf("%s_%s", app.Namespace, app.Name)
			app.Annotations[DatabaseKey] = annoDB
		}

		app.Spec.Env["DB_NAME"] = &annoDB
	} else {
		app.Annotations[DatabaseKey] = *envDB
	}

	return applicationToService(app, svc, s)
}
