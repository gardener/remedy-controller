package azure

import (
	"encoding/json"
	"io"
	"os"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-11-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
)

// AzCredentials TODO
type AzCredentials struct {
	SubscriptionID string `json:"subscriptionID"`
	TenantID       string `json:"tenantID"`
	ClientID       string `json:"clientID"`
	ClientSecret   string `json:"clientSecret"`
	ResourceGroup  string `json:"resourceGroup"`
}

// AzureDriverClients TODO
type AzureDriverClients struct {
	Ip  network.PublicIPAddressesClient
	Nic network.InterfacesClient
	Lb  network.LoadBalancersClient
}

// ReadAzureConfig TODO
func ReadAzureConfig(path string) (*AzCredentials, error) {
	input, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer input.Close()

	var (
		credentials = &AzCredentials{}
		decoder     = json.NewDecoder(io.Reader(input))
	)
	if err := decoder.Decode(credentials); err != nil {
		return nil, err
	}
	return credentials, nil
}

// NewAzureDriverClients TODO
func NewAzureDriverClients(credentials *AzCredentials) (*AzureDriverClients, error) {
	oauthConfig, err := adal.NewOAuthConfig(azure.PublicCloud.ActiveDirectoryEndpoint, credentials.TenantID)
	if err != nil {
		return nil, err
	}

	servicePrincipalToken, err := adal.NewServicePrincipalToken(*oauthConfig, credentials.ClientID, credentials.ClientSecret, azure.PublicCloud.ResourceManagerEndpoint)
	if err != nil {
		return nil, err
	}
	authorizer := autorest.NewBearerAuthorizer(servicePrincipalToken)

	ipclient := network.NewPublicIPAddressesClient(credentials.SubscriptionID)
	ipclient.Authorizer = authorizer

	nicclient := network.NewInterfacesClient(credentials.SubscriptionID)
	nicclient.Authorizer = authorizer

	lbclient := network.NewLoadBalancersClient(credentials.SubscriptionID)
	lbclient.Authorizer = authorizer

	return &AzureDriverClients{
		Ip:  ipclient,
		Nic: nicclient,
		Lb:  lbclient,
	}, nil
}
