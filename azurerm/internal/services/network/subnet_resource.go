package network

import (
	"fmt"
	"log"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-11-01/network"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/azure"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/tf"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/clients"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/locks"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/services/network/common"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/services/network/parse"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/tf/pluginsdk"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/tf/validation"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/internal/timeouts"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/utils"
)

var SubnetResourceName = "azurerm_subnet"

func resourceSubnet() *pluginsdk.Resource {
	return &pluginsdk.Resource{
		Create: resourceSubnetCreate,
		Read:   resourceSubnetRead,
		Update: resourceSubnetUpdate,
		Delete: resourceSubnetDelete,
		Importer: pluginsdk.ImporterValidatingResourceId(func(id string) error {
			_, err := parse.SubnetID(id)
			return err
		}),

		Timeouts: &pluginsdk.ResourceTimeout{
			Create: pluginsdk.DefaultTimeout(30 * time.Minute),
			Read:   pluginsdk.DefaultTimeout(5 * time.Minute),
			Update: pluginsdk.DefaultTimeout(30 * time.Minute),
			Delete: pluginsdk.DefaultTimeout(30 * time.Minute),
		},

		Schema: map[string]*pluginsdk.Schema{
			"name": {
				Type:     pluginsdk.TypeString,
				Required: true,
				ForceNew: true,
			},

			"resource_group_name": azure.SchemaResourceGroupName(),

			"virtual_network_name": {
				Type:     pluginsdk.TypeString,
				Required: true,
				ForceNew: true,
			},

			"address_prefix": {
				Type:     pluginsdk.TypeString,
				Optional: true,
				Computed: true,
				// TODO Remove this in the next major version release
				Deprecated:   "Use the `address_prefixes` property instead.",
				ExactlyOneOf: []string{"address_prefix", "address_prefixes"},
			},

			"address_prefixes": {
				Type:     pluginsdk.TypeList,
				Optional: true,
				Computed: true,
				MinItems: 1,
				Elem: &pluginsdk.Schema{
					Type:         pluginsdk.TypeString,
					ValidateFunc: validation.StringIsNotEmpty,
				},
				ExactlyOneOf: []string{"address_prefix", "address_prefixes"},
			},

			"service_endpoints": common.SchemaSubnetServiceEndpoints(),

			"service_endpoint_policy_ids": common.SchemaSubnetServiceEndpointPolicyIds(),

			"delegation": common.SchemaSubnetDelegation(),

			"enforce_private_link_endpoint_network_policies": common.SchemaEnforcePrivateLinkEndpointNetworkPolicies(),

			"enforce_private_link_service_network_policies": common.SchemaEnforcePrivateLinkServiceNetworkPolicies(),
		},
	}
}

// TODO: refactor the create/flatten functions
func resourceSubnetCreate(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Network.SubnetsClient
	subscriptionId := meta.(*clients.Client).Account.SubscriptionId
	ctx, cancel := timeouts.ForCreateUpdate(meta.(*clients.Client).StopContext, d)
	defer cancel()

	log.Printf("[INFO] preparing arguments for Azure ARM Subnet creation.")

	id := parse.NewSubnetID(subscriptionId, d.Get("resource_group_name").(string), d.Get("virtual_network_name").(string), d.Get("name").(string))
	existing, err := client.Get(ctx, id.ResourceGroup, id.VirtualNetworkName, id.Name, "")
	if err != nil {
		if !utils.ResponseWasNotFound(existing.Response) {
			return fmt.Errorf("checking for presence of existing %s: %+v", id, err)
		}
	}

	if !utils.ResponseWasNotFound(existing.Response) {
		return tf.ImportAsExistsError("azurerm_subnet", id.ID())
	}

	locks.ByName(id.VirtualNetworkName, VirtualNetworkResourceName)
	defer locks.UnlockByName(id.VirtualNetworkName, VirtualNetworkResourceName)

	properties := network.SubnetPropertiesFormat{}
	if value, ok := d.GetOk("address_prefixes"); ok {
		var addressPrefixes []string
		for _, item := range value.([]interface{}) {
			addressPrefixes = append(addressPrefixes, item.(string))
		}
		properties.AddressPrefixes = &addressPrefixes
	}
	if value, ok := d.GetOk("address_prefix"); ok {
		addressPrefix := value.(string)
		properties.AddressPrefix = &addressPrefix
	}
	if properties.AddressPrefixes != nil && len(*properties.AddressPrefixes) == 1 {
		properties.AddressPrefix = &(*properties.AddressPrefixes)[0]
		properties.AddressPrefixes = nil
	}

	// To enable private endpoints you must disable the network policies for the subnet because
	// Network policies like network security groups are not supported by private endpoints.
	privateEndpointNetworkPolicies := d.Get("enforce_private_link_endpoint_network_policies").(bool)
	privateLinkServiceNetworkPolicies := d.Get("enforce_private_link_service_network_policies").(bool)
	properties.PrivateEndpointNetworkPolicies = network.VirtualNetworkPrivateEndpointNetworkPolicies(common.ExpandSubnetPrivateLinkNetworkPolicy(privateEndpointNetworkPolicies))
	properties.PrivateLinkServiceNetworkPolicies = network.VirtualNetworkPrivateLinkServiceNetworkPolicies(common.ExpandSubnetPrivateLinkNetworkPolicy(privateLinkServiceNetworkPolicies))

	serviceEndpointsRaw := d.Get("service_endpoints").([]interface{})
	properties.ServiceEndpoints = common.ExpandSubnetServiceEndpoints(serviceEndpointsRaw)

	serviceEndpointPoliciesRaw := d.Get("service_endpoint_policy_ids").(*pluginsdk.Set).List()
	properties.ServiceEndpointPolicies = common.ExpandSubnetServiceEndpointPolicies(serviceEndpointPoliciesRaw)

	delegationsRaw := d.Get("delegation").([]interface{})
	properties.Delegations = common.ExpandSubnetDelegation(delegationsRaw)

	subnet := network.Subnet{
		Name:                   utils.String(id.Name),
		SubnetPropertiesFormat: &properties,
	}

	future, err := client.CreateOrUpdate(ctx, id.ResourceGroup, id.VirtualNetworkName, id.Name, subnet)
	if err != nil {
		return fmt.Errorf("creating/updating %s: %+v", id, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("waiting for creation/update of %s: %+v", id, err)
	}

	d.SetId(id.ID())
	return resourceSubnetRead(d, meta)
}

func resourceSubnetUpdate(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Network.SubnetsClient
	ctx, cancel := timeouts.ForUpdate(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := parse.SubnetID(d.Id())
	if err != nil {
		return err
	}

	existing, err := client.Get(ctx, id.ResourceGroup, id.VirtualNetworkName, id.Name, "")
	if err != nil {
		return fmt.Errorf("retrieving %s: %+v", *id, err)
	}

	if existing.SubnetPropertiesFormat == nil {
		return fmt.Errorf("retrieving %s: `properties` was nil", *id)
	}

	// TODO: locking on the NSG/Route Table if applicable

	props := *existing.SubnetPropertiesFormat

	if d.HasChange("address_prefix") {
		props.AddressPrefix = utils.String(d.Get("address_prefix").(string))
	}

	if d.HasChange("address_prefixes") {
		addressPrefixesRaw := d.Get("address_prefixes").([]interface{})
		switch len(addressPrefixesRaw) {
		case 0:
			// Will never happen as the "MinItem: 1" constraint is set on "address_prefixes"
		case 1:
			// N->1: we shall insist on using the `AddressPrefix` and clear the `AddressPrefixes`.
			props.AddressPrefix = utils.String(addressPrefixesRaw[0].(string))
			props.AddressPrefixes = nil
		default:
			// 1->N: we shall insist on using the `AddressPrefixes` and clear the `AddressPrefix`. If both are set, service be confused and (currently) will only
			// return the `AddressPrefix` in response.
			props.AddressPrefixes = utils.ExpandStringSlice(addressPrefixesRaw)
			props.AddressPrefix = nil
		}
	}

	if d.HasChange("delegation") {
		delegationsRaw := d.Get("delegation").([]interface{})
		props.Delegations = common.ExpandSubnetDelegation(delegationsRaw)
	}

	if d.HasChange("enforce_private_link_endpoint_network_policies") {
		v := d.Get("enforce_private_link_endpoint_network_policies").(bool)
		props.PrivateEndpointNetworkPolicies = network.VirtualNetworkPrivateEndpointNetworkPolicies(common.ExpandSubnetPrivateLinkNetworkPolicy(v))
	}

	if d.HasChange("enforce_private_link_service_network_policies") {
		v := d.Get("enforce_private_link_service_network_policies").(bool)
		props.PrivateLinkServiceNetworkPolicies = network.VirtualNetworkPrivateLinkServiceNetworkPolicies(common.ExpandSubnetPrivateLinkNetworkPolicy(v))
	}

	if d.HasChange("service_endpoints") {
		serviceEndpointsRaw := d.Get("service_endpoints").([]interface{})
		props.ServiceEndpoints = common.ExpandSubnetServiceEndpoints(serviceEndpointsRaw)
	}

	if d.HasChange("service_endpoint_policy_ids") {
		serviceEndpointPoliciesRaw := d.Get("service_endpoint_policy_ids").(*pluginsdk.Set).List()
		props.ServiceEndpointPolicies = common.ExpandSubnetServiceEndpointPolicies(serviceEndpointPoliciesRaw)
	}

	subnet := network.Subnet{
		Name:                   utils.String(id.Name),
		SubnetPropertiesFormat: &props,
	}

	future, err := client.CreateOrUpdate(ctx, id.ResourceGroup, id.VirtualNetworkName, id.Name, subnet)
	if err != nil {
		return fmt.Errorf("updating %s: %+v", *id, err)
	}

	if err := future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("waiting for update of %s: %+v", *id, err)
	}

	return resourceSubnetRead(d, meta)
}

func resourceSubnetRead(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Network.SubnetsClient
	ctx, cancel := timeouts.ForRead(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := parse.SubnetID(d.Id())
	if err != nil {
		return err
	}

	resp, err := client.Get(ctx, id.ResourceGroup, id.VirtualNetworkName, id.Name, "")
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("retrieving %s: %+v", *id, err)
	}

	d.Set("name", id.Name)
	d.Set("virtual_network_name", id.VirtualNetworkName)
	d.Set("resource_group_name", id.ResourceGroup)

	if props := resp.SubnetPropertiesFormat; props != nil {
		d.Set("address_prefix", props.AddressPrefix)
		if props.AddressPrefixes == nil {
			if props.AddressPrefix != nil && len(*props.AddressPrefix) > 0 {
				d.Set("address_prefixes", []string{*props.AddressPrefix})
			} else {
				d.Set("address_prefixes", []string{})
			}
		} else {
			d.Set("address_prefixes", props.AddressPrefixes)
		}

		delegation := common.FlattenSubnetDelegation(props.Delegations)
		if err := d.Set("delegation", delegation); err != nil {
			return fmt.Errorf("Error flattening `delegation`: %+v", err)
		}

		d.Set("enforce_private_link_endpoint_network_policies", common.FlattenSubnetPrivateLinkNetworkPolicy(string(props.PrivateEndpointNetworkPolicies)))
		d.Set("enforce_private_link_service_network_policies", common.FlattenSubnetPrivateLinkNetworkPolicy(string(props.PrivateLinkServiceNetworkPolicies)))

		serviceEndpoints := common.FlattenSubnetServiceEndpoints(props.ServiceEndpoints)
		if err := d.Set("service_endpoints", serviceEndpoints); err != nil {
			return fmt.Errorf("Error setting `service_endpoints`: %+v", err)
		}

		serviceEndpointPolicies := common.FlattenSubnetServiceEndpointPolicies(props.ServiceEndpointPolicies)
		if err := d.Set("service_endpoint_policy_ids", serviceEndpointPolicies); err != nil {
			return fmt.Errorf("Error setting `service_endpoint_policy_ids`: %+v", err)
		}
	}

	return nil
}

func resourceSubnetDelete(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Network.SubnetsClient
	ctx, cancel := timeouts.ForDelete(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := parse.SubnetID(d.Id())
	if err != nil {
		return err
	}

	locks.ByName(id.VirtualNetworkName, VirtualNetworkResourceName)
	defer locks.UnlockByName(id.VirtualNetworkName, VirtualNetworkResourceName)

	locks.ByName(id.Name, SubnetResourceName)
	defer locks.UnlockByName(id.Name, SubnetResourceName)

	future, err := client.Delete(ctx, id.ResourceGroup, id.VirtualNetworkName, id.Name)
	if err != nil {
		return fmt.Errorf("deleting %s: %+v", *id, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("waiting for deletion of %s: %+v", *id, err)
	}

	return nil
}
