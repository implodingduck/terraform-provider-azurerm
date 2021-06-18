package common

import (
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-11-01/network"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/tf/pluginsdk"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/tf/validation"
)

func SchemaSubnetDelegation() *pluginsdk.Schema {
	return &pluginsdk.Schema{
		Type:       pluginsdk.TypeList,
		Optional:   true,
		ConfigMode: pluginsdk.SchemaConfigModeAttr,
		Elem: &pluginsdk.Resource{
			Schema: map[string]*pluginsdk.Schema{
				"name": {
					Type:     pluginsdk.TypeString,
					Required: true,
				},
				"service_delegation": {
					Type:       pluginsdk.TypeList,
					Required:   true,
					MaxItems:   1,
					ConfigMode: pluginsdk.SchemaConfigModeAttr,
					Elem: &pluginsdk.Resource{
						Schema: map[string]*pluginsdk.Schema{
							"name": {
								Type:     pluginsdk.TypeString,
								Required: true,
								ValidateFunc: validation.StringInSlice([]string{
									"Microsoft.ApiManagement/service",
									"Microsoft.AzureCosmosDB/clusters",
									"Microsoft.BareMetal/AzureVMware",
									"Microsoft.BareMetal/CrayServers",
									"Microsoft.Batch/batchAccounts",
									"Microsoft.ContainerInstance/containerGroups",
									"Microsoft.Databricks/workspaces",
									"Microsoft.DBforMySQL/flexibleServers",
									"Microsoft.DBforMySQL/serversv2",
									"Microsoft.DBforPostgreSQL/flexibleServers",
									"Microsoft.DBforPostgreSQL/serversv2",
									"Microsoft.DBforPostgreSQL/singleServers",
									"Microsoft.HardwareSecurityModules/dedicatedHSMs",
									"Microsoft.Kusto/clusters",
									"Microsoft.Logic/integrationServiceEnvironments",
									"Microsoft.MachineLearningServices/workspaces",
									"Microsoft.Netapp/volumes",
									"Microsoft.Network/managedResolvers",
									"Microsoft.PowerPlatform/vnetaccesslinks",
									"Microsoft.ServiceFabricMesh/networks",
									"Microsoft.Sql/managedInstances",
									"Microsoft.Sql/servers",
									"Microsoft.StreamAnalytics/streamingJobs",
									"Microsoft.Synapse/workspaces",
									"Microsoft.Web/hostingEnvironments",
									"Microsoft.Web/serverFarms",
								}, false),
							},

							"actions": {
								Type:       pluginsdk.TypeList,
								Optional:   true,
								ConfigMode: pluginsdk.SchemaConfigModeAttr,
								Elem: &pluginsdk.Schema{
									Type: pluginsdk.TypeString,
									ValidateFunc: validation.StringInSlice([]string{
										"Microsoft.Network/networkinterfaces/*",
										"Microsoft.Network/virtualNetworks/subnets/action",
										"Microsoft.Network/virtualNetworks/subnets/join/action",
										"Microsoft.Network/virtualNetworks/subnets/prepareNetworkPolicies/action",
										"Microsoft.Network/virtualNetworks/subnets/unprepareNetworkPolicies/action",
									}, false),
								},
							},
						},
					},
				},
			},
		},
	}
}

func ExpandSubnetDelegation(input []interface{}) *[]network.Delegation {
	retDelegations := make([]network.Delegation, 0)

	for _, deleValue := range input {
		deleData := deleValue.(map[string]interface{})
		deleName := deleData["name"].(string)
		srvDelegations := deleData["service_delegation"].([]interface{})
		srvDelegation := srvDelegations[0].(map[string]interface{})
		srvName := srvDelegation["name"].(string)
		srvActions := srvDelegation["actions"].([]interface{})

		retSrvActions := make([]string, 0)
		for _, srvAction := range srvActions {
			srvActionData := srvAction.(string)
			retSrvActions = append(retSrvActions, srvActionData)
		}

		retDelegation := network.Delegation{
			Name: &deleName,
			ServiceDelegationPropertiesFormat: &network.ServiceDelegationPropertiesFormat{
				ServiceName: &srvName,
				Actions:     &retSrvActions,
			},
		}

		retDelegations = append(retDelegations, retDelegation)
	}

	return &retDelegations
}

func FlattenSubnetDelegation(delegations *[]network.Delegation) []interface{} {
	if delegations == nil {
		return []interface{}{}
	}

	retDeles := make([]interface{}, 0)

	for _, dele := range *delegations {
		retDele := make(map[string]interface{})
		if v := dele.Name; v != nil {
			retDele["name"] = *v
		}

		svcDeles := make([]interface{}, 0)
		svcDele := make(map[string]interface{})
		if props := dele.ServiceDelegationPropertiesFormat; props != nil {
			if v := props.ServiceName; v != nil {
				svcDele["name"] = *v
			}

			if v := props.Actions; v != nil {
				svcDele["actions"] = *v
			}
		}

		svcDeles = append(svcDeles, svcDele)

		retDele["service_delegation"] = svcDeles

		retDeles = append(retDeles, retDele)
	}

	return retDeles
}
