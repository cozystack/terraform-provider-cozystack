package provider

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/tenant"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

func fullModel() tenantModel {
	return tenantModel{
		Name:            types.StringValue("dev"),
		Namespace:       types.StringValue("tenant-root"),
		Host:            types.StringValue("dev.example.test"),
		Etcd:            types.BoolValue(true),
		Monitoring:      types.BoolValue(true),
		Ingress:         types.BoolValue(false),
		Seaweedfs:       types.BoolValue(false),
		SchedulingClass: types.StringValue("default"),
		ResourceQuotas: types.MapValueMust(types.StringType, map[string]attr.Value{
			"cpu":    types.StringValue("4"),
			"memory": types.StringValue("8Gi"),
		}),
	}
}

func TestExpand_CoreFields(t *testing.T) {
	t.Parallel()

	model := fullModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	if got.Name != "dev" || got.Namespace != "tenant-root" {
		t.Errorf("identity = %s/%s, want tenant-root/dev", got.Namespace, got.Name)
	}
	if got.Spec["host"] != "dev.example.test" {
		t.Errorf("spec.host = %v, want dev.example.test", got.Spec["host"])
	}
	if got.Spec["etcd"] != true {
		t.Errorf("spec.etcd = %v, want true", got.Spec["etcd"])
	}
	if got.Spec["ingress"] != false {
		t.Errorf("spec.ingress = %v, want false", got.Spec["ingress"])
	}
	if got.Spec["schedulingClass"] != "default" {
		t.Errorf("spec.schedulingClass = %v, want default", got.Spec["schedulingClass"])
	}
}

func TestExpand_ResourceQuotas(t *testing.T) {
	t.Parallel()

	model := fullModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	quotas, ok := got.Spec["resourceQuotas"].(map[string]any)
	if !ok {
		t.Fatalf("spec.resourceQuotas type = %T, want map[string]any", got.Spec["resourceQuotas"])
	}
	if quotas["cpu"] != "4" || quotas["memory"] != "8Gi" {
		t.Errorf("resourceQuotas = %v, want cpu=4 memory=8Gi", quotas)
	}
}

func TestExpand_NullQuotasYieldEmptyMap(t *testing.T) {
	t.Parallel()

	model := fullModel()
	model.ResourceQuotas = types.MapNull(types.StringType)

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	quotas, ok := got.Spec["resourceQuotas"].(map[string]any)
	if !ok {
		t.Fatalf("spec.resourceQuotas type = %T, want map[string]any", got.Spec["resourceQuotas"])
	}
	if len(quotas) != 0 {
		t.Errorf("resourceQuotas = %v, want empty", quotas)
	}
}

func TestFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	tn := &client.Application{
		Name:      "dev",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"host":            "dev.example.test",
			"etcd":            true,
			"monitoring":      false,
			"ingress":         true,
			"seaweedfs":       false,
			"schedulingClass": "default",
			"resourceQuotas":  map[string]any{"cpu": "4"},
		},
		Status: client.ApplicationStatus{
			Version: "1.4.2",
			Ready:   true,
			Raw:     map[string]any{"namespace": "tenant-dev"},
		},
	}

	var model tenantModel

	diags := model.flatten(tn)
	if diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.ID.ValueString() != "tenant-root/dev" {
		t.Errorf("id = %q, want tenant-root/dev", model.ID.ValueString())
	}
	if model.Host.ValueString() != "dev.example.test" {
		t.Errorf("host = %q, want dev.example.test", model.Host.ValueString())
	}
	if !model.Etcd.ValueBool() || model.Monitoring.ValueBool() {
		t.Errorf("etcd/monitoring = %v/%v, want true/false", model.Etcd.ValueBool(), model.Monitoring.ValueBool())
	}
	if !model.Ready.ValueBool() {
		t.Errorf("ready = false, want true")
	}
	if model.StatusNamespace.ValueString() != "tenant-dev" {
		t.Errorf("status_namespace = %q, want tenant-dev", model.StatusNamespace.ValueString())
	}
	if model.Version.ValueString() != "1.4.2" {
		t.Errorf("version = %q, want 1.4.2", model.Version.ValueString())
	}

	quotas := model.ResourceQuotas.Elements()
	if got := quotas["cpu"]; got == nil || got.(types.String).ValueString() != "4" {
		t.Errorf("resource_quotas[cpu] = %v, want 4", got)
	}
}

func TestFlatten_MissingBoolDefaultsFalse(t *testing.T) {
	t.Parallel()

	tn := &client.Application{
		Name:      "dev",
		Namespace: "tenant-root",
		Spec:      map[string]any{},
	}

	var model tenantModel

	if diags := model.flatten(tn); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if model.Etcd.ValueBool() || model.Monitoring.ValueBool() {
		t.Errorf("missing bools should default to false")
	}
	if model.Host.ValueString() != "" {
		t.Errorf("missing host should default to empty, got %q", model.Host.ValueString())
	}
}

// TestExpandKeysMatchConfigSpec guards against field-name drift: every spec key
// the provider emits must be a json tag on the pinned tenant.ConfigSpec, and the
// provider must cover all of them.
func TestExpandKeysMatchConfigSpec(t *testing.T) {
	t.Parallel()

	model := fullModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	emitted := make(map[string]bool, len(got.Spec))
	for key := range got.Spec {
		emitted[key] = true
	}

	tags := configSpecJSONTags(tenant.ConfigSpec{})

	for key := range emitted {
		if !tags[key] {
			t.Errorf("expand emits %q, which is not a tenant.ConfigSpec json tag", key)
		}
	}

	for tag := range tags {
		if !emitted[tag] {
			t.Errorf("tenant.ConfigSpec has %q but expand does not emit it", tag)
		}
	}
}

// assertSpecCoverage checks that every emitted spec key is a valid ConfigSpec
// json tag, and that every tag except the intentionally-omitted ones is emitted.
func assertSpecCoverage(t *testing.T, emitted map[string]bool, spec any, omit ...string) {
	t.Helper()

	omitted := make(map[string]bool, len(omit))
	for _, key := range omit {
		omitted[key] = true
	}

	tags := configSpecJSONTags(spec)

	for key := range emitted {
		if !tags[key] {
			t.Errorf("expand emits %q, which is not a ConfigSpec json tag", key)
		}
	}

	for tag := range tags {
		if !emitted[tag] && !omitted[tag] {
			t.Errorf("ConfigSpec has %q but expand does not emit it", tag)
		}
	}
}

func configSpecJSONTags(spec any) map[string]bool {
	tags := map[string]bool{}

	typ := reflect.TypeOf(spec)
	for i := range typ.NumField() {
		name := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
		if name != "" && name != "-" {
			tags[name] = true
		}
	}

	return tags
}
