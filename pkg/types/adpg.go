package types

type Database struct {
	Owner      string   `json:"owner,omitempty" yaml:"owner,omitempty"`
	Extensions []string `json:"extensions,omitempty" yaml:"extensions,omitempty"`
	Scripts    []string `json:"scripts,omitempty" yaml:"scripts,omitempty"`
}

type Role struct {
	Password string   `json:"password,omitempty" yaml:"password,omitempty"`
	Options  []string `json:"options,omitempty" yaml:"options,omitempty"`
	Grant    []string `json:"grant,omitempty" yaml:"grant,omitempty"`
}

type PGInit struct {
	DB   map[string]*Database `json:"db,omitempty" yaml:"db,omitempty"`
	Role map[string]*Role     `json:"role,omitempty" yaml:"role,omitempty"`
}

func NewPGInit() PGInit {
	return PGInit{
		DB:   make(map[string]*Database),
		Role: make(map[string]*Role),
	}
}
