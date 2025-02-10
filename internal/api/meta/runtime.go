package meta

type Object interface {
	GroupVersionKind() GroupVersionKind
}

type GroupVersionKind struct {
	Group   string
	Version string
	Kind    string
}
