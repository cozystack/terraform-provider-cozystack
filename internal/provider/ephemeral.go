package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	ephschema "github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

// Ephemeral resources fetch secrets at apply time without persisting them in
// state — e.g. a child cluster's kubeconfig for provider chaining, or a tenant
// secret's data — closing the "secrets in state" gap.

type kubeconfigEphemeralModel struct {
	Name       types.String `tfsdk:"name"`
	Namespace  types.String `tfsdk:"namespace"`
	Kubeconfig types.String `tfsdk:"kubeconfig"`
}

type kubeconfigEphemeral struct {
	client *client.Client
}

var (
	_ ephemeral.EphemeralResource              = (*kubeconfigEphemeral)(nil)
	_ ephemeral.EphemeralResourceWithConfigure = (*kubeconfigEphemeral)(nil)
)

// NewKubeconfigEphemeral is the ephemeral resource factory for cluster kubeconfigs.
func NewKubeconfigEphemeral() ephemeral.EphemeralResource {
	return &kubeconfigEphemeral{}
}

func (e *kubeconfigEphemeral) Metadata(_ context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes"
}

func (e *kubeconfigEphemeral) Configure(_ context.Context, req ephemeral.ConfigureRequest, resp *ephemeral.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	e.client = providerClient(req.ProviderData, &resp.Diagnostics)
}

func (e *kubeconfigEphemeral) Schema(_ context.Context, _ ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = ephschema.Schema{
		MarkdownDescription: "The admin kubeconfig of a Cozystack-managed Kubernetes cluster, fetched at " +
			"apply time without ever being stored in state. Use it to configure a downstream " +
			"`kubernetes`/`helm` provider for the cluster.",
		Attributes: map[string]ephschema.Attribute{
			attrName:      ephschema.StringAttribute{Required: true, MarkdownDescription: "Kubernetes cluster name."},
			attrNamespace: ephschema.StringAttribute{Required: true, MarkdownDescription: "Tenant namespace."},
			"kubeconfig":  ephschema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Admin kubeconfig."},
		},
	}
}

func (e *kubeconfigEphemeral) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var model kubeconfigEphemeralModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)

	if resp.Diagnostics.HasError() {
		return
	}

	secret := "kubernetes-" + model.Name.ValueString() + "-admin-kubeconfig"

	data, found, err := e.client.GetSecretData(ctx, model.Namespace.ValueString(), secret)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read kubeconfig", err.Error())

		return
	}

	conf, ok := data["super-admin.conf"]
	if !found || !ok {
		resp.Diagnostics.AddError("Kubeconfig not available yet",
			"The cluster's admin kubeconfig Secret does not exist yet; ensure the cluster is ready.")

		return
	}

	model.Kubeconfig = types.StringValue(string(conf))
	resp.Diagnostics.Append(resp.Result.Set(ctx, &model)...)
}

type tenantSecretEphemeralModel struct {
	Name      types.String `tfsdk:"name"`
	Namespace types.String `tfsdk:"namespace"`
	Type      types.String `tfsdk:"type"`
	Data      types.Map    `tfsdk:"data"`
}

type tenantSecretEphemeral struct {
	client *client.Client
}

var (
	_ ephemeral.EphemeralResource              = (*tenantSecretEphemeral)(nil)
	_ ephemeral.EphemeralResourceWithConfigure = (*tenantSecretEphemeral)(nil)
)

// NewTenantSecretEphemeral is the ephemeral resource factory for tenant secrets.
func NewTenantSecretEphemeral() ephemeral.EphemeralResource {
	return &tenantSecretEphemeral{}
}

func (e *tenantSecretEphemeral) Metadata(_ context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tenant_secret"
}

func (e *tenantSecretEphemeral) Configure(_ context.Context, req ephemeral.ConfigureRequest, resp *ephemeral.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	e.client = providerClient(req.ProviderData, &resp.Diagnostics)
}

func (e *tenantSecretEphemeral) Schema(_ context.Context, _ ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = ephschema.Schema{
		MarkdownDescription: "A TenantSecret's data, fetched at apply time without being stored in state.",
		Attributes: map[string]ephschema.Attribute{
			attrName:      ephschema.StringAttribute{Required: true, MarkdownDescription: "TenantSecret name."},
			attrNamespace: ephschema.StringAttribute{Required: true, MarkdownDescription: "Tenant namespace."},
			"type":        ephschema.StringAttribute{Computed: true, MarkdownDescription: "Secret type."},
			"data":        ephschema.MapAttribute{Computed: true, Sensitive: true, ElementType: types.StringType, MarkdownDescription: "Secret values keyed by name."},
		},
	}
}

func (e *tenantSecretEphemeral) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var model tenantSecretEphemeralModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)

	if resp.Diagnostics.HasError() {
		return
	}

	got, err := e.client.GetSecretObject(ctx, client.TenantSecretResource(), model.Namespace.ValueString(), model.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read TenantSecret", err.Error())

		return
	}

	values := make(map[string]string, len(got.Data))
	for key, value := range got.Data {
		values[key] = string(value)
	}

	data, diags := types.MapValueFrom(ctx, types.StringType, values)
	resp.Diagnostics.Append(diags...)

	model.Type = types.StringValue(got.Type)
	model.Data = data
	resp.Diagnostics.Append(resp.Result.Set(ctx, &model)...)
}
