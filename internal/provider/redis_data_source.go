package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

var (
	_ datasource.DataSource              = (*redisDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*redisDataSource)(nil)
)

// redisDataSource implements the cozystack_redis data source.
type redisDataSource struct {
	client *client.Client
}

// NewRedisDataSource is the data source factory registered with the provider.
func NewRedisDataSource() datasource.DataSource {
	return &redisDataSource{}
}

func (d *redisDataSource) Metadata(
	_ context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_redis"
}

func (d *redisDataSource) Configure(
	_ context.Context,
	req datasource.ConfigureRequest,
	resp *datasource.ConfigureResponse,
) {
	d.client = providerClient(req.ProviderData, &resp.Diagnostics)
}

func (d *redisDataSource) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Read an existing Cozystack Redis instance by name and namespace.",
		Attributes: map[string]schema.Attribute{
			attrID:             schema.StringAttribute{Computed: true, MarkdownDescription: "Synthetic identifier `namespace/name`."},
			attrName:           schema.StringAttribute{Required: true, MarkdownDescription: "Redis instance name."},
			attrNamespace:      schema.StringAttribute{Required: true, MarkdownDescription: "Tenant namespace."},
			attrReplicas:       schema.Int64Attribute{Computed: true, MarkdownDescription: "Number of Redis replicas."},
			attrSize:           schema.StringAttribute{Computed: true, MarkdownDescription: "Persistent volume size."},
			"storage_class":    schema.StringAttribute{Computed: true, MarkdownDescription: "StorageClass used to store the data."},
			attrExternal:       schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether external access is enabled."},
			attrVersion:        schema.StringAttribute{Computed: true, MarkdownDescription: "Redis major version."},
			"auth_enabled":     schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether authentication is enabled."},
			"resources_preset": schema.StringAttribute{Computed: true, MarkdownDescription: "Sizing preset."},
			attrResources: schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Explicit CPU and memory per replica, when set.",
				Attributes: map[string]schema.Attribute{
					attrCPU:    schema.StringAttribute{Computed: true, MarkdownDescription: "CPU available to each replica."},
					attrMemory: schema.StringAttribute{Computed: true, MarkdownDescription: "Memory available to each replica."},
				},
			},
			attrReady:       schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the instance's `Ready` condition is true."},
			"chart_version": schema.StringAttribute{Computed: true, MarkdownDescription: "Deployed chart version."},
		},
	}
}

func (d *redisDataSource) Read(
	ctx context.Context,
	req datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	readDataSource[redisModel](ctx, d.client, client.RedisResource(), req.Config, &resp.State, &resp.Diagnostics)
}
