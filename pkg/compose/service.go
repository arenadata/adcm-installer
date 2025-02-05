package compose

import (
	"strings"
)

func concat(sep string, parts ...string) string {
	var out []string

	var prev string
	for _, part := range parts {
		part = strings.ToLower(part)
		if prev != part {
			out = append(out, part)
		}
		prev = part
	}

	return strings.Join(out, sep)
}

func ContainerName(namespace, kind, name string) string {
	return concat("-", namespace, kind, name)
}

func ServiceName(kind, name string) string {
	return concat("-", kind, name)
}
