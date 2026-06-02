package provider

import (
	"context"
	"maps"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

var (
	_ resource.Resource                = (*tenantResource)(nil)
	_ resource.ResourceWithConfigure   = (*tenantResource)(nil)
	_ resource.ResourceWithImportState = (*tenantResource)(nil)
)

// tenantResource implements the cozystack_tenant managed resource.
type tenantResource struct {
	client *client.Client
}

// tenantResourceModel is the resource model: the shared tenant attributes plus
// the optional create/update wait behaviour.
type tenantResourceModel struct {
	tenantModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

// waitConfig exposes the wait-for-ready settings to the shared CRUD helpers.
func (m *tenantResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

// NewTenantResource is the resource factory registered with the provider.
func NewTenantResource() resource.Resource {
	return &tenantResource{}
}

func (r *tenantResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_tenant"
}

func (r *tenantResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = tenantSchema()
}

func (r *tenantResource) Configure(
	_ context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	r.client = providerClient(req.ProviderData, &resp.Diagnostics)
}

func (r *tenantResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	createOrUpdate[tenantResourceModel](
		ctx, r.client, client.TenantResource(), true, req.Plan, &resp.State, &resp.Diagnostics,
	)
}

func (r *tenantResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	createOrUpdate[tenantResourceModel](
		ctx, r.client, client.TenantResource(), false, req.Plan, &resp.State, &resp.Diagnostics,
	)
}

func (r *tenantResource) Read(
	ctx context.Context,
	_ resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	readResource[tenantResourceModel](ctx, r.client, client.TenantResource(), &resp.State, &resp.Diagnostics)
}

func (r *tenantResource) Delete(
	ctx context.Context,
	_ resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	deleteResource[tenantResourceModel](ctx, r.client, client.TenantResource(), &resp.State, &resp.Diagnostics)
}

func (r *tenantResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	namespace, name, ok := parseImportID(req.ID)
	if !ok {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			`expected "namespace/name", for example "tenant-root/dev"`,
		)

		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(attrNamespace), namespace)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(attrName), name)...)
}

// tenantSchema returns the cozystack_tenant resource schema.
func tenantSchema() schema.Schema {
	attributes := map[string]schema.Attribute{}
	maps.Copy(attributes, tenantIdentityAttributes())
	maps.Copy(attributes, tenantSpecAttributes())
	maps.Copy(attributes, tenantStatusAttributes())
	maps.Copy(attributes, waitBehaviorAttributes())

	return schema.Schema{
		MarkdownDescription: "A Cozystack tenant: an isolated namespace under a parent tenant " +
			"in which other Cozystack applications are deployed.",
		Attributes: attributes,
	}
}

func tenantIdentityAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		attrID: schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Synthetic identifier in the form `namespace/name`.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		attrName: schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "Tenant name (`metadata.name`). Immutable.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
		attrNamespace: schema.StringAttribute{
			Required: true,
			MarkdownDescription: "Parent tenant namespace the tenant is created in " +
				"(the cluster root is `tenant-root`). Immutable.",
			PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
	}
}

func tenantSpecAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		attrHost: schema.StringAttribute{
			Optional:            true,
			Computed:            true,
			Default:             stringdefault.StaticString(""),
			MarkdownDescription: "Hostname used to access tenant services. Defaults to a subdomain of the parent host.",
		},
		attrEtcd:       boolToggle("Deploy a dedicated etcd cluster for the tenant."),
		attrMonitoring: boolToggle("Deploy a dedicated monitoring stack for the tenant."),
		attrIngress:    boolToggle("Deploy a dedicated ingress controller for the tenant."),
		"gateway": schema.BoolAttribute{
			Optional: true,
			MarkdownDescription: "Deploy a dedicated Gateway API controller (newer Cozystack only). " +
				"Tri-state: omitted when unset, so it is a no-op on clusters that do not support it.",
		},
		attrSeaweedfs: boolToggle("Deploy a dedicated SeaweedFS instance for the tenant."),
		"scheduling_class": schema.StringAttribute{
			Optional:            true,
			Computed:            true,
			Default:             stringdefault.StaticString(""),
			MarkdownDescription: "Name of a SchedulingClass CR applied to the tenant's workloads.",
		},
		"resource_quotas": schema.MapAttribute{
			Optional:            true,
			Computed:            true,
			ElementType:         types.StringType,
			Default:             mapdefault.StaticValue(types.MapValueMust(types.StringType, map[string]attr.Value{})),
			MarkdownDescription: "Resource quotas for the tenant, as quantity strings (for example `{cpu = \"4\"}`).",
		},
	}
}

// waitBehaviorAttributes returns the optional create/update wait attributes
// shared by every resource kind.
func waitBehaviorAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"wait_for_ready": schema.BoolAttribute{
			Optional:            true,
			Computed:            true,
			Default:             booldefault.StaticBool(false),
			MarkdownDescription: "Block on create/update until the tenant's `Ready` condition is true.",
		},
		"wait_timeout": schema.StringAttribute{
			Optional:            true,
			Computed:            true,
			Default:             stringdefault.StaticString("10m"),
			MarkdownDescription: "Maximum time to wait when `wait_for_ready` is set (Go duration, e.g. `10m`).",
		},
	}
}

func tenantStatusAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"status_namespace": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Namespace created for the tenant (`status.namespace`).",
		},
		attrReady: schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether the tenant's `Ready` condition is true.",
		},
		attrUID: schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Server-assigned object UID (`metadata.uid`).",
		},
		"version": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Deployed chart version (`status.version`).",
		},
	}
}

// boolToggle builds an optional/computed boolean spec flag defaulting to false.
func boolToggle(description string) schema.BoolAttribute {
	return schema.BoolAttribute{
		Optional:            true,
		Computed:            true,
		Default:             booldefault.StaticBool(false),
		MarkdownDescription: description,
	}
}
