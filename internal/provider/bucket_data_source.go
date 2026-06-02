package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

var (
	_ datasource.DataSource              = (*bucketDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*bucketDataSource)(nil)
)

// bucketDataSource implements the cozystack_bucket data source.
type bucketDataSource struct {
	client *client.Client
}

// NewBucketDataSource is the data source factory registered with the provider.
func NewBucketDataSource() datasource.DataSource {
	return &bucketDataSource{}
}

func (d *bucketDataSource) Metadata(
	_ context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_bucket"
}

func (d *bucketDataSource) Configure(
	_ context.Context,
	req datasource.ConfigureRequest,
	resp *datasource.ConfigureResponse,
) {
	d.client = providerClient(req.ProviderData, &resp.Diagnostics)
}

func (d *bucketDataSource) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Read an existing Cozystack bucket by name and namespace.",
		Attributes: map[string]schema.Attribute{
			attrID:         schema.StringAttribute{Computed: true, MarkdownDescription: "Synthetic identifier `namespace/name`."},
			attrName:       schema.StringAttribute{Required: true, MarkdownDescription: "Bucket name."},
			attrNamespace:  schema.StringAttribute{Required: true, MarkdownDescription: "Tenant namespace."},
			"locking":      schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether object lock is enabled."},
			"storage_pool": schema.StringAttribute{Computed: true, MarkdownDescription: "BucketClass storage pool name."},
			"users": schema.MapNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Bucket users keyed by user name.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"readonly": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the user has read-only access."},
					},
				},
			},
			"credentials": schema.MapNestedAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "S3 credentials per user.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"bucket_name": schema.StringAttribute{Computed: true, MarkdownDescription: "Backing bucket name."},
						"endpoint":    schema.StringAttribute{Computed: true, MarkdownDescription: "S3 endpoint URL."},
						"region":      schema.StringAttribute{Computed: true, MarkdownDescription: "S3 region."},
						"access_key":  schema.StringAttribute{Computed: true, MarkdownDescription: "S3 access key ID."},
						"secret_key":  schema.StringAttribute{Computed: true, MarkdownDescription: "S3 secret access key."},
					},
				},
			},
			attrReady:        schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the bucket's `Ready` condition is true."},
			attrChartVersion: schema.StringAttribute{Computed: true, MarkdownDescription: "Deployed chart version."},
		},
	}
}

func (d *bucketDataSource) Read(
	ctx context.Context,
	req datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	readDataSource[bucketModel](ctx, d.client, client.BucketResource(), req.Config, &resp.State, &resp.Diagnostics)
}
