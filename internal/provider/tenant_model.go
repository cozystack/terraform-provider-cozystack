package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

// tenantResourceModel maps the cozystack_tenant schema to Go types. The spec
// attributes mirror the json tags of the pinned tenant.ConfigSpec one-to-one.
type tenantResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Namespace       types.String `tfsdk:"namespace"`
	Host            types.String `tfsdk:"host"`
	Etcd            types.Bool   `tfsdk:"etcd"`
	Monitoring      types.Bool   `tfsdk:"monitoring"`
	Ingress         types.Bool   `tfsdk:"ingress"`
	Seaweedfs       types.Bool   `tfsdk:"seaweedfs"`
	SchedulingClass types.String `tfsdk:"scheduling_class"`
	ResourceQuotas  types.Map    `tfsdk:"resource_quotas"`
	StatusNamespace types.String `tfsdk:"status_namespace"`
	Ready           types.Bool   `tfsdk:"ready"`
	Version         types.String `tfsdk:"version"`
}

// expand converts the Terraform model into a client.Tenant ready to send.
func (m *tenantResourceModel) expand(ctx context.Context) (*client.Tenant, diag.Diagnostics) {
	var diags diag.Diagnostics

	quotas, qDiags := expandQuotas(ctx, m.ResourceQuotas)
	diags.Append(qDiags...)

	if diags.HasError() {
		return nil, diags
	}

	spec := map[string]any{
		"host":            m.Host.ValueString(),
		"etcd":            m.Etcd.ValueBool(),
		"monitoring":      m.Monitoring.ValueBool(),
		"ingress":         m.Ingress.ValueBool(),
		"seaweedfs":       m.Seaweedfs.ValueBool(),
		"schedulingClass": m.SchedulingClass.ValueString(),
		"resourceQuotas":  quotas,
	}

	return &client.Tenant{
		Name:      m.Name.ValueString(),
		Namespace: m.Namespace.ValueString(),
		Spec:      spec,
	}, diags
}

// flatten populates the Terraform model from the server view of a tenant.
func (m *tenantResourceModel) flatten(tenant *client.Tenant) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(tenant.Namespace + "/" + tenant.Name)
	m.Name = types.StringValue(tenant.Name)
	m.Namespace = types.StringValue(tenant.Namespace)
	m.Host = types.StringValue(specString(tenant.Spec, "host"))
	m.Etcd = types.BoolValue(specBool(tenant.Spec, "etcd"))
	m.Monitoring = types.BoolValue(specBool(tenant.Spec, "monitoring"))
	m.Ingress = types.BoolValue(specBool(tenant.Spec, "ingress"))
	m.Seaweedfs = types.BoolValue(specBool(tenant.Spec, "seaweedfs"))
	m.SchedulingClass = types.StringValue(specString(tenant.Spec, "schedulingClass"))

	quotas, qDiags := flattenQuotas(tenant.Spec["resourceQuotas"])
	diags.Append(qDiags...)

	m.ResourceQuotas = quotas

	m.StatusNamespace = types.StringValue(tenant.Status.Namespace)
	m.Ready = types.BoolValue(tenant.Status.Ready)
	m.Version = types.StringValue(tenant.Status.Version)

	return diags
}

// expandQuotas converts the resource_quotas map into a spec submap. A null or
// unknown map yields an empty (but non-nil) map so the spec key is always set.
func expandQuotas(ctx context.Context, value types.Map) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	out := map[string]any{}

	if value.IsNull() || value.IsUnknown() {
		return out, diags
	}

	elements := map[string]string{}
	diags.Append(value.ElementsAs(ctx, &elements, false)...)

	for key, quantity := range elements {
		out[key] = quantity
	}

	return out, diags
}

// flattenQuotas converts a spec resourceQuotas submap into a Terraform map.
func flattenQuotas(raw any) (types.Map, diag.Diagnostics) {
	elements := map[string]attr.Value{}

	if quotas, ok := raw.(map[string]any); ok {
		for key, quantity := range quotas {
			elements[key] = types.StringValue(quantityToString(quantity))
		}
	}

	return types.MapValue(types.StringType, elements)
}

// quantityToString renders a resource.Quantity (serialised as a string over the
// wire) without losing its canonical form.
func quantityToString(value any) string {
	if s, ok := value.(string); ok {
		return s
	}

	return fmt.Sprintf("%v", value)
}

func specString(spec map[string]any, key string) string {
	if v, ok := spec[key].(string); ok {
		return v
	}

	return ""
}

func specBool(spec map[string]any, key string) bool {
	if v, ok := spec[key].(bool); ok {
		return v
	}

	return false
}
