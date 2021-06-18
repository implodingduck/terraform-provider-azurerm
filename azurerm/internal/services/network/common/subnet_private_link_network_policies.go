package common

import (
	"strings"

	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/tf/pluginsdk"
)

func SchemaEnforcePrivateLinkEndpointNetworkPolicies() *pluginsdk.Schema {
	return &pluginsdk.Schema{
		Type:     pluginsdk.TypeBool,
		Optional: true,
		Default:  false,
	}
}

func SchemaEnforcePrivateLinkServiceNetworkPolicies() *pluginsdk.Schema {
	return &pluginsdk.Schema{
		Type:     pluginsdk.TypeBool,
		Optional: true,
		Default:  false,
	}
}

func ExpandSubnetPrivateLinkNetworkPolicy(enabled bool) string {
	// This is strange logic, but to get the schema to make sense for the end user
	// I exposed it with the same name that the Azure CLI does to be consistent
	// between the tool sets, which means true == Disabled.
	if enabled {
		return "Disabled"
	}

	return "Enabled"
}

func FlattenSubnetPrivateLinkNetworkPolicy(input string) bool {
	// This is strange logic, but to get the schema to make sense for the end user
	// I exposed it with the same name that the Azure CLI does to be consistent
	// between the tool sets, which means true == Disabled.
	return strings.EqualFold(input, "Disabled")
}
