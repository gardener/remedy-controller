// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gardener/remedy-controller/pkg/version"
)

func getVersionCommand() *cobra.Command {
	var (
		cmd = &cobra.Command{
			Use:  "version",
			Long: "Get detailed version and build information",
			Run: func(_ *cobra.Command, _ []string) {
				fmt.Println(version.Get())
			},
		}
	)
	return cmd
}
