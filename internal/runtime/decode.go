package runtime

import (
	"bytes"
	"fmt"

	"github.com/arenadata/arenadata-installer/internal/api/meta"

	"gopkg.in/yaml.v3"
)

func Decode(b []byte) (meta.Object, error) {
	gvk := new(meta.TypeMeta)
	if err := yaml.Unmarshal(b, gvk); err != nil {
		return nil, err
	}

	obj := reg.get(gvk.GroupVersionKind())
	if obj == nil {
		return nil, fmt.Errorf("unknown apiVersion: %s", gvk.ApiVersion)
	}

	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)

	if err := dec.Decode(obj); err != nil {
		return nil, err
	}

	return obj.(meta.Object), nil
}

func Convert(obj meta.Object, v any, scope meta.ConversionScope) error {
	gvk := obj.GroupVersionKind()
	conv := reg.converter(gvk)
	if conv == nil {
		return fmt.Errorf("converter for kind %s (%s/%s) not found", gvk.Kind, gvk.Group, gvk.Version)
	}

	return conv(obj, v, scope)
}
