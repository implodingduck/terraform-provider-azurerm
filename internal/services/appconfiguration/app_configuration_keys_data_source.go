package appconfiguration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/Azure/go-autorest/autorest"
	"github.com/hashicorp/terraform-provider-azurerm/helpers/azure"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/appconfiguration/parse"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tags"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/utils"
)

type KeysDataSource struct{}

var _ sdk.DataSource = KeysDataSource{}

type KeyDataSourceModel struct {
	Key               string                 `tfschema:"key"`
	ContentType       string                 `tfschema:"content_type"`
	Etag              string                 `tfschema:"etag"`
	Label             string                 `tfschema:"label"`
	Value             string                 `tfschema:"value"`
	Locked            bool                   `tfschema:"locked"`
	Tags              map[string]interface{} `tfschema:"tags"`
	Type              string                 `tfschema:"type"`
	VaultKeyReference string                 `tfschema:"vault_key_reference"`
}

type KeysDataSourceModel struct {
	ConfigurationStoreId string               `tfschema:"configuration_store_id"`
	Key                  string               `tfschema:"key"`
	Label                string               `tfschema:"label"`
	Items                []KeyDataSourceModel `tfschema:"items"`
}

func (k KeysDataSource) Arguments() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"configuration_store_id": {
			Type:         pluginsdk.TypeString,
			Required:     true,
			ValidateFunc: azure.ValidateResourceID,
		},
		"key": {
			Type:     pluginsdk.TypeString,
			Optional: true,
			Default:  "",
		},
		"label": {
			Type:     pluginsdk.TypeString,
			Optional: true,
			Default:  "",
		},
	}
}

func (k KeysDataSource) Attributes() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"items": {
			Type:     pluginsdk.TypeList,
			Computed: true,
			Elem: &pluginsdk.Resource{
				Schema: map[string]*pluginsdk.Schema{
					"key": {
						Type:     pluginsdk.TypeString,
						Computed: true,
					},
					"label": {
						Type:     pluginsdk.TypeString,
						Computed: true,
					},
					"content_type": {
						Type:     pluginsdk.TypeString,
						Computed: true,
					},
					"etag": {
						Type:     pluginsdk.TypeString,
						Computed: true,
					},
					"value": {
						Type:     pluginsdk.TypeString,
						Computed: true,
					},
					"locked": {
						Type:     pluginsdk.TypeBool,
						Computed: true,
					},
					"type": {
						Type:     pluginsdk.TypeString,
						Computed: true,
					},
					"vault_key_reference": {
						Type:     pluginsdk.TypeString,
						Computed: true,
					},
					"tags": tags.SchemaDataSource(),
				},
			},
		},
	}
}

func (k KeysDataSource) ModelObject() interface{} {
	return &KeysDataSourceModel{}
}

func (k KeysDataSource) ResourceType() string {
	return "azurerm_app_configuration_keys"
}

func (k KeysDataSource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 5 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			var model KeysDataSourceModel
			if err := metadata.Decode(&model); err != nil {
				return err
			}

			decodedKey, err := url.QueryUnescape(model.Key)
			if err != nil {
				return fmt.Errorf("while decoding key of resource ID: %+v", err)
			}

			id := parse.AppConfigurationKeyId{
				ConfigurationStoreId: model.ConfigurationStoreId,
				Key:                  decodedKey,
				Label:                model.Label,
			}

			client, err := metadata.Client.AppConfiguration.DataPlaneClient(ctx, model.ConfigurationStoreId)
			if client == nil {
				return fmt.Errorf("building data plane client: app configuration %q was not found", model.ConfigurationStoreId)
			}
			if err != nil {
				return err
			}

			iter, err := client.GetKeyValuesComplete(ctx, decodedKey, model.Label, "", "", []string{})
			if err != nil {
				if v, ok := err.(autorest.DetailedError); ok {
					if utils.ResponseWasNotFound(autorest.Response{Response: v.Response}) {
						return fmt.Errorf("key %s was not found", decodedKey)
					}
				} else {
					return fmt.Errorf("while checking for key's %q existence: %+v", decodedKey, err)
				}
				return fmt.Errorf("while checking for key's %q existence: %+v", decodedKey, err)
			}

			for iter.NotDone() {
				kv := iter.Value()
				var krmodel KeyDataSourceModel
				krmodel.Key = *kv.Key
				krmodel.Label = *kv.Label
				if contentType := utils.NormalizeNilableString(kv.ContentType); contentType != VaultKeyContentType {
					krmodel.Type = KeyTypeKV
					krmodel.ContentType = contentType
					krmodel.Value = utils.NormalizeNilableString(kv.Value)
				} else {
					var ref VaultKeyReference
					refBytes := []byte(utils.NormalizeNilableString(kv.Value))
					err := json.Unmarshal(refBytes, &ref)
					if err != nil {
						return fmt.Errorf("while unmarshalling vault reference: %+v", err)
					}

					krmodel.Type = KeyTypeVault
					krmodel.VaultKeyReference = ref.URI
					krmodel.ContentType = VaultKeyContentType
					krmodel.Value = ref.URI
				}

				if kv.Locked != nil {
					krmodel.Locked = *kv.Locked
				}
				krmodel.Etag = utils.NormalizeNilableString(kv.Etag)
				if id.Label == "" {
					// We set an empty label as %00 in the resource ID
					// Otherwise it breaks the ID parsing logic
					id.Label = "%00"
				}
				model.Items = append(model.Items, krmodel)
				if err := iter.NextWithContext(ctx); err != nil {
					return fmt.Errorf("fetching keys for %q: %+v", id, err)
				}
			}
			metadata.SetID(id)
			return metadata.Encode(&model)
		},
	}
}
