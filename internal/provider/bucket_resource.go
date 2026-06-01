package provider

import (
	"context"
	"maps"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

var (
	_ resource.Resource                = (*bucketResource)(nil)
	_ resource.ResourceWithConfigure   = (*bucketResource)(nil)
	_ resource.ResourceWithImportState = (*bucketResource)(nil)
)

// bucketResource implements the cozystack_bucket managed resource.
type bucketResource struct {
	client *client.Client
}

// bucketResourceModel is the resource model: the shared bucket attributes plus
// the optional create/update wait behaviour.
type bucketResourceModel struct {
	bucketModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

// waitConfig exposes the wait-for-ready settings to the shared CRUD helpers.
func (m *bucketResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

// NewBucketResource is the resource factory registered with the provider.
func NewBucketResource() resource.Resource {
	return &bucketResource{}
}

func (r *bucketResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_bucket"
}

func (r *bucketResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = bucketSchema()
}

func (r *bucketResource) Configure(
	_ context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	r.client = providerClient(req.ProviderData, &resp.Diagnostics)
}

func (r *bucketResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	createOrUpdate[bucketResourceModel](
		ctx, r.client, client.BucketResource(), true, req.Plan, &resp.State, &resp.Diagnostics,
	)
}

func (r *bucketResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	createOrUpdate[bucketResourceModel](
		ctx, r.client, client.BucketResource(), false, req.Plan, &resp.State, &resp.Diagnostics,
	)
}

func (r *bucketResource) Read(
	ctx context.Context,
	_ resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	readResource[bucketResourceModel](ctx, r.client, client.BucketResource(), &resp.State, &resp.Diagnostics)
}

func (r *bucketResource) Delete(
	ctx context.Context,
	_ resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	deleteResource[bucketResourceModel](ctx, r.client, client.BucketResource(), &resp.State, &resp.Diagnostics)
}

func (r *bucketResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	namespace, name, ok := parseImportID(req.ID)
	if !ok {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			`expected "namespace/name", for example "tenant-root/assets"`,
		)

		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(attrNamespace), namespace)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(attrName), name)...)
}

// bucketSchema returns the cozystack_bucket resource schema.
func bucketSchema() schema.Schema {
	attributes := map[string]schema.Attribute{
		attrID: schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Synthetic identifier in the form `namespace/name`.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		attrName: schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "Bucket name (`metadata.name`). Immutable.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
		attrNamespace: schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "Tenant namespace the bucket is created in. Immutable.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
		"locking": schema.BoolAttribute{
			Optional:            true,
			Computed:            true,
			Default:             booldefault.StaticBool(false),
			MarkdownDescription: "Provision the bucket with object lock enabled.",
		},
		"storage_pool": schema.StringAttribute{
			Optional:            true,
			Computed:            true,
			Default:             stringdefault.StaticString(""),
			MarkdownDescription: "Selects a specific BucketClass by storage pool name.",
		},
		"users": schema.MapNestedAttribute{
			Optional:            true,
			MarkdownDescription: "Bucket users keyed by user name.",
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"readonly": schema.BoolAttribute{
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
						MarkdownDescription: "Whether the user has read-only access.",
					},
				},
			},
		},
		attrReady: schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether the bucket's `Ready` condition is true.",
		},
		attrChartVersion: schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Deployed chart version (`status.version`).",
		},
	}

	maps.Copy(attributes, waitBehaviorAttributes())

	return schema.Schema{
		MarkdownDescription: "A Cozystack S3-compatible bucket, provisioned inside a tenant namespace.",
		Attributes:          attributes,
	}
}
