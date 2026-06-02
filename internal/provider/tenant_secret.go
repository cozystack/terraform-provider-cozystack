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

func (m *tenantSecretModel) toSecretObject(ctx context.Context) (*client.SecretObject, diag.Diagnostics) {
	var data map[string]string

	diags := m.Data.ElementsAs(ctx, &data, false)

	decoded := make(map[string][]byte, len(data))
	for key, value := range data {
		decoded[key] = []byte(value)
	}

	return &client.SecretObject{
		Name:      m.Name.ValueString(),
		Namespace: m.Namespace.ValueString(),
		Type:      m.Type.ValueString(),
		Data:      decoded,
	}, diags
}

func (m *tenantSecretModel) fromSecretObject(ctx context.Context, obj *client.SecretObject) diag.Diagnostics {
	m.ID = types.StringValue(obj.Namespace + "/" + obj.Name)
	m.Name = types.StringValue(obj.Name)
	m.Namespace = types.StringValue(obj.Namespace)
	m.Type = types.StringValue(obj.Type)
	m.UID = types.StringValue(obj.UID)

	values := make(map[string]string, len(obj.Data))
	for key, value := range obj.Data {
		values[key] = string(value)
	}

	data, diags := types.MapValueFrom(ctx, types.StringType, values)
	m.Data = data

	return diags
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
				Required:            true,
				Sensitive:           true,
				ElementType:         types.StringType,
				MarkdownDescription: "Secret values keyed by name (plaintext; stored base64-encoded).",
			},
			attrUID: rschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Server-assigned object UID.",
			},
		},
	}
}

func (r *tenantSecretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model tenantSecretModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)

	if resp.Diagnostics.HasError() {
		return
	}

	obj, diags := model.toSecretObject(ctx)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateSecretObject(ctx, client.TenantSecretResource(), obj)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create TenantSecret", err.Error())

		return
	}

	resp.Diagnostics.Append(model.fromSecretObject(ctx, &created)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func (r *tenantSecretResource) Read(ctx context.Context, _ resource.ReadRequest, resp *resource.ReadResponse) {
	var model tenantSecretModel

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

	resp.Diagnostics.Append(model.fromSecretObject(ctx, &got)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func (r *tenantSecretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var model tenantSecretModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)

	if resp.Diagnostics.HasError() {
		return
	}

	obj, diags := model.toSecretObject(ctx)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateSecretObject(ctx, client.TenantSecretResource(), obj)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update TenantSecret", err.Error())

		return
	}

	resp.Diagnostics.Append(model.fromSecretObject(ctx, &updated)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func (r *tenantSecretResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model tenantSecretModel

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

	resp.Diagnostics.Append(model.fromSecretObject(ctx, &got)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}
