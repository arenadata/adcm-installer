package compose

import (
	"strings"

	"github.com/gosimple/slug"
)

func Concat(sep string, parts ...string) string {
	var out []string

	var prev string
	for _, part := range parts {
		part = strings.ToLower(part)
		if prev != part && len(part) > 0 {
			out = append(out, part)
		}
		prev = part
	}

	return strings.Join(out, sep)
}

func containerName(namespace, kind, name string) string {
	return slug.Make(Concat("-", namespace, kind, name))
}

func serviceName(kind, name string) string {
	return slug.Make(Concat("-", kind, name))
}
