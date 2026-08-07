package provider

import (
	"context"

	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*tenantDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*tenantDataSource)(nil)
)

// tenantDataSource implements the cozystack_tenant data source.
type tenantDataSource struct {
	client *client.Client
}

// NewTenantDataSource is the data source factory registered with the provider.
func NewTenantDataSource() datasource.DataSource {
	return &tenantDataSource{}
}

func (d *tenantDataSource) Metadata(
	_ context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_tenant"
}

func (d *tenantDataSource) Configure(
	_ context.Context,
	req datasource.ConfigureRequest,
	resp *datasource.ConfigureResponse,
) {
	d.client = providerClient(req.ProviderData, &resp.Diagnostics)
}

func (d *tenantDataSource) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Read an existing Cozystack tenant by name and parent namespace.",
		Attributes: map[string]schema.Attribute{
			attrID:             schema.StringAttribute{Computed: true, MarkdownDescription: "Synthetic identifier `namespace/name`."},
			attrName:           schema.StringAttribute{Required: true, MarkdownDescription: "Tenant name (`metadata.name`)."},
			attrNamespace:      schema.StringAttribute{Required: true, MarkdownDescription: "Parent tenant namespace."},
			attrHost:           schema.StringAttribute{Computed: true, MarkdownDescription: "Hostname used to access tenant services."},
			attrEtcd:           schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether a dedicated etcd cluster is deployed."},
			attrMonitoring:     schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether a dedicated monitoring stack is deployed."},
			attrIngress:        schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether a dedicated ingress controller is deployed."},
			"gateway":          schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the tenant has a Gateway of its own; null when it never asked."},
			attrSeaweedfs:      schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether a dedicated SeaweedFS instance is deployed."},
			"scheduling_class": schema.StringAttribute{Computed: true, MarkdownDescription: "Name of the applied SchedulingClass CR."},
			"resource_quotas": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Resource quotas for the tenant, as quantity strings.",
			},
			"status_namespace": schema.StringAttribute{Computed: true, MarkdownDescription: "Namespace created for the tenant."},
			attrReady:          schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the tenant's `Ready` condition is true."},
			attrUID:            schema.StringAttribute{Computed: true, MarkdownDescription: "Server-assigned object UID (`metadata.uid`)."},
			"version":          schema.StringAttribute{Computed: true, MarkdownDescription: "Deployed chart version."},
		},
	}
}

func (d *tenantDataSource) Read(
	ctx context.Context,
	req datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	readDataSource[tenantModel](ctx, d.client, client.TenantResource(), req.Config, &resp.State, &resp.Diagnostics)
}
