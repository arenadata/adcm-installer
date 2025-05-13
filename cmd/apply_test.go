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

package cmd

import (
	"strings"
	"testing"

	"github.com/arenadata/adcm-installer/pkg/compose"

	"github.com/bmizerany/assert"
	composeTypes "github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/compose/v2/pkg/api"
)

func Test_fillADCMLabels(t *testing.T) {
	prj := &composeTypes.Project{Services: make(composeTypes.Services)}
	addService("one", prj)
	addService("two", prj)
	addService("three", prj)

	fillADCMLabels(prj)

	for _, svc := range prj.Services {
		if _, ok := svc.CustomLabels[compose.ADLabel]; !ok {
			t.Errorf("ADCM label not set for service %s", svc.Name)
		}
	}
}

func Test_fillProjectFields(t *testing.T) {
	prj := &composeTypes.Project{Name: "empty-project", Services: make(composeTypes.Services)}
	addService("one", prj)
	addService("two", prj)
	addService("three", prj)

	fillProjectFields(prj)

	tests := []struct {
		name string
		lbl  string
		want string
	}{
		{"ProjectLabel", api.ProjectLabel, prj.Name},
		{"ServiceLabel", api.ServiceLabel, ""},
		{"VersionLabel", api.VersionLabel, api.ComposeVersion},
		{"WorkingDirLabel", api.WorkingDirLabel, prj.WorkingDir},
		{"ConfigFilesLabel", api.ConfigFilesLabel, strings.Join(prj.ComposeFiles, ",")},
		{"OneoffLabel", api.OneoffLabel, "False"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, svc := range prj.Services {
				v, ok := svc.CustomLabels[tt.lbl]
				if !ok {
					t.Errorf("Label not set for service %s", svc.Name)
				}
				if tt.lbl == api.ServiceLabel {
					assert.Equal(t, v, svc.Name)
				} else {
					assert.Equal(t, v, tt.want)
				}
			}
		})
	}
}
