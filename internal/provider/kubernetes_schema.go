package provider

import (
	"maps"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func k8sNodeGroupsResourceAttribute() rschema.MapNestedAttribute {
	return rschema.MapNestedAttribute{
		Required:            true,
		MarkdownDescription: "Worker node groups keyed by name.",
		NestedObject: rschema.NestedAttributeObject{
			Attributes: map[string]rschema.Attribute{
				"disk_size": rschema.StringAttribute{
					Optional: true, Computed: true,
					Default:             stringdefault.StaticString("20Gi"),
					MarkdownDescription: "Persistent disk size for kubelet and containerd data.",
				},
				"instance_type": rschema.StringAttribute{
					Optional: true, Computed: true,
					Default:             stringdefault.StaticString("u1.medium"),
					MarkdownDescription: "Virtual machine instance type.",
				},
				"min_replicas": rschema.Int64Attribute{
					Optional: true, Computed: true,
					Default:             int64default.StaticInt64(0),
					MarkdownDescription: "Minimum number of replicas.",
				},
				"max_replicas": rschema.Int64Attribute{
					Optional: true, Computed: true,
					Default:             int64default.StaticInt64(10),
					MarkdownDescription: "Maximum number of replicas.",
				},
				"roles": rschema.ListAttribute{
					Optional:            true,
					ElementType:         types.StringType,
					MarkdownDescription: "Node roles (e.g. `ingress-nginx`).",
				},
				"storage_class": rschema.StringAttribute{
					Optional: true, Computed: true,
					Default:             stringdefault.StaticString(""),
					MarkdownDescription: "StorageClass for worker node persistent disks.",
				},
				"resources": resourcesResourceAttribute(),
			},
		},
	}
}

// k8sTalosResourceAttribute returns the talos worker-image block. None of its
// fields carries a provider-side default: upstream moves the Talos release and
// the tested schematic with every Cozystack release, and a materialised default
// would freeze the cluster on whatever was current when the provider shipped.
// Leaving a field unset keeps the key out of the spec, so the server's own
// default applies and is reported back into state.
func k8sTalosResourceAttribute() rschema.SingleNestedAttribute {
	return rschema.SingleNestedAttribute{
		Optional: true, Computed: true,
		MarkdownDescription: "Talos worker OS image coordinates. Every field follows the " +
			"platform default while unset; set one only to pin it.",
		Attributes: map[string]rschema.Attribute{
			"image_factory_url": rschema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Base URL of the Talos Image Factory serving the worker OS disk image " +
					"(no trailing slash). Point at a self-hosted factory or caching mirror for air-gapped or " +
					"rate-limited environments. Follows the platform default (`https://factory.talos.dev`) while unset.",
			},
			"installer_repository": rschema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "OCI repository prefix for the Talos installer image, resolved as " +
					"`<installer_repository>/<schematic_id>:<version>` (no trailing slash). Follows the platform " +
					"default (`factory.talos.dev/installer`) while unset.",
			},
			"schematic_id": rschema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Talos image-factory schematic ID. Set it only to use a custom schematic " +
					"(system extensions, kernel args); while unset the cluster follows the platform's tested " +
					"schematic, which changes between Cozystack releases.",
			},
			attrVersion: rschema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Talos release used for the worker OS image and installer. Must satisfy the " +
					"chart's Talos/Kubernetes support matrix against `version`. Follows the platform default " +
					"while unset, which is the safe choice — the matrix moves with each Cozystack release.",
			},
		},
	}
}

func k8sTalosDataSourceAttribute() dsschema.SingleNestedAttribute {
	return dsschema.SingleNestedAttribute{
		Computed:            true,
		MarkdownDescription: "Talos worker OS image coordinates.",
		Attributes: map[string]dsschema.Attribute{
			"image_factory_url":    dsschema.StringAttribute{Computed: true, MarkdownDescription: "Talos Image Factory base URL."},
			"installer_repository": dsschema.StringAttribute{Computed: true, MarkdownDescription: "Talos installer OCI repository prefix."},
			"schematic_id":         dsschema.StringAttribute{Computed: true, MarkdownDescription: "Talos image-factory schematic ID."},
			attrVersion:            dsschema.StringAttribute{Computed: true, MarkdownDescription: "Talos release used for workers."},
		},
	}
}

func kubernetesSchema() rschema.Schema {
	attributes := identityResourceAttributes("Kubernetes cluster name (`metadata.name`). Immutable.")

	maps.Copy(attributes, map[string]rschema.Attribute{
		attrStorageClass: rschema.StringAttribute{
			Optional: true, Computed: true,
			Default:             stringdefault.StaticString("replicated"),
			MarkdownDescription: "StorageClass used to store the data.",
		},
		attrVersion: rschema.StringAttribute{
			Optional: true, Computed: true,
			Default:             stringdefault.StaticString("v1.35"),
			Validators:          []validator.String{stringvalidator.OneOf("v1.35", "v1.34", "v1.33", "v1.32", "v1.31", "v1.30")},
			MarkdownDescription: "Kubernetes major.minor version to deploy.",
		},
		attrHost: rschema.StringAttribute{
			Optional: true, Computed: true,
			MarkdownDescription: "External hostname for the cluster. " +
				"Defaults to `<cluster-name>.<tenant-host>`.",
		},
		"node_groups": k8sNodeGroupsResourceAttribute(),
		specTalos:     k8sTalosResourceAttribute(),
		"kubeconfig": rschema.StringAttribute{
			Computed:  true,
			Sensitive: true,
			MarkdownDescription: "Admin kubeconfig for the provisioned cluster (from the " +
				"`<name>-admin-kubeconfig` Secret). Populated once the cluster is ready — " +
				"set `wait_for_ready = true` to have it available on first apply.",
		},
	})
	maps.Copy(attributes, statusResourceAttributes())
	maps.Copy(attributes, waitBehaviorAttributes())

	return rschema.Schema{
		MarkdownDescription: "A Cozystack managed Kubernetes cluster, deployed inside a tenant namespace. " +
			"The addons, control-plane, image-override blocks, and per-node-group GPU and kubelet " +
			"tuning use server defaults.",
		Attributes: attributes,
	}
}

func kubernetesDataSourceSchema() dsschema.Schema {
	attributes := identityDataSourceAttributes("Kubernetes cluster name.")

	maps.Copy(attributes, map[string]dsschema.Attribute{
		attrStorageClass: dsschema.StringAttribute{Computed: true, MarkdownDescription: "StorageClass used to store the data."},
		attrVersion:      dsschema.StringAttribute{Computed: true, MarkdownDescription: "Kubernetes version."},
		attrHost:         dsschema.StringAttribute{Computed: true, MarkdownDescription: "External hostname for the cluster."},
		"node_groups": dsschema.MapNestedAttribute{
			Computed:            true,
			MarkdownDescription: "Worker node groups keyed by name.",
			NestedObject: dsschema.NestedAttributeObject{
				Attributes: map[string]dsschema.Attribute{
					"disk_size":     dsschema.StringAttribute{Computed: true, MarkdownDescription: "Persistent disk size."},
					"instance_type": dsschema.StringAttribute{Computed: true, MarkdownDescription: "Instance type."},
					"min_replicas":  dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Minimum replicas."},
					"max_replicas":  dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum replicas."},
					"roles":         dsschema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Node roles."},
					"storage_class": dsschema.StringAttribute{Computed: true, MarkdownDescription: "Worker node StorageClass."},
					"resources":     resourcesDataSourceAttribute(),
				},
			},
		},
		specTalos: k8sTalosDataSourceAttribute(),
		"kubeconfig": dsschema.StringAttribute{
			Computed:            true,
			Sensitive:           true,
			MarkdownDescription: "Admin kubeconfig for the cluster.",
		},
	})
	maps.Copy(attributes, statusDataSourceAttributes())

	return dsschema.Schema{
		MarkdownDescription: "Read an existing Cozystack Kubernetes cluster by name and namespace.",
		Attributes:          attributes,
	}
}
