package provider

import (
	"context"
	"testing"

	"github.com/cozystack/cozystack/api/apps/v1alpha1/kafka"
	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func fullKafkaModel() kafkaModel {
	topic := types.ObjectValueMust(kafkaTopicObjectType(), map[string]attr.Value{
		"name":       types.StringValue("events"),
		"partitions": types.Int64Value(3),
		"replicas":   types.Int64Value(2),
	})

	return kafkaModel{
		Name:      types.StringValue("queue"),
		Namespace: types.StringValue("tenant-root"),
		External:  types.BoolValue(false),
		TLS:       tlsBlock(true),
		Topics:    types.ListValueMust(types.ObjectType{AttrTypes: kafkaTopicObjectType()}, []attr.Value{topic}),
		Kafka:     kafkaBrokerDefault("10Gi"),
		Zookeeper: kafkaBrokerDefault("5Gi"),
	}
}

func TestKafkaExpand_TopicsAndBrokers(t *testing.T) {
	t.Parallel()

	model := fullKafkaModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	topics, _ := got.Spec["topics"].([]any)
	if len(topics) != 1 {
		t.Fatalf("topics = %v, want one element", topics)
	}

	topic, _ := topics[0].(map[string]any)
	if topic["name"] != "events" || topic["partitions"] != int64(3) {
		t.Errorf("topic = %v, want {name:events, partitions:3}", topic)
	}

	kafkaSpec, _ := got.Spec["kafka"].(map[string]any)
	if kafkaSpec["size"] != "10Gi" {
		t.Errorf("kafka.size = %v, want 10Gi", kafkaSpec["size"])
	}

	zkSpec, _ := got.Spec["zookeeper"].(map[string]any)
	if zkSpec["size"] != "5Gi" {
		t.Errorf("zookeeper.size = %v, want 5Gi", zkSpec["size"])
	}
}

func TestKafkaFlatten_RoundTrip(t *testing.T) {
	t.Parallel()

	app := &client.Application{
		Name:      "queue",
		Namespace: "tenant-root",
		Spec: map[string]any{
			"external": true,
			"topics":   []any{map[string]any{"name": "events", "partitions": int64(3), "replicas": int64(2)}},
			"kafka": map[string]any{
				"replicas": int64(3), "resourcesPreset": "c1.small", "size": "10Gi", "storageClass": "", "resources": map[string]any{},
			},
			"zookeeper": map[string]any{
				"replicas": int64(3), "resourcesPreset": "c1.small", "size": "5Gi", "storageClass": "", "resources": map[string]any{},
			},
		},
		Status: client.ApplicationStatus{Version: "1.0.0", Ready: true},
	}

	var model kafkaModel

	if diags := model.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if !model.External.ValueBool() {
		t.Errorf("external = false, want true")
	}

	if len(model.Topics.Elements()) != 1 {
		t.Errorf("topics = %v, want one element", model.Topics.Elements())
	}

	kafkaAttrs := model.Kafka.Attributes()
	if kafkaAttrs["size"].(types.String).ValueString() != "10Gi" {
		t.Errorf("kafka.size = %v, want 10Gi", kafkaAttrs["size"])
	}
}

func TestKafkaExpandKeysMatchConfigSpec(t *testing.T) {
	t.Parallel()

	model := fullKafkaModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	emitted := make(map[string]bool, len(got.Spec))
	for key := range got.Spec {
		emitted[key] = true
	}

	assertSpecCoverage(t, emitted, kafka.ConfigSpec{})
}
