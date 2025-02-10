package runtime

import (
	"fmt"
	"reflect"

	"github.com/arenadata/arenadata-installer/internal/api/meta"
)

var reg *register

func init() {
	reg = &register{
		gvk: make(map[meta.GroupVersionKind]*object),
	}
}

type conversion func(in, out any, scope meta.ConversionScope) error

type register struct {
	gvk map[meta.GroupVersionKind]*object
}

type object struct {
	obj        reflect.Type
	conversion func(in, out any, scope meta.ConversionScope) error
}

func (r *register) register(gv meta.GroupVersionKind, obj any) {
	o, ok := r.gvk[gv]
	if !ok {
		o = &object{}

		r.gvk[gv] = o
	}

	switch t := obj.(type) {
	case meta.Object:
		o.obj = reflect.TypeOf(t).Elem()
	case conversion:
		o.conversion = t
	default:
		panic(fmt.Sprintf("unknown type %T", t))
	}
}

func (r *register) converter(gvk meta.GroupVersionKind) conversion {
	obj, ok := r.gvk[gvk]
	if !ok {
		return nil
	}

	return obj.conversion
}

func (r *register) get(gvk meta.GroupVersionKind) any {
	obj, ok := r.gvk[gvk]
	if !ok {
		return nil
	}

	return reflect.New(obj.obj).Elem().Addr().Interface()
}

func Register(gv GroupVersion, obj meta.Object, aliases ...string) {
	kind := reflect.TypeOf(obj).Elem().Name()
	gvk := meta.GroupVersionKind{Group: gv.Group, Version: gv.Version, Kind: kind}

	reg.register(gvk, obj)

	for _, alias := range aliases {
		gvk.Kind = alias
		reg.register(gvk, obj)
	}
}

func RegisterConversions(gvk meta.GroupVersionKind, fn conversion) {
	reg.register(gvk, fn)
}
