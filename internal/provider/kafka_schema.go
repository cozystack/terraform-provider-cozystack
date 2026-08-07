package provider

import (
	"maps"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func kafkaBrokerDefault(sizeDefault string) types.Object {
	return types.ObjectValueMust(kafkaBrokerObjectType(), map[string]attr.Value{
		"replicas":         types.Int64Value(3),
		"resources":        types.ObjectNull(resourcesObjectType()),
		"resources_preset": types.StringValue("c1.small"),
		"size":             types.StringValue(sizeDefault),
		"storage_class":    types.StringValue(""),
	})
}

func kafkaBrokerResourceAttribute(desc, sizeDefault string) rschema.SingleNestedAttribute {
	return rschema.SingleNestedAttribute{
		Optional: true, Computed: true,
		Default:             objectdefault.StaticValue(kafkaBrokerDefault(sizeDefault)),
		MarkdownDescription: desc,
		Attributes: map[string]rschema.Attribute{
			"replicas": rschema.Int64Attribute{
				Optional: true, Computed: true,
				Default:             int64default.StaticInt64(3),
				MarkdownDescription: "Number of replicas.",
			},
			"resources": rschema.SingleNestedAttribute{
				Optional: true, Computed: true,
				Default:             objectdefault.StaticValue(types.ObjectNull(resourcesObjectType())),
				MarkdownDescription: "Explicit CPU and memory configuration. When omitted, the preset is applied.",
				Attributes: map[string]rschema.Attribute{
					attrCPU:    rschema.StringAttribute{Optional: true, MarkdownDescription: "CPU available to each replica."},
					attrMemory: rschema.StringAttribute{Optional: true, MarkdownDescription: "Memory available to each replica."},
				},
			},
			"resources_preset": rschema.StringAttribute{
				Optional: true, Computed: true,
				Default:             stringdefault.StaticString("c1.small"),
				MarkdownDescription: "Default sizing preset used when `resources` is omitted.",
			},
			"size": rschema.StringAttribute{
				Optional: true, Computed: true,
				Default:             stringdefault.StaticString(sizeDefault),
				MarkdownDescription: "Persistent volume size (quantity, e.g. `10Gi`).",
			},
			"storage_class": rschema.StringAttribute{
				Optional: true, Computed: true,
				Default:             stringdefault.StaticString(""),
				MarkdownDescription: "StorageClass used to store the data.",
			},
		},
	}
}

func kafkaTopicsResourceAttribute() rschema.ListNestedAttribute {
	return rschema.ListNestedAttribute{
		Optional:            true,
		MarkdownDescription: "Topics to provision.",
		NestedObject: rschema.NestedAttributeObject{
			Attributes: map[string]rschema.Attribute{
				"name":       rschema.StringAttribute{Required: true, MarkdownDescription: "Topic name."},
				"partitions": rschema.Int64Attribute{Required: true, MarkdownDescription: "Number of partitions."},
				"replicas":   rschema.Int64Attribute{Required: true, MarkdownDescription: "Number of replicas."},
			},
		},
	}
}

func kafkaSchema() rschema.Schema {
	attributes := identityResourceAttributes("Kafka instance name (`metadata.name`). Immutable.")

	maps.Copy(attributes, map[string]rschema.Attribute{
		attrExternal: rschema.BoolAttribute{
			Optional: true, Computed: true,
			Default:             booldefault.StaticBool(false),
			MarkdownDescription: "Enable external access from outside the cluster.",
		},
		specTLS: tlsResourceAttribute(
			"TLS configuration for the external listener on port 9094. The internal listener on 9093 is " +
				"always TLS, and Strimzi manages the cluster PKI itself. Omit the block to follow `external`; " +
				"disabling TLS while `external` is true publishes Kafka in plaintext on a public address.",
		),
		"topics":    kafkaTopicsResourceAttribute(),
		"kafka":     kafkaBrokerResourceAttribute("Kafka broker configuration.", "10Gi"),
		"zookeeper": kafkaBrokerResourceAttribute("ZooKeeper configuration.", "5Gi"),
	})
	maps.Copy(attributes, statusResourceAttributes())
	maps.Copy(attributes, waitBehaviorAttributes())

	return rschema.Schema{
		MarkdownDescription: "A Cozystack managed Kafka cluster, deployed inside a tenant namespace. " +
			"Per-topic opaque tuning is not managed.",
		Attributes: attributes,
	}
}

func kafkaBrokerDataSourceAttribute(desc string) dsschema.SingleNestedAttribute {
	return dsschema.SingleNestedAttribute{
		Computed:            true,
		MarkdownDescription: desc,
		Attributes: map[string]dsschema.Attribute{
			"replicas":         dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Number of replicas."},
			"resources":        resourcesDataSourceAttribute(),
			"resources_preset": dsschema.StringAttribute{Computed: true, MarkdownDescription: "Sizing preset."},
			"size":             dsschema.StringAttribute{Computed: true, MarkdownDescription: "Persistent volume size."},
			"storage_class":    dsschema.StringAttribute{Computed: true, MarkdownDescription: "StorageClass used to store the data."},
		},
	}
}

func kafkaDataSourceSchema() dsschema.Schema {
	attributes := identityDataSourceAttributes("Kafka instance name.")

	maps.Copy(attributes, map[string]dsschema.Attribute{
		attrExternal: dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether external access is enabled."},
		specTLS:      tlsDataSourceAttribute("TLS configuration for the external listener."),
		"topics": dsschema.ListNestedAttribute{
			Computed:            true,
			MarkdownDescription: "Provisioned topics.",
			NestedObject: dsschema.NestedAttributeObject{
				Attributes: map[string]dsschema.Attribute{
					"name":       dsschema.StringAttribute{Computed: true, MarkdownDescription: "Topic name."},
					"partitions": dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Number of partitions."},
					"replicas":   dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Number of replicas."},
				},
			},
		},
		"kafka":     kafkaBrokerDataSourceAttribute("Kafka broker configuration."),
		"zookeeper": kafkaBrokerDataSourceAttribute("ZooKeeper configuration."),
	})
	maps.Copy(attributes, statusDataSourceAttributes())

	return dsschema.Schema{
		MarkdownDescription: "Read an existing Cozystack Kafka cluster by name and namespace.",
		Attributes:          attributes,
	}
}
