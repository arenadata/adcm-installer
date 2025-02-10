package v1alpha1

import (
	"github.com/arenadata/arenadata-installer/internal/api/meta"
	"github.com/arenadata/arenadata-installer/internal/runtime"

	composeTypes "github.com/compose-spec/compose-go/v2/types"
)

const (
	GroupName = "app.arenadata.io"
	Version   = "v1alpha1"
)

func init() {
	runtime.Register(
		runtime.GroupVersion{Group: GroupName, Version: Version},
		&Application{},
		"ADCM", "Postgres",
	)

	gvk := meta.GroupVersionKind{Group: GroupName, Version: Version, Kind: "Application"}
	runtime.RegisterConversions(gvk, func(in, out any, scope meta.ConversionScope) error {
		return applicationToService(in.(*Application), out.(*composeTypes.ServiceConfig), scope)
	})

	gvk.Kind = "ADCM"
	runtime.RegisterConversions(gvk, func(in, out any, scope meta.ConversionScope) error {
		return adcm(in.(*Application), out.(*composeTypes.ServiceConfig), scope)
	})

	gvk.Kind = "Postgres"
	runtime.RegisterConversions(gvk, func(in, out any, scope meta.ConversionScope) error {
		return postgres(in.(*Application), out.(*composeTypes.ServiceConfig), scope)
	})
}
