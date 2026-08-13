package provider

import (
	"maps"

	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
)

func vmdiskSourceResourceAttribute() rschema.SingleNestedAttribute {
	return rschema.SingleNestedAttribute{
		Required: true,
		MarkdownDescription: "Source image used to create the disk. Set exactly one of " +
			"`disk`, `http`, or `image`.",
		Attributes: map[string]rschema.Attribute{
			"disk": rschema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Clone an existing vm-disk.",
				Attributes: map[string]rschema.Attribute{
					"name": rschema.StringAttribute{Required: true, MarkdownDescription: "Name of the vm-disk to clone."},
				},
			},
			"http": rschema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Download an image from an HTTP source.",
				Attributes: map[string]rschema.Attribute{
					"url": rschema.StringAttribute{Required: true, MarkdownDescription: "URL to download the image."},
				},
			},
			"image": rschema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Use an image by name from the default collection.",
				Attributes: map[string]rschema.Attribute{
					"name": rschema.StringAttribute{Required: true, MarkdownDescription: "Name of the image to use."},
				},
			},
		},
	}
}

func vmdiskSchema() rschema.Schema {
	attributes := identityResourceAttributes("VMDisk instance name (`metadata.name`). Immutable.")

	maps.Copy(attributes, map[string]rschema.Attribute{
		"source": vmdiskSourceResourceAttribute(),
		"optical": rschema.BoolAttribute{
			Optional: true, Computed: true,
			Default:             booldefault.StaticBool(false),
			MarkdownDescription: "Whether the disk should be considered optical.",
		},
		"storage": rschema.StringAttribute{
			Optional: true, Computed: true,
			Default:             stringdefault.StaticString("5Gi"),
			MarkdownDescription: "Disk size allocated for the virtual machine (quantity, e.g. `5Gi`).",
		},
		attrStorageClass: storageClassAttribute("replicated"),
	})
	maps.Copy(attributes, statusResourceAttributes())
	maps.Copy(attributes, waitBehaviorAttributes())

	return rschema.Schema{
		MarkdownDescription: "A Cozystack VM disk, deployed inside a tenant namespace. " +
			"The interactive upload source is not managed.",
		Attributes: attributes,
	}
}

func vmdiskSourceDataSourceAttribute() dsschema.SingleNestedAttribute {
	nameObject := func(desc string) dsschema.SingleNestedAttribute {
		return dsschema.SingleNestedAttribute{
			Computed:            true,
			MarkdownDescription: desc,
			Attributes: map[string]dsschema.Attribute{
				"name": dsschema.StringAttribute{Computed: true, MarkdownDescription: "Image or disk name."},
			},
		}
	}

	return dsschema.SingleNestedAttribute{
		Computed:            true,
		MarkdownDescription: "Source image used to create the disk.",
		Attributes: map[string]dsschema.Attribute{
			"disk": nameObject("Cloned vm-disk source."),
			"http": dsschema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "HTTP download source.",
				Attributes: map[string]dsschema.Attribute{
					"url": dsschema.StringAttribute{Computed: true, MarkdownDescription: "Image download URL."},
				},
			},
			"image": nameObject("Named image source."),
		},
	}
}

func vmdiskDataSourceSchema() dsschema.Schema {
	attributes := identityDataSourceAttributes("VMDisk instance name.")

	maps.Copy(attributes, map[string]dsschema.Attribute{
		"source":         vmdiskSourceDataSourceAttribute(),
		"optical":        dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the disk is optical."},
		"storage":        dsschema.StringAttribute{Computed: true, MarkdownDescription: "Disk size."},
		attrStorageClass: dsschema.StringAttribute{Computed: true, MarkdownDescription: "StorageClass used to store the data."},
	})
	maps.Copy(attributes, statusDataSourceAttributes())

	return dsschema.Schema{
		MarkdownDescription: "Read an existing Cozystack VM disk by name and namespace.",
		Attributes:          attributes,
	}
}
