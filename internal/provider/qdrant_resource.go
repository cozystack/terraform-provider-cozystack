package provider

import (
	"context"
	"maps"

	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*qdrantResource)(nil)
	_ resource.ResourceWithConfigure   = (*qdrantResource)(nil)
	_ resource.ResourceWithImportState = (*qdrantResource)(nil)
)

// qdrantResource implements the cozystack_qdrant managed resource.
type qdrantResource struct {
	client *client.Client
}

// qdrantResourceModel is the resource model: the shared qdrant attributes plus
// the optional create/update wait behaviour.
type qdrantResourceModel struct {
	qdrantModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

// waitConfig exposes the wait-for-ready settings to the shared CRUD helpers.
func (m *qdrantResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

// NewQdrantResource is the resource factory registered with the provider.
func NewQdrantResource() resource.Resource {
	return &qdrantResource{}
}

func (r *qdrantResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_qdrant"
}

func (r *qdrantResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = qdrantSchema()
}

func (r *qdrantResource) Configure(
	_ context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	r.client = providerClient(req.ProviderData, &resp.Diagnostics)
}

func (r *qdrantResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	createOrUpdate[qdrantResourceModel](
		ctx, r.client, client.QdrantResource(), true, req.Plan, &resp.State, &resp.Diagnostics,
	)
}

func (r *qdrantResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	createOrUpdate[qdrantResourceModel](
		ctx, r.client, client.QdrantResource(), false, req.Plan, &resp.State, &resp.Diagnostics,
	)
}

func (r *qdrantResource) Read(
	ctx context.Context,
	_ resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	readResource[qdrantResourceModel](ctx, r.client, client.QdrantResource(), &resp.State, &resp.Diagnostics)
}

func (r *qdrantResource) Delete(
	ctx context.Context,
	_ resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	deleteResource[qdrantResourceModel](ctx, r.client, client.QdrantResource(), &resp.State, &resp.Diagnostics)
}

func (r *qdrantResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	namespace, name, ok := parseImportID(req.ID)
	if !ok {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			`expected "namespace/name", for example "tenant-root/vectors"`,
		)

		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(attrNamespace), namespace)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(attrName), name)...)
}

// qdrantSchema returns the cozystack_qdrant resource schema.
func qdrantSchema() schema.Schema {
	attributes := map[string]schema.Attribute{}
	maps.Copy(attributes, qdrantIdentityAttributes())
	maps.Copy(attributes, qdrantSpecAttributes())
	maps.Copy(attributes, qdrantStatusAttributes())
	maps.Copy(attributes, waitBehaviorAttributes())

	return schema.Schema{
		MarkdownDescription: "A Cozystack managed Qdrant vector database, deployed inside a tenant namespace.",
		Attributes:          attributes,
	}
}

func qdrantIdentityAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		attrID: schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Synthetic identifier in the form `namespace/name`.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		attrName: schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "Qdrant instance name (`metadata.name`). Immutable.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
		attrNamespace: schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "Tenant namespace the instance is created in. Immutable.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
	}
}

func qdrantSpecAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		attrReplicas: schema.Int64Attribute{
			Optional:            true,
			Computed:            true,
			Default:             int64default.StaticInt64(1),
			MarkdownDescription: "Number of Qdrant replicas (cluster mode is enabled when > 1).",
		},
		attrSize: schema.StringAttribute{
			Optional:            true,
			Computed:            true,
			Default:             stringdefault.StaticString("10Gi"),
			MarkdownDescription: "Persistent volume size for vector data (quantity, e.g. `10Gi`).",
		},
		attrStorageClass: schema.StringAttribute{
			Optional:            true,
			Computed:            true,
			Default:             stringdefault.StaticString(""),
			MarkdownDescription: "StorageClass used to store the data.",
		},
		attrExternal: schema.BoolAttribute{
			Optional:            true,
			Computed:            true,
			Default:             booldefault.StaticBool(false),
			MarkdownDescription: "Enable external access from outside the cluster.",
		},
		attrResourcesPreset: schema.StringAttribute{
			Optional:            true,
			Computed:            true,
			Default:             stringdefault.StaticString("t1.small"),
			MarkdownDescription: "Sizing preset applied when `resources` is omitted (e.g. `t1.small`).",
		},
		attrResources: schema.SingleNestedAttribute{
			Optional: true,
			MarkdownDescription: "Explicit CPU and memory per replica. Overrides `resources_preset` " +
				"for any field set.",
			Attributes: map[string]schema.Attribute{
				attrCPU: schema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "CPU available to each replica (quantity, e.g. `500m`).",
				},
				attrMemory: schema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "Memory available to each replica (quantity, e.g. `512Mi`).",
				},
			},
		},
	}
}

func qdrantStatusAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		attrReady: schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether the instance's `Ready` condition is true.",
		},
		attrUID: schema.StringAttribute{Computed: true, MarkdownDescription: "Server-assigned object UID (`metadata.uid`)."},
		attrChartVersion: schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Deployed chart version (`status.version`).",
		},
	}
}
