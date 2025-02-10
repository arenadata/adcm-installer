package secrets

type Secrets struct {
	Files map[string]*File  `json:"f,omitempty" yaml:"files,omitempty"`
	Env   map[string]string `json:"e,omitempty" yaml:"env,omitempty"`
}

type File struct {
	EnvKey *string `json:"e,omitempty" yaml:"envKey,omitempty"`
	Data   string  `json:"d" yaml:"data"`
}
