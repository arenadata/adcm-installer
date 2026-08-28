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

package services

const (
	ADCMImage                 = "hub.arenadata.io/adcm/adcm"
	ADCMTag                   = "3.0.0"
	ADCMPublishPort    uint16 = 8000
	ADCMPublishSSLPort uint16 = 8443
	ADCMMountPath             = "/adcm/data"

	ADCMVaultMinTag        = "2.12.0"
	ADCMUnprivilegedMinTag = "3.0.0"
	ADCMWorkerMinTag       = "3.0.0"
	ADCMHealthCheckMinTag  = "3.0.0"

	ADCMLivenessProbePath = "/api/health/live"

	ADPGImage                = "hub.arenadata.io/adcm/postgres"
	ADPGTag                  = "v16.3.1"
	ADPGPublishPort   uint16 = 5432
	ADPGDataMountPath        = "/data"

	ConsulImage              = "hub.arenadata.io/adcm/consul"
	ConsulTag                = "v1.0.0"
	ConsulPublishPort uint16 = 8500

	VaultImage              = "openbao/openbao"
	VaultTag                = "2.2.0"
	VaultPublishPort uint16 = 8200
)
