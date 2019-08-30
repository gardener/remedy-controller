package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.wdf.sap.corp/kubernetes/remedy-controller/pkg/version"
)

func getVersionCommand() *cobra.Command {
	var (
		cmd = &cobra.Command{
			Use:  "version",
			Long: "Get detailed version and build information",
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println(version.Get())
			},
		}
	)
	return cmd
}
