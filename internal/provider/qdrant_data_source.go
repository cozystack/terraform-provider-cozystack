package provider

import (
	"context"

	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

var (
	_ datasource.DataSource              = (*qdrantDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*qdrantDataSource)(nil)
)

// qdrantDataSource implements the cozystack_qdrant data source.
type qdrantDataSource struct {
	client *client.Client
}

// NewQdrantDataSource is the data source factory registered with the provider.
func NewQdrantDataSource() datasource.DataSource {
	return &qdrantDataSource{}
}

func (d *qdrantDataSource) Metadata(
	_ context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_qdrant"
}

func (d *qdrantDataSource) Configure(
	_ context.Context,
	req datasource.ConfigureRequest,
	resp *datasource.ConfigureResponse,
) {
	d.client = providerClient(req.ProviderData, &resp.Diagnostics)
}

func (d *qdrantDataSource) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Read an existing Cozystack Qdrant instance by name and namespace.",
		Attributes: map[string]schema.Attribute{
			attrID:              schema.StringAttribute{Computed: true, MarkdownDescription: "Synthetic identifier `namespace/name`."},
			attrName:            schema.StringAttribute{Required: true, MarkdownDescription: "Qdrant instance name."},
			attrNamespace:       schema.StringAttribute{Required: true, MarkdownDescription: "Tenant namespace."},
			attrReplicas:        schema.Int64Attribute{Computed: true, MarkdownDescription: "Number of Qdrant replicas."},
			attrSize:            schema.StringAttribute{Computed: true, MarkdownDescription: "Persistent volume size."},
			attrStorageClass:    schema.StringAttribute{Computed: true, MarkdownDescription: "StorageClass used to store the data."},
			attrExternal:        schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether external access is enabled."},
			attrResourcesPreset: schema.StringAttribute{Computed: true, MarkdownDescription: "Sizing preset."},
			attrResources: schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Explicit CPU and memory per replica, when set.",
				Attributes: map[string]schema.Attribute{
					attrCPU:    schema.StringAttribute{Computed: true, MarkdownDescription: "CPU available to each replica."},
					attrMemory: schema.StringAttribute{Computed: true, MarkdownDescription: "Memory available to each replica."},
				},
			},
			attrReady:        schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the instance's `Ready` condition is true."},
			attrUID:          schema.StringAttribute{Computed: true, MarkdownDescription: "Server-assigned object UID (`metadata.uid`)."},
			attrChartVersion: schema.StringAttribute{Computed: true, MarkdownDescription: "Deployed chart version."},
		},
	}
}

func (d *qdrantDataSource) Read(
	ctx context.Context,
	req datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	readDataSource[qdrantModel](ctx, d.client, client.QdrantResource(), req.Config, &resp.State, &resp.Diagnostics)
}
