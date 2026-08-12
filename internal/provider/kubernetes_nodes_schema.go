package provider

import (
	"maps"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const k8sNodesNameDescription = "Node pool name (`metadata.name`). Must be `<cluster>-<pool>`, where `<cluster>` " +
	"matches the `cluster` attribute and `<pool>` names the pool — `demo-md1` for pool `md1` of cluster `demo`. " +
	"Immutable."

const k8sNodesDescription = "A worker node pool for a Cozystack managed Kubernetes cluster. The pool attaches to " +
	"its parent cluster by name: `cluster` names the `cozystack_kubernetes` object in the same namespace, and the " +
	"pool itself must be named `<cluster>-<pool>`. A pool whose name collides with a node group the parent cluster " +
	"still manages — the default `md0`, for instance — fails to render on an ownership conflict.\n\n" +
	"The `talos`, `images`, and `kubelet` blocks are left to the chart unless set, and stay out of state when " +
	"unset, so a pool keeps following the Talos release, schematic, and images that ship with the installed " +
	"Cozystack version — the same ones its parent cluster follows.\n\n" +
	"A pool cannot report `Ready` until its workers join, which needs the parent control plane up first, so " +
	"`wait_for_ready` on a pool created alongside its cluster will usually spend the whole `wait_timeout`.\n\n" +
	"Import captures the overrides an existing pool carries. Copy the imported `talos`, `images`, `kubelet`, " +
	"`roles`, and `gpus` values into the configuration before the first apply: an update replaces the whole " +
	"spec, so an override present on the object but absent from configuration is removed by that apply — the " +
	"plan shows the removal, but only the configuration can prevent it."

// k8sNodesSizingAttributes returns the pool sizing and placement attributes.
func k8sNodesSizingAttributes() map[string]rschema.Attribute {
	return map[string]rschema.Attribute{
		"cluster": rschema.StringAttribute{
			Required:      true,
			Validators:    []validator.String{stringvalidator.LengthAtLeast(1)},
			PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			MarkdownDescription: "Name of the parent `cozystack_kubernetes` cluster in the same namespace this " +
				"pool attaches to. A pool cannot be rewired to another cluster in place. Immutable.",
		},
		attrStorageClass: rschema.StringAttribute{
			Optional: true, Computed: true,
			Default: stringdefault.StaticString("replicated"),
			MarkdownDescription: "StorageClass for the worker node system disks. Worker VMs live-migrate, so the " +
				"class must serve ReadWriteMany volumes — prefer a replicated, DRBD-backed class.",
		},
		"min_replicas": rschema.Int64Attribute{
			Optional: true, Computed: true,
			Default:             int64default.StaticInt64(0),
			MarkdownDescription: "Minimum number of nodes in the pool; the cluster-autoscaler floor.",
		},
		"max_replicas": rschema.Int64Attribute{
			Optional: true, Computed: true,
			Default:             int64default.StaticInt64(10),
			MarkdownDescription: "Maximum number of nodes in the pool; the cluster-autoscaler ceiling.",
		},
		"instance_type": rschema.StringAttribute{
			Optional: true, Computed: true,
			Default:             stringdefault.StaticString("u1.medium"),
			MarkdownDescription: "Virtual machine instance type.",
		},
		"disk_size": rschema.StringAttribute{
			Optional: true, Computed: true,
			Default: stringdefault.StaticString("20Gi"),
			MarkdownDescription: "System disk size for each worker VM. Carries the Talos image, kubelet state, and " +
				"the containerd image cache.",
		},
		"roles": rschema.ListAttribute{
			Optional:    true,
			ElementType: types.StringType,
			MarkdownDescription: "Node roles. Each role `r` labels the pool's nodes `node-role.kubernetes.io/<r>`; " +
				"use `[\"ingress-nginx\"]` for a pool hosting the tenant ingress controller.",
		},
		attrResources: k8sNodesResourcesAttribute(),
		"gpus": rschema.ListNestedAttribute{
			Optional:            true,
			MarkdownDescription: "GPUs to attach to each node (the NVIDIA driver needs at least 4 GiB of memory).",
			NestedObject: rschema.NestedAttributeObject{
				Attributes: map[string]rschema.Attribute{
					attrName: rschema.StringAttribute{
						Required:            true,
						MarkdownDescription: "GPU resource name, such as `nvidia.com/AD102GL_L40S`.",
					},
				},
			},
		},
	}
}

// k8sNodesResourcesAttribute returns the per-node CPU and memory block, which
// replaces instance-type sizing when both fields are set.
func k8sNodesResourcesAttribute() rschema.SingleNestedAttribute {
	return rschema.SingleNestedAttribute{
		Optional: true,
		MarkdownDescription: "Explicit CPU and memory per node, replacing `instance_type` sizing. Set both " +
			"fields or neither — setting only one is rejected when the pool renders.",
		Attributes: map[string]rschema.Attribute{
			attrCPU: rschema.StringAttribute{
				Optional:            true,
				Validators:          []validator.String{stringvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName(attrMemory))},
				MarkdownDescription: "CPU available to each node (quantity, e.g. `4`). Requires `memory`.",
			},
			attrMemory: rschema.StringAttribute{
				Optional:            true,
				Validators:          []validator.String{stringvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName(attrCPU))},
				MarkdownDescription: "Memory available to each node (quantity, e.g. `8Gi`). Requires `cpu`.",
			},
		},
	}
}

// k8sNodesLifecycleAttributes returns the remediation, version, kubelet, Talos,
// and image-override attributes.
func k8sNodesLifecycleAttributes() map[string]rschema.Attribute {
	return map[string]rschema.Attribute{
		"max_unhealthy": rschema.StringAttribute{
			Optional: true, Computed: true,
			Default: stringdefault.StaticString("50%"),
			MarkdownDescription: "How many of the pool's nodes may be unhealthy before remediation pauses, as a " +
				"percentage (`50%`) or a bare integer (`1`). Drop to `0%` once the pool is stable.",
		},
		"node_startup_timeout": rschema.StringAttribute{
			Optional: true, Computed: true,
			Default: stringdefault.StaticString("10m"),
			MarkdownDescription: "How long a machine may take to reach `Ready` before it is remediated (Go " +
				"duration). Raise it for slow first boots.",
		},
		attrVersion: rschema.StringAttribute{
			Optional: true, Computed: true,
			Default:    stringdefault.StaticString("v1.35"),
			Validators: []validator.String{stringvalidator.OneOf("v1.35", "v1.34", "v1.33", "v1.32", "v1.31")},
			MarkdownDescription: "Kubernetes major.minor version the pool joins. Must match the parent cluster's " +
				"version and satisfy the Talos/Kubernetes support matrix against `talos.version`.",
		},
		"kubelet": rschema.SingleNestedAttribute{
			Optional: true,
			MarkdownDescription: "Kubelet resource reservations. The reserved CPU and memory fields are computed " +
				"from `instance_type` when left unset.",
			Attributes: map[string]rschema.Attribute{
				"eviction_hard_memory":   k8sNodesChartField("Hard memory eviction threshold, absolute (`200Mi`) or a percentage (`7%`)."),
				"eviction_soft_memory":   k8sNodesChartField("Soft memory eviction threshold, absolute (`1Gi`) or a percentage (`10%`)."),
				"kube_reserved_cpu":      k8sNodesChartField("CPU reserved for kubelet and the container runtime."),
				"kube_reserved_memory":   k8sNodesChartField("Memory reserved for kubelet and the container runtime."),
				"system_reserved_cpu":    k8sNodesChartField("CPU reserved for the host OS."),
				"system_reserved_memory": k8sNodesChartField("Memory reserved for the host OS."),
			},
		},
		"talos": rschema.SingleNestedAttribute{
			Optional: true,
			MarkdownDescription: "Talos worker image configuration, kept in sync with the parent cluster's Talos " +
				"settings. Every field follows the chart when unset.",
			Attributes: map[string]rschema.Attribute{
				attrVersion: k8sNodesChartField("Talos release used for the worker OS image and installer."),
				"schematic_id": k8sNodesChartField("Talos image-factory schematic ID. Set it for a custom " +
					"schematic carrying system extensions or extra kernel arguments."),
				"image_factory_url": k8sNodesChartField("Base URL of the Talos Image Factory serving the worker " +
					"OS disk image. Point it at a mirror or self-hosted factory for air-gapped installs. No trailing slash."),
				"installer_repository": k8sNodesChartField("OCI repository prefix for the Talos installer image, " +
					"resolved as `<installer_repository>/<schematic_id>:<version>`. No trailing slash."),
			},
		},
		"images": rschema.SingleNestedAttribute{
			Optional: true,
			MarkdownDescription: "Image overrides for air-gapped or rate-limited registries. Each field follows " +
				"the chart when unset.",
			Attributes: map[string]rschema.Attribute{
				"kubectl": k8sNodesChartField("Image used by the pool's `talos-reconcile` Job."),
			},
		},
	}
}

// k8sNodesChartField returns a field of a chart-owned nested block. It is
// optional and deliberately not computed: the value the server resolves for an
// unset field must not reach state, or the next update would write it back as
// an explicit spec key and pin the pool to it.
func k8sNodesChartField(description string) rschema.StringAttribute {
	return rschema.StringAttribute{Optional: true, MarkdownDescription: description}
}

func kubernetesNodesSchema() rschema.Schema {
	attributes := identityResourceAttributes(k8sNodesNameDescription)

	maps.Copy(attributes, k8sNodesSizingAttributes())
	maps.Copy(attributes, k8sNodesLifecycleAttributes())
	maps.Copy(attributes, statusResourceAttributes())
	maps.Copy(attributes, waitBehaviorAttributes())

	return rschema.Schema{
		MarkdownDescription: k8sNodesDescription,
		Attributes:          attributes,
	}
}

// k8sNodesDataSourceAttributes returns the computed pool attributes.
func k8sNodesDataSourceAttributes() map[string]dsschema.Attribute {
	return map[string]dsschema.Attribute{
		"cluster":        dsschema.StringAttribute{Computed: true, MarkdownDescription: "Parent Kubernetes cluster the pool attaches to."},
		attrStorageClass: dsschema.StringAttribute{Computed: true, MarkdownDescription: "StorageClass for the worker node system disks."},
		"min_replicas":   dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Minimum number of nodes."},
		"max_replicas":   dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum number of nodes."},
		"instance_type":  dsschema.StringAttribute{Computed: true, MarkdownDescription: "Instance type."},
		"disk_size":      dsschema.StringAttribute{Computed: true, MarkdownDescription: "System disk size for each worker VM."},
		"roles":          dsschema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Node roles."},
		attrResources: dsschema.SingleNestedAttribute{
			Computed:            true,
			MarkdownDescription: "Explicit CPU and memory per node, when set.",
			Attributes: map[string]dsschema.Attribute{
				attrCPU:    dsschema.StringAttribute{Computed: true, MarkdownDescription: "CPU available to each node."},
				attrMemory: dsschema.StringAttribute{Computed: true, MarkdownDescription: "Memory available to each node."},
			},
		},
		"gpus":                 vmNameListDataSourceAttribute("GPUs attached to each node."),
		"max_unhealthy":        dsschema.StringAttribute{Computed: true, MarkdownDescription: "Unhealthy-node budget before remediation pauses."},
		"node_startup_timeout": dsschema.StringAttribute{Computed: true, MarkdownDescription: "Machine startup budget before remediation."},
		attrVersion:            dsschema.StringAttribute{Computed: true, MarkdownDescription: "Kubernetes version the pool joins."},
		"kubelet": dsschema.SingleNestedAttribute{
			Computed:            true,
			MarkdownDescription: "Kubelet resource reservations.",
			Attributes: map[string]dsschema.Attribute{
				"eviction_hard_memory":   dsschema.StringAttribute{Computed: true, MarkdownDescription: "Hard memory eviction threshold."},
				"eviction_soft_memory":   dsschema.StringAttribute{Computed: true, MarkdownDescription: "Soft memory eviction threshold."},
				"kube_reserved_cpu":      dsschema.StringAttribute{Computed: true, MarkdownDescription: "CPU reserved for kubelet and the container runtime."},
				"kube_reserved_memory":   dsschema.StringAttribute{Computed: true, MarkdownDescription: "Memory reserved for kubelet and the container runtime."},
				"system_reserved_cpu":    dsschema.StringAttribute{Computed: true, MarkdownDescription: "CPU reserved for the host OS."},
				"system_reserved_memory": dsschema.StringAttribute{Computed: true, MarkdownDescription: "Memory reserved for the host OS."},
			},
		},
		"talos": dsschema.SingleNestedAttribute{
			Computed:            true,
			MarkdownDescription: "Talos worker image configuration.",
			Attributes: map[string]dsschema.Attribute{
				attrVersion:            dsschema.StringAttribute{Computed: true, MarkdownDescription: "Talos release."},
				"schematic_id":         dsschema.StringAttribute{Computed: true, MarkdownDescription: "Talos image-factory schematic ID."},
				"image_factory_url":    dsschema.StringAttribute{Computed: true, MarkdownDescription: "Talos Image Factory base URL."},
				"installer_repository": dsschema.StringAttribute{Computed: true, MarkdownDescription: "Talos installer image repository."},
			},
		},
		"images": dsschema.SingleNestedAttribute{
			Computed:            true,
			MarkdownDescription: "Image overrides.",
			Attributes: map[string]dsschema.Attribute{
				"kubectl": dsschema.StringAttribute{Computed: true, MarkdownDescription: "Image used by the pool's `talos-reconcile` Job."},
			},
		},
	}
}

func kubernetesNodesDataSourceSchema() dsschema.Schema {
	attributes := identityDataSourceAttributes("Node pool name (`<cluster>-<pool>`).")

	maps.Copy(attributes, k8sNodesDataSourceAttributes())
	maps.Copy(attributes, statusDataSourceAttributes())

	return dsschema.Schema{
		MarkdownDescription: "Read an existing Cozystack Kubernetes worker node pool by name and namespace.",
		Attributes:          attributes,
	}
}
