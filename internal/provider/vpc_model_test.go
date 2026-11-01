package provider

import (
	"context"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/vpc"
	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func fullVPCModel() vpcModel {
	subnet := types.ObjectValueMust(vpcSubnetObjectType(), map[string]attr.Value{
		"name": types.StringValue("web"),
		"cidr": types.StringValue("10.0.0.0/24"),
	})
	peer := types.ObjectValueMust(vpcPeerObjectType(), map[string]attr.Value{
		"tenant_namespace": types.StringValue("tenant-foo"),
		"vpc_name":         types.StringValue("net"),
	})
	route := types.ObjectValueMust(vpcRouteObjectType(), map[string]attr.Value{
		"cidr":        types.StringValue("0.0.0.0/0"),
		"next_hop_ip": types.StringValue("10.0.0.1"),
	})

	return vpcModel{
		Name:      types.StringValue("net"),
		Namespace: types.StringValue("tenant-root"),
		Subnets:   types.ListValueMust(types.ObjectType{AttrTypes: vpcSubnetObjectType()}, []attr.Value{subnet}),
		Peers:     types.ListValueMust(types.ObjectType{AttrTypes: vpcPeerObjectType()}, []attr.Value{peer}),
		Routes:    types.ListValueMust(types.ObjectType{AttrTypes: vpcRouteObjectType()}, []attr.Value{route}),
	}
}

func TestVPCExpand_Subnets(t *testing.T) {
	t.Parallel()

	model := fullVPCModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	subnets, _ := got.Spec["subnets"].([]any)
	if len(subnets) != 1 {
		t.Fatalf("subnets = %v, want one element", subnets)
	}

	subnet, _ := subnets[0].(map[string]any)
	if subnet["name"] != "web" || subnet["cidr"] != "10.0.0.0/24" {
		t.Errorf("subnet = %v, want {name:web, cidr:10.0.0.0/24}", subnet)
	}
}

func TestVPCFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "net",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"subnets": []any{map[string]any{"name": "web", "cidr": "10.0.0.0/24"}},
			"peers":   []any{map[string]any{"tenantNamespace": "tenant-foo", "vpcName": "net"}},
			"routes":  []any{map[string]any{"cidr": "0.0.0.0/0", "nextHopIP": "10.0.0.1"}},
		},
		Status: client.ApplicationStatus{Version: "1.0.0", Ready: true},
	}

	var model vpcModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if len(model.Subnets.Elements()) != 1 {
		t.Errorf("subnets = %v, want one element", model.Subnets.Elements())
	}

	if len(model.Peers.Elements()) != 1 {
		t.Errorf("peers = %v, want one element", model.Peers.Elements())
	}

	if len(model.Routes.Elements()) != 1 {
		t.Errorf("routes = %v, want one element", model.Routes.Elements())
	}
}

func TestVPCExpandKeysMatchConfigSpec(t *testing.T) {
	t.Parallel()

	model := fullVPCModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	assertSpecCoverage(t, got.Spec, vpc.ConfigSpec{})

	subnets, _ := got.Spec["subnets"].([]any)
	if len(subnets) != 1 {
		t.Fatalf("subnets = %v, want one element", subnets)
	}
	subnet, _ := subnets[0].(map[string]any)
	// allowSubnets is not exposed by the provider.
	assertSpecCoverage(t, subnet, vpc.Subnet{}, "allowSubnets")

	peers, _ := got.Spec["peers"].([]any)
	if len(peers) != 1 {
		t.Fatalf("peers = %v, want one element", peers)
	}
	peer, _ := peers[0].(map[string]any)
	assertSpecCoverage(t, peer, vpc.Peer{})

	routes, _ := got.Spec["routes"].([]any)
	if len(routes) != 1 {
		t.Fatalf("routes = %v, want one element", routes)
	}
	route, _ := routes[0].(map[string]any)
	assertSpecCoverage(t, route, vpc.Route{})
}
