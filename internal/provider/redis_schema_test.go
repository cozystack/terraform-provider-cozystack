package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// TestRedisMaxmemoryPolicySchema separates resource defaulting from read-only
// data-source state. Policy changes must not require replacement or retain a
// stale value through a plan modifier.
func TestRedisMaxmemoryPolicySchema(t *testing.T) {
	t.Parallel()

	attribute, ok := redisSchema().Attributes["maxmemory_policy"].(schema.StringAttribute)
	if !ok {
		t.Fatal("maxmemory_policy is not a string attribute")
	}
	if !attribute.Optional || !attribute.Computed || attribute.Default == nil {
		t.Fatal("maxmemory_policy must be optional and computed with a default")
	}
	if len(attribute.PlanModifiers) != 0 {
		t.Fatal("changing maxmemory_policy must not require replacement or preserve prior state")
	}

	var response datasource.SchemaResponse
	NewRedisDataSource().Schema(context.Background(), datasource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("data source schema diagnostics: %v", response.Diagnostics)
	}
	dataAttribute := response.Schema.Attributes["maxmemory_policy"]
	if dataAttribute == nil || !dataAttribute.IsComputed() || dataAttribute.IsOptional() || dataAttribute.IsRequired() {
		t.Fatal("data source maxmemory_policy must be computed only")
	}
}

// TestRedisMaxmemoryPolicyValidation allows null and unknown values during
// planning while requiring concrete policies to use Redis's case-sensitive names.
func TestRedisMaxmemoryPolicyValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   types.String
		invalid bool
	}{
		{name: "noeviction", value: types.StringValue("noeviction")},
		{name: "allkeys-lru", value: types.StringValue("allkeys-lru")},
		{name: "allkeys-lfu", value: types.StringValue("allkeys-lfu")},
		{name: "volatile-lru", value: types.StringValue("volatile-lru")},
		{name: "volatile-lfu", value: types.StringValue("volatile-lfu")},
		{name: "allkeys-random", value: types.StringValue("allkeys-random")},
		{name: "volatile-random", value: types.StringValue("volatile-random")},
		{name: "volatile-ttl", value: types.StringValue("volatile-ttl")},
		{name: "empty", value: types.StringValue(""), invalid: true},
		{name: "invalid", value: types.StringValue("allkeys-ttl"), invalid: true},
		{name: "case-sensitive", value: types.StringValue("NoEviction"), invalid: true},
		{name: "null", value: types.StringNull()},
		{name: "unknown", value: types.StringUnknown()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			attribute := redisSchema().Attributes["maxmemory_policy"].(schema.StringAttribute)
			var response validator.StringResponse
			for _, check := range attribute.Validators {
				check.ValidateString(context.Background(), validator.StringRequest{
					Path: path.Root("maxmemory_policy"), ConfigValue: tt.value,
				}, &response)
			}
			if response.Diagnostics.HasError() != tt.invalid {
				t.Errorf("validation diagnostics = %v, want error = %v", response.Diagnostics, tt.invalid)
			}
		})
	}
}

// TestRedisMaxmemoryPolicyPlan exercises framework-owned defaulting: removing
// configuration resets the prior policy, while an unknown expression stays
// unknown and policy changes remain in-place.
func TestRedisMaxmemoryPolicyPlan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   any
		prior    any
		proposed any
		want     any
	}{
		{name: "omitted on create", want: "noeviction"},
		{name: "removed from config resets policy", prior: "allkeys-lru", proposed: "allkeys-lru", want: "noeviction"},
		{name: "configured", config: "volatile-ttl", proposed: "volatile-ttl", want: "volatile-ttl"},
		{name: "changed without replacement", config: "allkeys-lfu", prior: "allkeys-lru", proposed: "allkeys-lfu", want: "allkeys-lfu"},
		{name: "unknown is not defaulted", config: tftypes.UnknownValue, proposed: tftypes.UnknownValue, want: tftypes.UnknownValue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			server := providerserver.NewProtocol6(New("test")())()
			schemaType := redisSchema().Type().TerraformType(ctx)
			prior, err := tfprotov6.NewDynamicValue(schemaType, tftypes.NewValue(schemaType, nil))
			if err != nil {
				t.Fatal(err)
			}
			if tt.prior != nil {
				prior = redisProtocolValue(t, schemaType, tt.prior)
			}

			config := redisProtocolValue(t, schemaType, tt.config)
			proposed := redisProtocolValue(t, schemaType, tt.proposed)
			response, err := server.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
				TypeName: "cozystack_redis", PriorState: &prior, Config: &config, ProposedNewState: &proposed,
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, diagnostic := range response.Diagnostics {
				if diagnostic.Severity == tfprotov6.DiagnosticSeverityError {
					t.Fatalf("plan diagnostic: %s: %s", diagnostic.Summary, diagnostic.Detail)
				}
			}
			planned, err := response.PlannedState.Unmarshal(schemaType)
			if err != nil {
				t.Fatal(err)
			}
			var attributes map[string]tftypes.Value
			if err := planned.As(&attributes); err != nil {
				t.Fatal(err)
			}
			want := tftypes.NewValue(tftypes.String, tt.want)
			if !attributes["maxmemory_policy"].Equal(want) {
				t.Errorf("planned maxmemory_policy = %v, want %v", attributes["maxmemory_policy"], want)
			}
			if len(response.RequiresReplace) != 0 {
				t.Errorf("RequiresReplace = %v, want none", response.RequiresReplace)
			}
		})
	}
}

func redisProtocolValue(t *testing.T, schemaType tftypes.Type, policy any) tfprotov6.DynamicValue {
	t.Helper()

	attributes := map[string]tftypes.Value{}
	for name, typ := range schemaType.(tftypes.Object).AttributeTypes {
		attributes[name] = tftypes.NewValue(typ, nil)
	}
	attributes["name"] = tftypes.NewValue(tftypes.String, "cache")
	attributes["namespace"] = tftypes.NewValue(tftypes.String, "tenant-root")
	attributes["maxmemory_policy"] = tftypes.NewValue(tftypes.String, policy)

	value, err := tfprotov6.NewDynamicValue(schemaType, tftypes.NewValue(schemaType, attributes))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

// TestRedisMaxmemoryPolicyStateBinding checks that embedding redisModel does
// not lose the policy's tfsdk binding in resource or data-source state.
func TestRedisMaxmemoryPolicyStateBinding(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	resourceSchema := redisSchema()
	state := tfsdk.State{Schema: resourceSchema, Raw: tftypes.NewValue(resourceSchema.Type().TerraformType(ctx), nil)}
	in := redisResourceModel{redisModel: fullRedisModel()}
	if diags := state.Set(ctx, &in); diags.HasError() {
		t.Fatalf("resource state diagnostics: %v", diags)
	}
	var out redisResourceModel
	if diags := state.Get(ctx, &out); diags.HasError() {
		t.Fatalf("resource model diagnostics: %v", diags)
	}
	if !out.MaxmemoryPolicy.Equal(in.MaxmemoryPolicy) {
		t.Errorf("resource maxmemory_policy = %v, want %v", out.MaxmemoryPolicy, in.MaxmemoryPolicy)
	}

	var response datasource.SchemaResponse
	NewRedisDataSource().Schema(ctx, datasource.SchemaRequest{}, &response)
	state = tfsdk.State{Schema: response.Schema, Raw: tftypes.NewValue(response.Schema.Type().TerraformType(ctx), nil)}
	if diags := state.Set(ctx, &in.redisModel); diags.HasError() {
		t.Fatalf("data source state diagnostics: %v", diags)
	}
	var dataOut redisModel
	if diags := state.Get(ctx, &dataOut); diags.HasError() {
		t.Fatalf("data source model diagnostics: %v", diags)
	}
	if !dataOut.MaxmemoryPolicy.Equal(in.MaxmemoryPolicy) {
		t.Errorf("data source maxmemory_policy = %v, want %v", dataOut.MaxmemoryPolicy, in.MaxmemoryPolicy)
	}
}
