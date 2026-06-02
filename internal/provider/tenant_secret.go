package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

// tenantSecretModel maps the cozystack_tenant_secret schema. Unlike the other
// kinds, a TenantSecret carries its payload in top-level data, not under spec.
type tenantSecretModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Namespace types.String `tfsdk:"namespace"`
	Type      types.String `tfsdk:"type"`
	Data      types.Map    `tfsdk:"data"`
	UID       types.String `tfsdk:"uid"`
}

// tenantSecretResourceModel adds the write-only data block (never persisted in
// state) used by the resource. The data source uses tenantSecretModel directly.
type tenantSecretResourceModel struct {
	tenantSecretModel

	DataWo        types.Map    `tfsdk:"data_wo"`
	DataWoVersion types.String `tfsdk:"data_wo_version"`
}

func (m *tenantSecretModel) toSecretObject(data map[string][]byte) *client.SecretObject {
	return &client.SecretObject{
		Name:      m.Name.ValueString(),
		Namespace: m.Namespace.ValueString(),
		Type:      m.Type.ValueString(),
		Data:      data,
	}
}

// fromSecretObject populates identity and metadata. The data is read back into
// state only when populateData is true (i.e. the user manages `data`, not the
// write-only `data_wo` — which must never land in state).
func (m *tenantSecretModel) fromSecretObject(ctx context.Context, obj *client.SecretObject, populateData bool) diag.Diagnostics {
	m.ID = types.StringValue(obj.Namespace + "/" + obj.Name)
	m.Name = types.StringValue(obj.Name)
	m.Namespace = types.StringValue(obj.Namespace)
	m.Type = types.StringValue(obj.Type)
	m.UID = types.StringValue(obj.UID)

	if !populateData {
		m.Data = types.MapNull(types.StringType)

		return nil
	}

	values := make(map[string]string, len(obj.Data))
	for key, value := range obj.Data {
		values[key] = string(value)
	}

	data, diags := types.MapValueFrom(ctx, types.StringType, values)
	m.Data = data

	return diags
}

// resolveSecretData picks the effective secret payload: the write-only `data_wo`
// (read from config) when set, otherwise the stored `data`. usedData reports
// which source won, so the caller knows whether to persist data in state.
func resolveSecretData(ctx context.Context, data, dataWo types.Map) (map[string][]byte, bool, diag.Diagnostics) {
	source := data
	usedData := true

	if !dataWo.IsNull() && !dataWo.IsUnknown() {
		source = dataWo
		usedData = false
	}

	var values map[string]string

	diags := source.ElementsAs(ctx, &values, false)

	decoded := make(map[string][]byte, len(values))
	for key, value := range values {
		decoded[key] = []byte(value)
	}

	return decoded, usedData, diags
}

// tenantSecretResource implements the cozystack_tenant_secret managed resource.
type tenantSecretResource struct {
	client *client.Client
}

var (
	_ resource.Resource                = (*tenantSecretResource)(nil)
	_ resource.ResourceWithConfigure   = (*tenantSecretResource)(nil)
	_ resource.ResourceWithImportState = (*tenantSecretResource)(nil)
)

// NewTenantSecretResource is the resource factory registered with the provider.
func NewTenantSecretResource() resource.Resource {
	return &tenantSecretResource{}
}

func (r *tenantSecretResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tenant_secret"
}

func (r *tenantSecretResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerClient(req.ProviderData, &resp.Diagnostics)
}

func (r *tenantSecretResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = rschema.Schema{
		MarkdownDescription: "A Cozystack TenantSecret: a Secret-shaped tenant resource whose payload " +
			"lives in `data` (not a spec).",
		Attributes: map[string]rschema.Attribute{
			attrID: rschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Synthetic identifier `namespace/name`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			attrName: rschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "TenantSecret name. Immutable.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			attrNamespace: rschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Tenant namespace. Immutable.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"type": rschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Secret type (defaults to `Opaque`).",
			},
			"data": rschema.MapAttribute{
				Optional:    true,
				Sensitive:   true,
				ElementType: types.StringType,
				MarkdownDescription: "Secret values keyed by name (plaintext in config, stored base64). " +
					"Persisted in state; use `data_wo` to keep secrets out of state.",
			},
			"data_wo": rschema.MapAttribute{
				Optional:    true,
				WriteOnly:   true,
				Sensitive:   true,
				ElementType: types.StringType,
				MarkdownDescription: "Write-only secret values: sent on apply but never stored in state. " +
					"Bump `data_wo_version` to push changes.",
			},
			"data_wo_version": rschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Change this to signal that `data_wo` was updated (write-only values are not diffed).",
			},
			attrUID: rschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Server-assigned object UID.",
			},
		},
	}
}

func (r *tenantSecretResource) persist(ctx context.Context, plan tfsdk.Plan, config tfsdk.Config, create bool, state *tfsdk.State, diags *diag.Diagnostics) {
	var model tenantSecretResourceModel

	diags.Append(plan.Get(ctx, &model)...)

	var dataWo types.Map

	diags.Append(config.GetAttribute(ctx, path.Root("data_wo"), &dataWo)...)

	if diags.HasError() {
		return
	}

	data, usedData, dDiags := resolveSecretData(ctx, model.Data, dataWo)
	diags.Append(dDiags...)

	if diags.HasError() {
		return
	}

	obj := model.toSecretObject(data)

	persist := r.client.UpdateSecretObject
	action := "update"

	if create {
		persist = r.client.CreateSecretObject
		action = "create"
	}

	result, err := persist(ctx, client.TenantSecretResource(), obj)
	if err != nil {
		diags.AddError("Unable to "+action+" TenantSecret", err.Error())

		return
	}

	model.DataWo = types.MapNull(types.StringType)
	diags.Append(model.fromSecretObject(ctx, &result, usedData)...)
	diags.Append(state.Set(ctx, &model)...)
}

func (r *tenantSecretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.persist(ctx, req.Plan, req.Config, true, &resp.State, &resp.Diagnostics)
}

func (r *tenantSecretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.persist(ctx, req.Plan, req.Config, false, &resp.State, &resp.Diagnostics)
}

func (r *tenantSecretResource) Read(ctx context.Context, _ resource.ReadRequest, resp *resource.ReadResponse) {
	var model tenantSecretResourceModel

	resp.Diagnostics.Append(resp.State.Get(ctx, &model)...)

	if resp.Diagnostics.HasError() {
		return
	}

	got, err := r.client.GetSecretObject(ctx, client.TenantSecretResource(), model.Namespace.ValueString(), model.Name.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError("Unable to read TenantSecret", err.Error())

		return
	}

	// Refresh data into state only when the user manages `data` (non-null in
	// prior state); secrets supplied via write-only `data_wo` stay out of state.
	populateData := !model.Data.IsNull()
	model.DataWo = types.MapNull(types.StringType)
	resp.Diagnostics.Append(model.fromSecretObject(ctx, &got, populateData)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func (r *tenantSecretResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model tenantSecretResourceModel

	resp.Diagnostics.Append(resp.State.Get(ctx, &model)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, client.TenantSecretResource(), model.Namespace.ValueString(), model.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to delete TenantSecret", err.Error())
	}
}

func (r *tenantSecretResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	namespace, name, ok := parseImportID(req.ID)
	if !ok {
		resp.Diagnostics.AddError("Invalid import ID", `expected "namespace/name"`)

		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(attrNamespace), namespace)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(attrName), name)...)
}

// tenantSecretDataSource implements the cozystack_tenant_secret data source.
type tenantSecretDataSource struct {
	client *client.Client
}

var (
	_ datasource.DataSource              = (*tenantSecretDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*tenantSecretDataSource)(nil)
)

// NewTenantSecretDataSource is the data source factory registered with the provider.
func NewTenantSecretDataSource() datasource.DataSource {
	return &tenantSecretDataSource{}
}

func (d *tenantSecretDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tenant_secret"
}

func (d *tenantSecretDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = providerClient(req.ProviderData, &resp.Diagnostics)
}

func (d *tenantSecretDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		MarkdownDescription: "Read a Cozystack TenantSecret by name and namespace.",
		Attributes: map[string]dsschema.Attribute{
			attrID:        dsschema.StringAttribute{Computed: true, MarkdownDescription: "Synthetic identifier `namespace/name`."},
			attrName:      dsschema.StringAttribute{Required: true, MarkdownDescription: "TenantSecret name."},
			attrNamespace: dsschema.StringAttribute{Required: true, MarkdownDescription: "Tenant namespace."},
			"type":        dsschema.StringAttribute{Computed: true, MarkdownDescription: "Secret type."},
			"data":        dsschema.MapAttribute{Computed: true, Sensitive: true, ElementType: types.StringType, MarkdownDescription: "Secret values keyed by name."},
			attrUID:       dsschema.StringAttribute{Computed: true, MarkdownDescription: "Server-assigned object UID."},
		},
	}
}

func (d *tenantSecretDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model tenantSecretModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)

	if resp.Diagnostics.HasError() {
		return
	}

	got, err := d.client.GetSecretObject(ctx, client.TenantSecretResource(), model.Namespace.ValueString(), model.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read TenantSecret", err.Error())

		return
	}

	resp.Diagnostics.Append(model.fromSecretObject(ctx, &got, true)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}
