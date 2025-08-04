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

package helpers

import (
	"fmt"
	"time"

	composeTypes "github.com/compose-spec/compose-go/v2/types"
)

type ModHelper func(*composeTypes.Project) error

type ModHelpers []ModHelper

func NewModHelpers() ModHelpers {
	return ModHelpers{}
}

type Mapping map[string]string

func (m Mapping) Values() []string {
	values := make([]string, 0, len(m))
	for k, v := range m {
		if len(v) > 0 {
			k = fmt.Sprintf("%s=%s", k, v)
		}
		values = append(values, k)
	}
	return values
}

type Depended struct {
	Service   string
	Condition string
	Required  bool
	Restart   bool
}

type Env struct {
	Name  string
	Value *string
}

type HealthCheckConfig struct {
	Cmd           []string
	Interval      time.Duration
	Timeout       time.Duration
	StartPeriod   time.Duration
	StartInterval time.Duration
	Retries       uint64
}

type Secret struct {
	Source     string
	Target     string
	EnvKey     string
	EnvFileKey string
	Value      string
	Rewrite    bool
	FileMode   int64
	UID        string
	GID        string
}

type TmpFs struct {
	Target       string
	MountOptions Mapping
}
