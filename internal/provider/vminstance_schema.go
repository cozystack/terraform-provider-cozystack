package provider

import (
	"maps"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func vmNameListResourceAttribute(desc, itemDesc string) rschema.ListNestedAttribute {
	return rschema.ListNestedAttribute{
		Optional:            true,
		MarkdownDescription: desc,
		NestedObject: rschema.NestedAttributeObject{
			Attributes: map[string]rschema.Attribute{
				"name": rschema.StringAttribute{Required: true, MarkdownDescription: itemDesc},
			},
		},
	}
}

func vminstanceAccessAttributes() map[string]rschema.Attribute {
	return map[string]rschema.Attribute{
		attrExternal: rschema.BoolAttribute{
			Optional: true, Computed: true,
			Default:             booldefault.StaticBool(false),
			MarkdownDescription: "Enable external access from outside the cluster.",
		},
		"external_method": rschema.StringAttribute{
			Optional: true, Computed: true,
			Default:             stringdefault.StaticString("PortList"),
			Validators:          []validator.String{stringvalidator.OneOf("PortList", "WholeIP")},
			MarkdownDescription: "Method to pass through traffic to the VM (`PortList` or `WholeIP`).",
		},
		"external_ports": rschema.ListAttribute{
			Optional: true, Computed: true,
			ElementType:         types.Int64Type,
			Default:             listdefault.StaticValue(types.ListValueMust(types.Int64Type, []attr.Value{types.Int64Value(22)})),
			MarkdownDescription: "Ports to forward from outside the cluster.",
		},
		"external_allow_icmp": rschema.BoolAttribute{
			Optional: true, Computed: true,
			Default:             booldefault.StaticBool(true),
			MarkdownDescription: "Accept ICMP traffic to the VM in PortList mode (preserves ping and PMTU discovery).",
		},
		"run_strategy": rschema.StringAttribute{
			Optional: true, Computed: true,
			Default: stringdefault.StaticString("Always"),
			Validators: []validator.String{
				stringvalidator.OneOf("Always", "Halted", "Manual", "RerunOnFailure", "Once"),
			},
			MarkdownDescription: "Requested running state of the VM instance.",
		},
	}
}

func vminstanceComputeAttributes() map[string]rschema.Attribute {
	return map[string]rschema.Attribute{
		"instance_type": rschema.StringAttribute{
			Optional: true, Computed: true,
			Default:             stringdefault.StaticString("u1.medium"),
			MarkdownDescription: "Virtual machine instance type.",
		},
		"instance_profile": rschema.StringAttribute{
			Optional: true, Computed: true,
			Default:             stringdefault.StaticString("ubuntu"),
			MarkdownDescription: "Virtual machine preferences profile.",
		},
		"disks": rschema.ListNestedAttribute{
			Optional:            true,
			MarkdownDescription: "Disks to attach.",
			NestedObject: rschema.NestedAttributeObject{
				Attributes: map[string]rschema.Attribute{
					"name": rschema.StringAttribute{Required: true, MarkdownDescription: "Disk name (references a `cozystack_vmdisk`)."},
					"bus":  rschema.StringAttribute{Optional: true, MarkdownDescription: "Disk bus type (e.g. `sata`, `virtio`)."},
				},
			},
		},
		"networks": vmNameListResourceAttribute("Networks to attach the VM to.", "Network attachment name."),
		"gpus":     vmNameListResourceAttribute("GPUs to attach (NVIDIA driver requires at least 4 GiB RAM).", "GPU resource name."),
		"cpu_model": rschema.StringAttribute{
			Optional: true, Computed: true,
			Default:             stringdefault.StaticString(""),
			MarkdownDescription: "CPU model exposed inside the VM.",
		},
		attrResources: rschema.SingleNestedAttribute{
			Optional:            true,
			MarkdownDescription: "Explicit CPU, memory, and socket configuration.",
			Attributes: map[string]rschema.Attribute{
				attrCPU:    rschema.StringAttribute{Optional: true, MarkdownDescription: "Number of CPU cores allocated."},
				attrMemory: rschema.StringAttribute{Optional: true, MarkdownDescription: "Amount of memory allocated."},
				"sockets":  rschema.StringAttribute{Optional: true, MarkdownDescription: "Number of CPU sockets (vCPU topology)."},
			},
		},
		"ssh_keys": rschema.ListAttribute{
			Optional:            true,
			ElementType:         types.StringType,
			MarkdownDescription: "SSH public keys for authentication.",
		},
		"cloud_init": rschema.StringAttribute{
			Optional: true, Computed: true,
			Default:             stringdefault.StaticString(""),
			MarkdownDescription: "Cloud-init user data.",
		},
		"cloud_init_seed": rschema.StringAttribute{
			Optional: true, Computed: true,
			Default:             stringdefault.StaticString(""),
			MarkdownDescription: "Seed string used to generate the SMBIOS UUID for the VM.",
		},
	}
}

func vminstanceOutputAttributes() map[string]rschema.Attribute {
	return map[string]rschema.Attribute{
		"ip_address": rschema.StringAttribute{
			Computed: true,
			MarkdownDescription: "Primary IP address of the running guest (from the backing " +
				"VirtualMachineInstance). Populated once the guest is up — set " +
				"`wait_for_ready = true` to have it available sooner.",
		},
		"ip_addresses": rschema.ListAttribute{
			Computed:            true,
			ElementType:         types.StringType,
			MarkdownDescription: "All IP addresses reported on the guest's primary interface.",
		},
	}
}

func vminstanceSchema() rschema.Schema {
	attributes := identityResourceAttributes("VMInstance instance name (`metadata.name`). Immutable.")

	maps.Copy(attributes, vminstanceAccessAttributes())
	maps.Copy(attributes, vminstanceComputeAttributes())
	maps.Copy(attributes, vminstanceOutputAttributes())
	maps.Copy(attributes, statusResourceAttributes())
	maps.Copy(attributes, waitBehaviorAttributes())

	return rschema.Schema{
		MarkdownDescription: "A Cozystack virtual machine instance, deployed inside a tenant namespace. " +
			"The deprecated subnets list is not managed (use networks instead).",
		Attributes: attributes,
	}
}

func vmNameListDataSourceAttribute(desc string) dsschema.ListNestedAttribute {
	return dsschema.ListNestedAttribute{
		Computed:            true,
		MarkdownDescription: desc,
		NestedObject: dsschema.NestedAttributeObject{
			Attributes: map[string]dsschema.Attribute{
				"name": dsschema.StringAttribute{Computed: true, MarkdownDescription: "Name."},
			},
		},
	}
}

func vminstanceDataSourceSchema() dsschema.Schema {
	attributes := identityDataSourceAttributes("VMInstance instance name.")

	maps.Copy(attributes, map[string]dsschema.Attribute{
		attrExternal:          dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether external access is enabled."},
		"external_method":     dsschema.StringAttribute{Computed: true, MarkdownDescription: "Traffic pass-through method."},
		"external_ports":      dsschema.ListAttribute{Computed: true, ElementType: types.Int64Type, MarkdownDescription: "Forwarded ports."},
		"external_allow_icmp": dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether ICMP is allowed."},
		"run_strategy":        dsschema.StringAttribute{Computed: true, MarkdownDescription: "Requested running state."},
		"instance_type":       dsschema.StringAttribute{Computed: true, MarkdownDescription: "Instance type."},
		"instance_profile":    dsschema.StringAttribute{Computed: true, MarkdownDescription: "Preferences profile."},
		"disks": dsschema.ListNestedAttribute{
			Computed:            true,
			MarkdownDescription: "Attached disks.",
			NestedObject: dsschema.NestedAttributeObject{
				Attributes: map[string]dsschema.Attribute{
					"name": dsschema.StringAttribute{Computed: true, MarkdownDescription: "Disk name."},
					"bus":  dsschema.StringAttribute{Computed: true, MarkdownDescription: "Disk bus type."},
				},
			},
		},
		"networks":  vmNameListDataSourceAttribute("Attached networks."),
		"gpus":      vmNameListDataSourceAttribute("Attached GPUs."),
		"cpu_model": dsschema.StringAttribute{Computed: true, MarkdownDescription: "CPU model."},
		attrResources: dsschema.SingleNestedAttribute{
			Computed:            true,
			MarkdownDescription: "Explicit CPU, memory, and socket configuration.",
			Attributes: map[string]dsschema.Attribute{
				attrCPU:    dsschema.StringAttribute{Computed: true, MarkdownDescription: "CPU cores."},
				attrMemory: dsschema.StringAttribute{Computed: true, MarkdownDescription: "Memory."},
				"sockets":  dsschema.StringAttribute{Computed: true, MarkdownDescription: "CPU sockets."},
			},
		},
		"ssh_keys":        dsschema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "SSH public keys."},
		"cloud_init":      dsschema.StringAttribute{Computed: true, MarkdownDescription: "Cloud-init user data."},
		"cloud_init_seed": dsschema.StringAttribute{Computed: true, MarkdownDescription: "SMBIOS UUID seed."},
		"ip_address":      dsschema.StringAttribute{Computed: true, MarkdownDescription: "Primary IP address of the running guest."},
		"ip_addresses":    dsschema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "All guest IP addresses."},
	})
	maps.Copy(attributes, statusDataSourceAttributes())

	return dsschema.Schema{
		MarkdownDescription: "Read an existing Cozystack VM instance by name and namespace.",
		Attributes:          attributes,
	}
}
