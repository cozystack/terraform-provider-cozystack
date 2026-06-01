package provider

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
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
	if req.ProviderData == nil {
		return
	}

	api, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("expected *client.Client, got %T", req.ProviderData),
		)

		return
	}

	r.client = api
}

func (r *tenantResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	r.applyPlan(ctx, req.Plan, &resp.State, &resp.Diagnostics, "create", r.client.CreateTenant)
}

func (r *tenantResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	r.applyPlan(ctx, req.Plan, &resp.State, &resp.Diagnostics, "update", r.client.UpdateTenant)
}

// applyPlan expands the planned model, persists it through persist, and writes
// the server view back to state. It is shared by Create and Update.
func (r *tenantResource) applyPlan(
	ctx context.Context,
	plan tfsdk.Plan,
	state *tfsdk.State,
	diags *diag.Diagnostics,
	action string,
	persist func(context.Context, *client.Tenant) (client.Tenant, error),
) {
	var model tenantResourceModel

	diags.Append(plan.Get(ctx, &model)...)

	if diags.HasError() {
		return
	}

	tenant, expandDiags := model.expand(ctx)
	diags.Append(expandDiags...)

	if diags.HasError() {
		return
	}

	result, err := persist(ctx, tenant)
	if err != nil {
		diags.AddError("Unable to "+action+" tenant", err.Error())

		return
	}

	diags.Append(model.flatten(&result)...)
	diags.Append(state.Set(ctx, &model)...)
}

func (r *tenantResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var model tenantResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)

	if resp.Diagnostics.HasError() {
		return
	}

	got, err := r.client.GetTenant(ctx, model.Namespace.ValueString(), model.Name.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError("Unable to read tenant", err.Error())

		return
	}

	resp.Diagnostics.Append(model.flatten(&got)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func (r *tenantResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var model tenantResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteTenant(ctx, model.Namespace.ValueString(), model.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to delete tenant", err.Error())
	}
}

func (r *tenantResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	namespace, name, ok := parseTenantImportID(req.ID)
	if !ok {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			`expected "namespace/name", for example "tenant-root/dev"`,
		)

		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("namespace"), namespace)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
}

// parseTenantImportID splits a "namespace/name" import identifier, reporting
// false when it is malformed.
func parseTenantImportID(id string) (string, string, bool) {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}

	return parts[0], parts[1], true
}

// tenantSchema returns the cozystack_tenant resource schema.
func tenantSchema() schema.Schema {
	attributes := map[string]schema.Attribute{}
	maps.Copy(attributes, tenantIdentityAttributes())
	maps.Copy(attributes, tenantSpecAttributes())
	maps.Copy(attributes, tenantStatusAttributes())

	return schema.Schema{
		MarkdownDescription: "A Cozystack tenant: an isolated namespace under a parent tenant " +
			"in which other Cozystack applications are deployed.",
		Attributes: attributes,
	}
}

func tenantIdentityAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Synthetic identifier in the form `namespace/name`.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		"name": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "Tenant name (`metadata.name`). Immutable.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
		"namespace": schema.StringAttribute{
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
		attrSeaweedfs:  boolToggle("Deploy a dedicated SeaweedFS instance for the tenant."),
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

func tenantStatusAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"status_namespace": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Namespace created for the tenant (`status.namespace`).",
		},
		"ready": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether the tenant's `Ready` condition is true.",
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
