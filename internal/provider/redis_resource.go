package provider

import (
	"context"
	"maps"

	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*redisResource)(nil)
	_ resource.ResourceWithConfigure   = (*redisResource)(nil)
	_ resource.ResourceWithImportState = (*redisResource)(nil)
)

// redisResource implements the cozystack_redis managed resource.
type redisResource struct {
	client *client.Client
}

// redisResourceModel is the resource model: the shared redis attributes plus the
// optional create/update wait behaviour.
type redisResourceModel struct {
	redisModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

// waitConfig exposes the wait-for-ready settings to the shared CRUD helpers.
func (m *redisResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

// NewRedisResource is the resource factory registered with the provider.
func NewRedisResource() resource.Resource {
	return &redisResource{}
}

func (r *redisResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_redis"
}

func (r *redisResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = redisSchema()
}

func (r *redisResource) Configure(
	_ context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	r.client = providerClient(req.ProviderData, &resp.Diagnostics)
}

func (r *redisResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	createOrUpdate[redisResourceModel](
		ctx, r.client, client.RedisResource(), true, req.Plan, &resp.State, &resp.Diagnostics,
	)
}

func (r *redisResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	createOrUpdate[redisResourceModel](
		ctx, r.client, client.RedisResource(), false, req.Plan, &resp.State, &resp.Diagnostics,
	)
}

func (r *redisResource) Read(
	ctx context.Context,
	_ resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	readResource[redisResourceModel](ctx, r.client, client.RedisResource(), &resp.State, &resp.Diagnostics)
}

func (r *redisResource) Delete(
	ctx context.Context,
	_ resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	deleteResource[redisResourceModel](ctx, r.client, client.RedisResource(), &resp.State, &resp.Diagnostics)
}

func (r *redisResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	namespace, name, ok := parseImportID(req.ID)
	if !ok {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			`expected "namespace/name", for example "tenant-root/cache"`,
		)

		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(attrNamespace), namespace)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(attrName), name)...)
}

// redisSchema returns the cozystack_redis resource schema.
func redisSchema() schema.Schema {
	attributes := map[string]schema.Attribute{}
	maps.Copy(attributes, redisIdentityAttributes())
	maps.Copy(attributes, redisSpecAttributes())
	maps.Copy(attributes, redisStatusAttributes())
	maps.Copy(attributes, waitBehaviorAttributes())

	return schema.Schema{
		MarkdownDescription: "A Cozystack managed Redis instance, deployed inside a tenant namespace.",
		Attributes:          attributes,
	}
}

func redisIdentityAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		attrID: schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Synthetic identifier in the form `namespace/name`.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		attrName: schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "Redis instance name (`metadata.name`). Immutable.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
		attrNamespace: schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "Tenant namespace the instance is created in. Immutable.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
	}
}

func redisSpecAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		attrReplicas: schema.Int64Attribute{
			Optional:            true,
			Computed:            true,
			Default:             int64default.StaticInt64(2),
			MarkdownDescription: "Number of Redis replicas.",
		},
		attrSize: schema.StringAttribute{
			Optional:            true,
			Computed:            true,
			Default:             stringdefault.StaticString("1Gi"),
			MarkdownDescription: "Persistent volume size for application data (quantity, e.g. `1Gi`).",
		},
		attrStorageClass: storageClassAttribute(""),
		attrExternal: schema.BoolAttribute{
			Optional:            true,
			Computed:            true,
			Default:             booldefault.StaticBool(false),
			MarkdownDescription: "Enable external access from outside the cluster.",
		},
		attrVersion: schema.StringAttribute{
			Optional:            true,
			Computed:            true,
			Default:             stringdefault.StaticString("v8"),
			Validators:          []validator.String{stringvalidator.OneOf("v7", "v8")},
			MarkdownDescription: "Redis major version to deploy (`v7` or `v8`).",
		},
		attrAuthEnabled: schema.BoolAttribute{
			Optional:            true,
			Computed:            true,
			Default:             booldefault.StaticBool(true),
			MarkdownDescription: "Generate a password and require authentication.",
		},
		attrResourcesPreset: schema.StringAttribute{
			Optional:            true,
			Computed:            true,
			Default:             stringdefault.StaticString("t1.nano"),
			MarkdownDescription: "Sizing preset applied when `resources` is omitted (e.g. `t1.nano`).",
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

func redisStatusAttributes() map[string]schema.Attribute {
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
