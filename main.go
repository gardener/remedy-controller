package main

import (
	"fmt"
	"os"

	"github.wdf.sap.corp/kubernetes/azure-remeny-controller/pkg/cmd"
)

func main() {
	rootCmd := cmd.GetRootCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
