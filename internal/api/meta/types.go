package meta

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/arenadata/arenadata-installer/pkg/secrets"
	composeTypes "github.com/compose-spec/compose-go/v2/types"
)

type TypeMeta struct {
	ApiVersion string `json:"apiVersion" yaml:"apiVersion"`
	Kind       string `json:"kind" yaml:"kind"`
}

type ObjectMeta struct {
	Name        string            `json:"name" yaml:"name"`
	Namespace   string            `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
}

func (m TypeMeta) GroupVersionKind() GroupVersionKind {
	if len(m.ApiVersion) == 0 || m.ApiVersion == "/" {
		return GroupVersionKind{}
	}

	gvk := GroupVersionKind{Kind: m.Kind}
	gv := strings.Split(m.ApiVersion, "/")
	switch len(gv) {
	case 1:
		gvk.Version = gv[0]
	case 2:
		gvk.Group = gv[0]
		gvk.Version = gv[1]
	default:
		return GroupVersionKind{}
	}

	return gvk
}

func (m ObjectMeta) Validate() error {
	pattern := `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	name := regexp.MustCompile(pattern)

	if !name.MatchString(m.Name) {
		return fmt.Errorf("invalid name: %q. Regex used for validation is %q", m.Name, pattern)
	}

	if len(m.Name) > 63 {
		return fmt.Errorf("the name must be no longer than 63 characters")
	}

	if !name.MatchString(m.Namespace) {
		return fmt.Errorf("invalid namespace: %q. Regex used for validation is %q", m.Namespace, pattern)
	}

	if len(m.Namespace) > 63 {
		return fmt.Errorf("the namespec must be no longer than 63 characters")
	}

	return nil
}

type ConversionScope interface {
	Meta() *Meta
}

type Meta struct {
	Context any
}

type Scope struct {
	DryRun  bool
	AgeKey  *secrets.AgeCrypt
	Project *composeTypes.Project
}

func (s *Scope) Meta() *Meta {
	return &Meta{
		Context: s,
	}
}
