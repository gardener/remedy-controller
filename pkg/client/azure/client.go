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

// Credentials TODO
type Credentials struct {
	ClientID       string `json:"aadClientId"`
	ClientSecret   string `json:"aadClientSecret"`
	TenantID       string `json:"tenantId"`
	SubscriptionID string `json:"subscriptionId"`
	ResourceGroup  string `json:"resourceGroup"`
}

// Clients TODO
type Clients struct {
	PublicIPAddressesClient network.PublicIPAddressesClient
	InterfacesClient        network.InterfacesClient
	LoadBalancersClient     network.LoadBalancersClient
}

// ReadConfig TODO
func ReadConfig(path string) (*Credentials, error) {
	input, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer input.Close()

	decoder := json.NewDecoder(io.Reader(input))
	credentials := &Credentials{}
	if err := decoder.Decode(credentials); err != nil {
		return nil, err
	}
	return credentials, nil
}

// NewClients TODO
func NewClients(credentials *Credentials) (*Clients, error) {
	oauthConfig, err := adal.NewOAuthConfig(azure.PublicCloud.ActiveDirectoryEndpoint, credentials.TenantID)
	if err != nil {
		return nil, err
	}

	servicePrincipalToken, err := adal.NewServicePrincipalToken(*oauthConfig, credentials.ClientID, credentials.ClientSecret, azure.PublicCloud.ResourceManagerEndpoint)
	if err != nil {
		return nil, err
	}
	authorizer := autorest.NewBearerAuthorizer(servicePrincipalToken)

	ipAddressesClient := network.NewPublicIPAddressesClient(credentials.SubscriptionID)
	ipAddressesClient.Authorizer = authorizer

	interfacesClient := network.NewInterfacesClient(credentials.SubscriptionID)
	interfacesClient.Authorizer = authorizer

	loadBalancersClient := network.NewLoadBalancersClient(credentials.SubscriptionID)
	loadBalancersClient.Authorizer = authorizer

	return &Clients{
		PublicIPAddressesClient: ipAddressesClient,
		InterfacesClient:        interfacesClient,
		LoadBalancersClient:     loadBalancersClient,
	}, nil
}
