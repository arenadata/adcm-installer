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

package compose

const (
	DefaultPlatform = "linux/amd64"

	ADLabel           = "app.arenadata.io"
	ADAppTypeLabelKey = ADLabel + "/type"

	SecretsPath = "/run/csecrets"

	ADCMImage              = "hub.arenadata.io/adcm/adcm"
	ADCMTag                = "2.6.0"
	ADCMPublishPort uint16 = 8000

	ADPGImage              = "hub.arenadata.io/adcm/postgres"
	ADPGTag                = "v16.3.1"
	ADPGPublishPort uint16 = 5432

	ConsulImage              = "hub.arenadata.io/adcm/consul"
	ConsulTag                = "v0.0.0"
	ConsulPublishPort uint16 = 8500

	VaultImage              = "openbao/openbao"
	VaultTag                = "2.2.0"
	VaultPublishPort uint16 = 8200
)
