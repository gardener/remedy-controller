package main

import (
	"fmt"
	"os"

	"github.wdf.sap.corp/kubernetes/remedy-controller/cmd/remedy-applier-azure/commands"
)

func main() {
	rootCmd := commands.GetRootCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
