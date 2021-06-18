package common

import (
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-11-01/network"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/services/network/validate"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/tf/pluginsdk"
)

func SchemaSubnetServiceEndpointPolicyIds() *pluginsdk.Schema {
	return &pluginsdk.Schema{
		Type:     pluginsdk.TypeSet,
		Optional: true,
		MinItems: 1,
		Elem: &pluginsdk.Schema{
			Type:         pluginsdk.TypeString,
			ValidateFunc: validate.SubnetServiceEndpointStoragePolicyID,
		},
	}
}

func ExpandSubnetServiceEndpointPolicies(input []interface{}) *[]network.ServiceEndpointPolicy {
	output := make([]network.ServiceEndpointPolicy, 0)
	for _, policy := range input {
		policy := policy.(string)
		output = append(output, network.ServiceEndpointPolicy{ID: &policy})
	}
	return &output
}

func FlattenSubnetServiceEndpointPolicies(input *[]network.ServiceEndpointPolicy) []interface{} {
	if input == nil {
		return nil
	}

	var output []interface{}
	for _, policy := range *input {
		id := ""
		if policy.ID != nil {
			id = *policy.ID
		}
		output = append(output, id)
	}
	return output
}
