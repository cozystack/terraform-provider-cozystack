package provider

import (
	"context"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/vminstance"
	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func fullVMInstanceModel() vminstanceModel {
	disk := types.ObjectValueMust(vmDiskObjectType(), map[string]attr.Value{
		"name": types.StringValue("os"),
		"bus":  types.StringNull(),
	})

	return vminstanceModel{
		Name:              types.StringValue("vm"),
		Namespace:         types.StringValue("tenant-root"),
		External:          types.BoolValue(false),
		ExternalMethod:    types.StringValue("PortList"),
		ExternalPorts:     types.ListValueMust(types.Int64Type, []attr.Value{types.Int64Value(22)}),
		ExternalAllowICMP: types.BoolValue(true),
		RunStrategy:       types.StringValue("Always"),
		InstanceType:      types.StringValue("u1.medium"),
		InstanceProfile:   types.StringValue("ubuntu"),
		Disks:             types.ListValueMust(types.ObjectType{AttrTypes: vmDiskObjectType()}, []attr.Value{disk}),
		Networks:          types.ListNull(types.ObjectType{AttrTypes: vmNetworkObjectType()}),
		Gpus:              types.ListNull(types.ObjectType{AttrTypes: vmGpuObjectType()}),
		CPUModel:          types.StringValue(""),
		Resources:         types.ObjectNull(vmResourcesObjectType()),
		SSHKeys:           types.ListNull(types.StringType),
		CloudInit:         types.StringValue(""),
		CloudInitSeed:     types.StringValue(""),
	}
}

func TestVMInstanceExpand_PortsAndDisks(t *testing.T) {
	t.Parallel()

	model := fullVMInstanceModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	ports, _ := got.Spec["externalPorts"].([]any)
	if len(ports) != 1 || ports[0] != int64(22) {
		t.Errorf("externalPorts = %v, want [22]", ports)
	}

	disks, _ := got.Spec["disks"].([]any)
	disk, _ := disks[0].(map[string]any)
	if disk["name"] != "os" {
		t.Errorf("disk = %v, want name=os", disk)
	}

	if _, ok := disk["bus"]; ok {
		t.Errorf("disk has bus for unset bus: %v", disk)
	}
}

func TestVMInstanceFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "vm",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"external":        true,
			"externalMethod":  "WholeIP",
			"externalPorts":   []any{int64(22), int64(443)},
			"runStrategy":     "Always",
			"instanceType":    "u1.large",
			"instanceProfile": "ubuntu",
			"disks":           []any{map[string]any{"name": "os", "bus": "virtio"}},
			"resources":       map[string]any{"cpu": "2", "memory": "4Gi"},
			"sshKeys":         []any{"ssh-ed25519 AAAA"},
			"cloudInit":       "#cloud-config",
		},
		Status: client.ApplicationStatus{Version: "1.0.0", Ready: true},
	}

	var model vminstanceModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.InstanceType.ValueString() != "u1.large" {
		t.Errorf("instance_type = %q, want u1.large", model.InstanceType.ValueString())
	}

	if len(model.ExternalPorts.Elements()) != 2 {
		t.Errorf("external_ports = %v, want two elements", model.ExternalPorts.Elements())
	}

	resources := model.Resources.Attributes()
	if resources["cpu"].(types.String).ValueString() != "2" {
		t.Errorf("resources.cpu = %v, want 2", resources["cpu"])
	}

	if !resources["sockets"].IsNull() {
		t.Errorf("resources.sockets populated, want null")
	}
}

func TestVMInstanceExpandKeysMatchConfigSpec(t *testing.T) {
	t.Parallel()

	model := fullVMInstanceModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	emitted := make(map[string]bool, len(got.Spec))
	for key := range got.Spec {
		emitted[key] = true
	}

	assertSpecCoverage(t, emitted, vminstance.ConfigSpec{}, "subnets")
}
