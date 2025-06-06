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
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var updateKeyCmd = &cobra.Command{
	Use:   "update-key",
	Short: "NOT IMPLEMENTED",
	Long: `Allows you to rotate the private key.
- --file specifies the path to the configuration file
- --new-age-key takes the value of the private key in clear text. Has priority
                over --new-age-key-file
- --new-age-key-file takes the value of the path to the file with the private
                     key
- --old-age-key takes the value of the private key in clear text. Has priority
                over --old-age-key-file
- --old-age-key-file takes the value of the path to the file with the private
                     key`,
	Run: secretsUpdateKey,
}

func init() {
	secretsCmd.AddCommand(updateKeyCmd)

	ageKeyFlags(updateKeyCmd, "old-age-key", ageKeyFileName, updateKeyCmd.MarkFlagsOneRequired)
	ageKeyFlags(updateKeyCmd, "new-age-key", "", updateKeyCmd.MarkFlagsOneRequired)

	configFileFlags(updateKeyCmd)
}

func secretsUpdateKey(cmd *cobra.Command, _ []string) {
	log.Fatal("not implemented")
}
