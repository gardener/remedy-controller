package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-07-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-11-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"

	azclient "github.com/gardener/remedy-controller/pkg/client/azure"
)

func main() {
	var vmName string
	if len(os.Args) > 1 {
		vmName = os.Args[1]
	}
	if len(vmName) == 0 {
		fmt.Println("VM name not specified")
		os.Exit(1)
	}

	// Read Azure credentials
	credentials, err := azclient.ReadConfig("dev/credentials.yaml")
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	// Create OAuth config
	oauthConfig, err := adal.NewOAuthConfig(azure.PublicCloud.ActiveDirectoryEndpoint, credentials.TenantID)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	// Create service principal token
	servicePrincipalToken, err := adal.NewServicePrincipalToken(*oauthConfig, credentials.ClientID, credentials.ClientSecret, azure.PublicCloud.ResourceManagerEndpoint)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	authorizer := autorest.NewBearerAuthorizer(servicePrincipalToken)

	// Create clients
	vmClient := compute.NewVirtualMachinesClient(credentials.SubscriptionID)
	vmClient.Authorizer = authorizer
	nicClient := network.NewInterfacesClient(credentials.SubscriptionID)
	nicClient.Authorizer = authorizer

	// Create context
	ctx := context.TODO()

	// Get VM
	fmt.Printf("Getting VM %s...\n", vmName)
	vm, err := vmClient.Get(ctx, credentials.ResourceGroup, vmName, compute.InstanceView)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	vmData, err := json.Marshal(vm)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	fmt.Printf("VM %s data: %s\n", vmName, string(vmData))

	// Delete VM and wait for it to be deleted
	fmt.Printf("Deleting VM %s...\n", vmName)
	future2, err := vmClient.Delete(ctx, credentials.ResourceGroup, vmName)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	fmt.Printf("Waiting for VM %s to be deleted...\n", vmName)
	if err = future2.WaitForCompletionRef(ctx, vmClient.Client); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	// Create or update the VM
	// Change a few VM properties so that the CreateOrUpdate call could pass without an error
	fmt.Printf("Creating or updating VM %s...\n", vmName)
	vm.VMID = nil
	vm.StorageProfile.OsDisk.ManagedDisk.ID = nil
	vm.OsProfile.RequireGuestProvisionSignal = nil
	future3, err := vmClient.CreateOrUpdate(ctx, credentials.ResourceGroup, vmName, vm)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	fmt.Printf("Waiting for VM %s to be created or updated...\n", vmName)
	if err = future3.WaitForCompletionRef(ctx, vmClient.Client); err != nil {
		fmt.Println(err.Error())
		// os.Exit(1)
	}

	// Get VM again
	// Here, the VM should be in a failed state
	fmt.Printf("Getting VM %s...\n", vmName)
	vm, err = vmClient.Get(ctx, credentials.ResourceGroup, vmName, compute.InstanceView)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	vmData, err = json.Marshal(vm)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	fmt.Printf("VM %s data: %s\n", vmName, string(vmData))
}
