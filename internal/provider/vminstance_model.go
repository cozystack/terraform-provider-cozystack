package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

// vminstanceModel maps the cozystack_vminstance schema to Go types. The
// deprecated subnets list is not managed (use networks instead).
type vminstanceModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Namespace         types.String `tfsdk:"namespace"`
	External          types.Bool   `tfsdk:"external"`
	ExternalMethod    types.String `tfsdk:"external_method"`
	ExternalPorts     types.List   `tfsdk:"external_ports"`
	ExternalAllowICMP types.Bool   `tfsdk:"external_allow_icmp"`
	RunStrategy       types.String `tfsdk:"run_strategy"`
	InstanceType      types.String `tfsdk:"instance_type"`
	InstanceProfile   types.String `tfsdk:"instance_profile"`
	Disks             types.List   `tfsdk:"disks"`
	Networks          types.List   `tfsdk:"networks"`
	Gpus              types.List   `tfsdk:"gpus"`
	CPUModel          types.String `tfsdk:"cpu_model"`
	Resources         types.Object `tfsdk:"resources"`
	SSHKeys           types.List   `tfsdk:"ssh_keys"`
	CloudInit         types.String `tfsdk:"cloud_init"`
	CloudInitSeed     types.String `tfsdk:"cloud_init_seed"`
	Ready             types.Bool   `tfsdk:"ready"`
	ChartVersion      types.String `tfsdk:"chart_version"`
	UID               types.String `tfsdk:"uid"`
	IPAddress         types.String `tfsdk:"ip_address"`
	IPAddresses       types.List   `tfsdk:"ip_addresses"`
}

type vminstanceResourceModel struct {
	vminstanceModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

func (m *vminstanceResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

func (m *vminstanceModel) identity() (string, string) {
	return m.Namespace.ValueString(), m.Name.ValueString()
}

func vmDiskObjectType() map[string]attr.Type {
	return map[string]attr.Type{"name": types.StringType, "bus": types.StringType}
}

func vmNetworkObjectType() map[string]attr.Type {
	return map[string]attr.Type{"name": types.StringType}
}

func vmGpuObjectType() map[string]attr.Type {
	return map[string]attr.Type{"name": types.StringType}
}

func vmResourcesObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		attrCPU:    types.StringType,
		attrMemory: types.StringType,
		"sockets":  types.StringType,
	}
}

type vmDiskData struct {
	Name types.String `tfsdk:"name"`
	Bus  types.String `tfsdk:"bus"`
}

type vmNameData struct {
	Name types.String `tfsdk:"name"`
}

type vmResourcesData struct {
	CPU     types.String `tfsdk:"cpu"`
	Memory  types.String `tfsdk:"memory"`
	Sockets types.String `tfsdk:"sockets"`
}

func (m *vminstanceModel) expand(ctx context.Context) (*client.Application, diag.Diagnostics) {
	var diags diag.Diagnostics

	ports, portDiags := expandIntList(ctx, m.ExternalPorts)
	diags.Append(portDiags...)

	disks, dDiags := expandObjectList(ctx, m.Disks, func(d vmDiskData) map[string]any {
		entry := map[string]any{"name": d.Name.ValueString()}
		if bus := d.Bus.ValueString(); bus != "" {
			entry["bus"] = bus
		}

		return entry
	})
	diags.Append(dDiags...)

	networks, nDiags := expandObjectList(ctx, m.Networks, func(n vmNameData) map[string]any {
		return map[string]any{"name": n.Name.ValueString()}
	})
	diags.Append(nDiags...)

	gpus, gDiags := expandObjectList(ctx, m.Gpus, func(g vmNameData) map[string]any {
		return map[string]any{"name": g.Name.ValueString()}
	})
	diags.Append(gDiags...)

	sshKeys, kDiags := expandStringList(ctx, m.SSHKeys)
	diags.Append(kDiags...)

	resources, rDiags := expandVMResources(ctx, m.Resources)
	diags.Append(rDiags...)

	if diags.HasError() {
		return nil, diags
	}

	spec := map[string]any{
		attrExternal:        m.External.ValueBool(),
		"externalMethod":    m.ExternalMethod.ValueString(),
		"externalPorts":     ports,
		"externalAllowICMP": m.ExternalAllowICMP.ValueBool(),
		"runStrategy":       m.RunStrategy.ValueString(),
		"instanceType":      m.InstanceType.ValueString(),
		"instanceProfile":   m.InstanceProfile.ValueString(),
		"disks":             disks,
		"networks":          networks,
		"gpus":              gpus,
		"cpuModel":          m.CPUModel.ValueString(),
		attrResources:       resources,
		"sshKeys":           sshKeys,
		"cloudInit":         m.CloudInit.ValueString(),
		"cloudInitSeed":     m.CloudInitSeed.ValueString(),
	}

	return &client.Application{
		Name:      m.Name.ValueString(),
		Namespace: m.Namespace.ValueString(),
		Spec:      spec,
	}, diags
}

func expandVMResources(ctx context.Context, value types.Object) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	out := map[string]any{}

	if value.IsNull() || value.IsUnknown() {
		return out, diags
	}

	var data vmResourcesData

	diags.Append(value.As(ctx, &data, basetypes.ObjectAsOptions{})...)

	if diags.HasError() {
		return out, diags
	}

	if cpu := data.CPU.ValueString(); cpu != "" {
		out[attrCPU] = cpu
	}

	if memory := data.Memory.ValueString(); memory != "" {
		out[attrMemory] = memory
	}

	if sockets := data.Sockets.ValueString(); sockets != "" {
		out["sockets"] = sockets
	}

	return out, diags
}

func (m *vminstanceModel) flatten(app *client.Application) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(app.Namespace + "/" + app.Name)
	m.Name = types.StringValue(app.Name)
	m.Namespace = types.StringValue(app.Namespace)
	m.External = types.BoolValue(specBool(app.Spec, attrExternal))
	m.ExternalMethod = types.StringValue(specString(app.Spec, "externalMethod"))
	m.ExternalAllowICMP = types.BoolValue(specBool(app.Spec, "externalAllowICMP"))
	m.RunStrategy = types.StringValue(specString(app.Spec, "runStrategy"))
	m.InstanceType = types.StringValue(specString(app.Spec, "instanceType"))
	m.InstanceProfile = types.StringValue(specString(app.Spec, "instanceProfile"))
	m.CPUModel = types.StringValue(specString(app.Spec, "cpuModel"))
	m.CloudInit = types.StringValue(specString(app.Spec, "cloudInit"))
	m.CloudInitSeed = types.StringValue(specString(app.Spec, "cloudInitSeed"))
	m.ExternalPorts = flattenIntList(app.Spec["externalPorts"])

	disks, dDiags := flattenObjectList(app.Spec["disks"], vmDiskObjectType(), func(disk map[string]any) map[string]attr.Value {
		name, _ := disk["name"].(string)
		bus, _ := disk["bus"].(string)

		busValue := types.StringNull()
		if bus != "" {
			busValue = types.StringValue(bus)
		}

		return map[string]attr.Value{"name": types.StringValue(name), "bus": busValue}
	})
	diags.Append(dDiags...)

	m.Disks = disks

	networks, nDiags := flattenObjectList(app.Spec["networks"], vmNetworkObjectType(), vmNameValue)
	diags.Append(nDiags...)

	m.Networks = networks

	gpus, gDiags := flattenObjectList(app.Spec["gpus"], vmGpuObjectType(), vmNameValue)
	diags.Append(gDiags...)

	m.Gpus = gpus

	sshKeys, kDiags := flattenStringList(app.Spec["sshKeys"])
	diags.Append(kDiags...)

	m.SSHKeys = sshKeys
	m.Resources = flattenVMResources(app.Spec[attrResources])

	m.Ready = types.BoolValue(app.Status.Ready)
	m.ChartVersion = types.StringValue(app.Status.Version)
	m.UID = types.StringValue(app.UID)

	return diags
}

// readOutputs reads the running VM's IP addresses from the backing KubeVirt
// VirtualMachineInstance (`vm-instance-<name>`). Addresses appear only once the
// guest is up, so an absent VMI or one without an address leaves them null.
func (m *vminstanceModel) readOutputs(ctx context.Context, api *client.Client) diag.Diagnostics {
	var diags diag.Diagnostics

	m.IPAddress = types.StringNull()
	m.IPAddresses = types.ListNull(types.StringType)

	namespace, name := m.identity()

	addresses, found, err := api.GetVMIAddresses(ctx, namespace, "vm-instance-"+name)
	if err != nil {
		diags.AddError("Unable to read VMInstance addresses", err.Error())

		return diags
	}

	if !found || len(addresses) == 0 {
		return diags
	}

	m.IPAddress = types.StringValue(addresses[0])
	m.IPAddresses = stringListOrNull(stringsToAny(addresses))

	return diags
}

func vmNameValue(entry map[string]any) map[string]attr.Value {
	name, _ := entry["name"].(string)

	return map[string]attr.Value{"name": types.StringValue(name)}
}

func flattenVMResources(raw any) types.Object {
	resources, ok := raw.(map[string]any)
	if !ok || len(resources) == 0 {
		return types.ObjectNull(vmResourcesObjectType())
	}

	return types.ObjectValueMust(vmResourcesObjectType(), map[string]attr.Value{
		attrCPU:    vmResourceField(resources, attrCPU),
		attrMemory: vmResourceField(resources, attrMemory),
		"sockets":  vmResourceField(resources, "sockets"),
	})
}

func vmResourceField(resources map[string]any, key string) attr.Value {
	value, ok := resources[key].(string)
	if !ok || value == "" {
		return types.StringNull()
	}

	return types.StringValue(value)
}
