package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

// kafkaModel maps the cozystack_kafka schema to Go types. Per-topic opaque
// tuning (`config`) is not managed.
type kafkaModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Namespace    types.String `tfsdk:"namespace"`
	External     types.Bool   `tfsdk:"external"`
	Topics       types.List   `tfsdk:"topics"`
	Kafka        types.Object `tfsdk:"kafka"`
	Zookeeper    types.Object `tfsdk:"zookeeper"`
	Ready        types.Bool   `tfsdk:"ready"`
	ChartVersion types.String `tfsdk:"chart_version"`
}

type kafkaResourceModel struct {
	kafkaModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

func (m *kafkaResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

func (m *kafkaModel) identity() (string, string) {
	return m.Namespace.ValueString(), m.Name.ValueString()
}

func kafkaTopicObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"name":       types.StringType,
		"partitions": types.Int64Type,
		"replicas":   types.Int64Type,
	}
}

func kafkaBrokerObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"replicas":         types.Int64Type,
		"resources":        types.ObjectType{AttrTypes: resourcesObjectType()},
		"resources_preset": types.StringType,
		"size":             types.StringType,
		"storage_class":    types.StringType,
	}
}

type kafkaTopicData struct {
	Name       types.String `tfsdk:"name"`
	Partitions types.Int64  `tfsdk:"partitions"`
	Replicas   types.Int64  `tfsdk:"replicas"`
}

type kafkaBrokerData struct {
	Replicas        types.Int64  `tfsdk:"replicas"`
	Resources       types.Object `tfsdk:"resources"`
	ResourcesPreset types.String `tfsdk:"resources_preset"`
	Size            types.String `tfsdk:"size"`
	StorageClass    types.String `tfsdk:"storage_class"`
}

func (m *kafkaModel) expand(ctx context.Context) (*client.Application, diag.Diagnostics) {
	var diags diag.Diagnostics

	topics, tDiags := expandObjectList(ctx, m.Topics, func(t kafkaTopicData) map[string]any {
		return map[string]any{
			"name":       t.Name.ValueString(),
			"partitions": t.Partitions.ValueInt64(),
			"replicas":   t.Replicas.ValueInt64(),
		}
	})
	diags.Append(tDiags...)

	kafka, kDiags := expandKafkaBroker(ctx, m.Kafka)
	diags.Append(kDiags...)

	zookeeper, zDiags := expandKafkaBroker(ctx, m.Zookeeper)
	diags.Append(zDiags...)

	if diags.HasError() {
		return nil, diags
	}

	spec := map[string]any{
		attrExternal: m.External.ValueBool(),
		"topics":     topics,
		"kafka":      kafka,
		"zookeeper":  zookeeper,
	}

	return &client.Application{
		Name:      m.Name.ValueString(),
		Namespace: m.Namespace.ValueString(),
		Spec:      spec,
	}, diags
}

func expandKafkaBroker(ctx context.Context, value types.Object) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	out := map[string]any{}

	if value.IsNull() || value.IsUnknown() {
		return out, diags
	}

	var data kafkaBrokerData

	diags.Append(value.As(ctx, &data, basetypes.ObjectAsOptions{})...)

	if diags.HasError() {
		return out, diags
	}

	resources, rDiags := expandResources(ctx, data.Resources)
	diags.Append(rDiags...)

	out["replicas"] = data.Replicas.ValueInt64()
	out["resources"] = resources
	out["resourcesPreset"] = data.ResourcesPreset.ValueString()
	out["size"] = data.Size.ValueString()
	out[specStorageClass] = data.StorageClass.ValueString()

	return out, diags
}

func (m *kafkaModel) flatten(app *client.Application) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(app.Namespace + "/" + app.Name)
	m.Name = types.StringValue(app.Name)
	m.Namespace = types.StringValue(app.Namespace)
	m.External = types.BoolValue(specBool(app.Spec, attrExternal))

	topics, tDiags := flattenObjectList(app.Spec["topics"], kafkaTopicObjectType(), func(topic map[string]any) map[string]attr.Value {
		name, _ := topic["name"].(string)

		return map[string]attr.Value{
			"name":       types.StringValue(name),
			"partitions": types.Int64Value(anyToInt64(topic["partitions"])),
			"replicas":   types.Int64Value(anyToInt64(topic["replicas"])),
		}
	})
	diags.Append(tDiags...)

	m.Topics = topics

	kafka, kDiags := flattenKafkaBroker(app.Spec["kafka"])
	diags.Append(kDiags...)

	m.Kafka = kafka

	zookeeper, zDiags := flattenKafkaBroker(app.Spec["zookeeper"])
	diags.Append(zDiags...)

	m.Zookeeper = zookeeper

	m.Ready = types.BoolValue(app.Status.Ready)
	m.ChartVersion = types.StringValue(app.Status.Version)

	return diags
}

func flattenKafkaBroker(raw any) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics

	broker, ok := raw.(map[string]any)
	if !ok {
		return types.ObjectNull(kafkaBrokerObjectType()), diags
	}

	resources, rDiags := flattenResources(broker["resources"])
	diags.Append(rDiags...)

	object, oDiags := types.ObjectValue(kafkaBrokerObjectType(), map[string]attr.Value{
		"replicas":         types.Int64Value(anyToInt64(broker["replicas"])),
		"resources":        resources,
		"resources_preset": types.StringValue(specString(broker, "resourcesPreset")),
		"size":             types.StringValue(specString(broker, "size")),
		"storage_class":    types.StringValue(specString(broker, specStorageClass)),
	})
	diags.Append(oDiags...)

	return object, diags
}
