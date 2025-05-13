/*
 Copyright (c) 2025 Arenadata Softwer LLC.
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

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
