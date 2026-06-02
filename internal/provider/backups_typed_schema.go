package provider

import (
	"maps"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func appRefResourceAttribute(desc string, required bool) rschema.SingleNestedAttribute {
	return rschema.SingleNestedAttribute{
		Required:            required,
		Optional:            !required,
		MarkdownDescription: desc,
		Attributes: map[string]rschema.Attribute{
			"api_group": rschema.StringAttribute{Optional: true, MarkdownDescription: "API group of the referenced application."},
			"kind":      rschema.StringAttribute{Required: true, MarkdownDescription: "Kind of the referenced application."},
			"name":      rschema.StringAttribute{Required: true, MarkdownDescription: "Name of the referenced application."},
		},
	}
}

func appRefDataSourceAttribute(desc string) dsschema.SingleNestedAttribute {
	return dsschema.SingleNestedAttribute{
		Computed:            true,
		MarkdownDescription: desc,
		Attributes: map[string]dsschema.Attribute{
			"api_group": dsschema.StringAttribute{Computed: true, MarkdownDescription: "API group."},
			"kind":      dsschema.StringAttribute{Computed: true, MarkdownDescription: "Kind."},
			"name":      dsschema.StringAttribute{Computed: true, MarkdownDescription: "Name."},
		},
	}
}

func backupPlanSchema() rschema.Schema {
	attributes := identityResourceAttributes("Plan name (`metadata.name`). Immutable.")

	maps.Copy(attributes, map[string]rschema.Attribute{
		"application_ref":   appRefResourceAttribute("Application to back up.", true),
		"backup_class_name": rschema.StringAttribute{Required: true, MarkdownDescription: "BackupClass that provides the backup strategy."},
		"schedule": rschema.SingleNestedAttribute{
			Required:            true,
			MarkdownDescription: "When backup copies are created.",
			Attributes: map[string]rschema.Attribute{
				"cron": rschema.StringAttribute{Optional: true, MarkdownDescription: "Cron expression."},
				"type": rschema.StringAttribute{Optional: true, MarkdownDescription: "Schedule type."},
			},
		},
	})
	maps.Copy(attributes, statusResourceAttributes())
	maps.Copy(attributes, waitBehaviorAttributes())

	return rschema.Schema{
		MarkdownDescription: "A Cozystack backup Plan: a schedule that backs up a tenant application using a BackupClass strategy.",
		Attributes:          attributes,
	}
}

func backupPlanDataSourceSchema() dsschema.Schema {
	attributes := identityDataSourceAttributes("Plan name.")

	maps.Copy(attributes, map[string]dsschema.Attribute{
		"application_ref":   appRefDataSourceAttribute("Application backed up."),
		"backup_class_name": dsschema.StringAttribute{Computed: true, MarkdownDescription: "BackupClass providing the strategy."},
		"schedule": dsschema.SingleNestedAttribute{
			Computed:            true,
			MarkdownDescription: "Backup schedule.",
			Attributes: map[string]dsschema.Attribute{
				"cron": dsschema.StringAttribute{Computed: true, MarkdownDescription: "Cron expression."},
				"type": dsschema.StringAttribute{Computed: true, MarkdownDescription: "Schedule type."},
			},
		},
	})
	maps.Copy(attributes, statusDataSourceAttributes())

	return dsschema.Schema{
		MarkdownDescription: "Read a Cozystack backup Plan by name and namespace.",
		Attributes:          attributes,
	}
}

func restoreJobSchema() rschema.Schema {
	attributes := identityResourceAttributes("RestoreJob name (`metadata.name`). Immutable.")

	maps.Copy(attributes, map[string]rschema.Attribute{
		"backup_name":            rschema.StringAttribute{Required: true, MarkdownDescription: "Name of the Backup to restore."},
		"target_application_ref": appRefResourceAttribute("Application to restore into.", false),
		"options": rschema.StringAttribute{
			Optional:            true,
			CustomType:          jsontypes.NormalizedType{},
			MarkdownDescription: "Driver-specific restore options as JSON.",
		},
	})
	maps.Copy(attributes, statusResourceAttributes())
	maps.Copy(attributes, waitBehaviorAttributes())

	return rschema.Schema{
		MarkdownDescription: "A Cozystack RestoreJob: restores a Backup into a tenant application.",
		Attributes:          attributes,
	}
}

func restoreJobDataSourceSchema() dsschema.Schema {
	attributes := identityDataSourceAttributes("RestoreJob name.")

	maps.Copy(attributes, map[string]dsschema.Attribute{
		"backup_name":            dsschema.StringAttribute{Computed: true, MarkdownDescription: "Backup restored."},
		"target_application_ref": appRefDataSourceAttribute("Application restored into."),
		"options":                dsschema.StringAttribute{Computed: true, CustomType: jsontypes.NormalizedType{}, MarkdownDescription: "Driver-specific restore options as JSON."},
	})
	maps.Copy(attributes, statusDataSourceAttributes())

	return dsschema.Schema{
		MarkdownDescription: "Read a Cozystack RestoreJob by name and namespace.",
		Attributes:          attributes,
	}
}
