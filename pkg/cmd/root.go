package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	azclient "github.wdf.sap.corp/kubernetes/azure-remedy-controller/pkg/azure/client"
	k8sclient "github.wdf.sap.corp/kubernetes/azure-remedy-controller/pkg/k8s"
	"github.wdf.sap.corp/kubernetes/azure-remedy-controller/pkg/remedies/pubips"
)

// GetRootCommand TODO
func GetRootCommand() *cobra.Command {
	var (
		kubeconfigPath, azureConfigPath string
		cmd                             = &cobra.Command{
			Use:  "azure-remedy-controller",
			Long: "TODO",
			Run: func(cmd *cobra.Command, args []string) {
				k8sClientSet, err := k8sclient.GetClientSet(kubeconfigPath)
				if err != nil {
					fmt.Println(err.Error())
					os.Exit(1)
				}

				azConfig, err := azclient.ReadAzureConfig(azureConfigPath)
				if err != nil {
					fmt.Println(err.Error())
					os.Exit(1)
				}

				azdrivers, err := azclient.NewAzureDriverClients(azConfig)
				if err != nil {
					fmt.Println(err.Error())
					os.Exit(1)
				}

				go pubips.CleanPubIps(k8sClientSet, azdrivers)

				select {}
			},
		}
	)
	cmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", "", "path to kubeconfig to target whatever")
	cmd.Flags().StringVar(&azureConfigPath, "azure-config", "", "path to Azure config")

	cmd.MarkFlagRequired("kubeconfig")
	cmd.MarkFlagRequired("azure-config")

	return cmd
}
