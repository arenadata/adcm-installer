package v1alpha1

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/arenadata/arenadata-installer/internal/api/meta"
	"github.com/arenadata/arenadata-installer/pkg/compose"
)

const (
	AppKeyPrefix = "app.arenadata.io/"
	DependsOnKey = AppKeyPrefix + "depends-on"
	NetworkKey   = AppKeyPrefix + "network"
	DatabaseKey  = AppKeyPrefix + "database"

	SecretsKeyPrefix       = "secrets.arenadata.io/"
	SecretsAgeRecipientKey = SecretsKeyPrefix + "age.recipient"
	SecretsAgeKey          = SecretsKeyPrefix + "local"
)

func New(kind, namespace, name string) (*Application, error) {
	if len(kind) == 0 {
		kind = "Application"
	}

	if len(namespace) == 0 {
		namespace = compose.DefaultNamespace
	}

	if len(name) < 3 {
		return nil, fmt.Errorf("the name must contain at least 3 characters, got %d", len(name))
	}

	app := &Application{
		TypeMeta: meta.TypeMeta{
			ApiVersion: fmt.Sprintf("%s/%s", GroupName, Version),
			Kind:       kind,
		},
		ObjectMeta: meta.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: make(map[string]string),
		},
		Spec: ApplicationSpec{},
	}

	switch kind {
	case "ADCM":
		adcmDefaults(app)
	case "Postgres":
		postgresDefaults(app)
	}

	return app, nil
}

type Application struct {
	meta.TypeMeta   `json:",inline" yaml:",inline"`
	meta.ObjectMeta `json:"metadata" yaml:"metadata"`
	Spec            ApplicationSpec `json:"spec" yaml:"spec"`
}

type ApplicationSpec struct {
	Image   Image              `json:"image,omitempty" yaml:"image,omitempty"`
	Ingress *Ingress           `json:"ingress,omitempty" yaml:"ingress,omitempty"`
	Volumes []Volume           `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	Env     map[string]*string `json:"env,omitempty" yaml:"env,omitempty"`

	RawOptions string `json:"options,omitempty" yaml:"options,omitempty"`
}

type Image struct {
	Registry string `json:"registry,omitempty" yaml:"registry,omitempty"`
	Name     string `json:"name,omitempty" yaml:"name,omitempty"`
	Tag      string `json:"tag,omitempty" yaml:"tag,omitempty"`
}

func (i Image) String() string {
	const sep = "/"
	var image []string

	if len(i.Registry) > 0 {
		reg := i.Registry
		if strings.HasPrefix(reg, "http") {
			const schemeSep = "://"
			idx := strings.Index(reg, schemeSep)
			reg = reg[idx+len(schemeSep):]
		}

		reg = strings.Trim(reg, sep)

		image = append(image, i.Registry)
	}

	image = append(image, strings.Trim(i.Name, sep))

	tagSep := ":"
	if strings.HasPrefix(i.Tag, "sha") {
		tagSep = "@"
	}

	img := strings.Join(image, sep)

	return img + tagSep + i.Tag
}

type Ingress []Publish

func (p *Ingress) UnmarshalYAML(unmarshal func(any) error) error {
	type alias Ingress
	var ing alias
	if err := unmarshal(&ing); err == nil {
		*p = Ingress(ing)
		return nil
	}

	var intPort int
	if err := unmarshal(&intPort); err == nil {
		*p = append(*p, Publish{Port: uint16(intPort)})
		return nil
	}

	var stringPort string
	if err := unmarshal(&stringPort); err == nil {
		pub, err := parsePublishPort(stringPort)
		if err != nil {

		}

		*p = append(*p, pub)
		return nil
	}

	var slicePort []string
	if err := unmarshal(&slicePort); err == nil {
		for _, port := range slicePort {
			pub, err := parsePublishPort(port)
			if err != nil {
				return err
			}

			*p = append(*p, pub)
		}
		return nil
	}

	var servicePorts []int
	if err := unmarshal(&servicePorts); err == nil {
		for _, port := range servicePorts {
			*p = append(*p, Publish{Port: uint16(port)})
		}
		return nil
	}

	var x any
	if err := unmarshal(&x); err == nil {
		return nil
	}

	return fmt.Errorf("invalid ingress defined: %s", x)
}

func parsePublishPort(pp string) (Publish, error) {
	s := strings.TrimSpace(pp)
	if len(s) == 0 {
		return Publish{}, fmt.Errorf("invalid port value %q", pp)
	}

	var err error
	pub := Publish{}
	ports := strings.Split(s, ":")
	switch len(ports) {
	case 1:
		pub.Port, err = parsePort(ports[0])
		if err != nil {
			return Publish{}, err
		}
	case 2:
		pub.Port, err = parsePort(ports[0])
		if err != nil {
			return Publish{}, err
		}
		pub.TargetPort, err = parsePort(ports[1])
		if err != nil {
			return Publish{}, err
		}
	case 3:
		pub.IP = ports[0]
		pub.Port, err = parsePort(ports[1])
		if err != nil {
			return Publish{}, err
		}
		pub.TargetPort, err = parsePort(ports[2])
		if err != nil {
			return Publish{}, err
		}
	default:
		return Publish{}, fmt.Errorf("invalid publish format: %q", s)
	}

	return pub, nil
}

func parsePort(s string) (uint16, error) {
	port, err := strconv.ParseUint(s, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid port value %q", s)
	}

	return uint16(port), nil
}

type Publish struct {
	IP         string `json:"ip,omitempty" yaml:"ip,omitempty"`
	Port       uint16 `json:"port" yaml:"port"`
	TargetPort uint16 `json:"target-port,omitempty" yaml:"targetPort,omitempty"`
}

func (p Publish) String() string {
	var out []string
	if len(p.IP) > 0 {
		out = append(out, p.IP)
	}
	out = append(out, strconv.Itoa(int(p.Port)), strconv.Itoa(int(p.TargetPort)))

	return strings.Join(out, ":")
}

type Volume struct {
	Source  string `json:"source" yaml:"source"`
	Target  string `json:"target,omitempty" yaml:"target,omitempty"`
	Options string `json:"options,omitempty" yaml:"options,omitempty"`
}

func (v Volume) String() string {
	vol := []string{v.Source, v.Target}
	if len(v.Options) > 0 {
		vol = append(vol, v.Options)
	}

	return strings.Join(vol, ":")
}
