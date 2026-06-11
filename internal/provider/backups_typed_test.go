package provider

import (
	"context"
	"testing"

	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Vendored guard structs mirror the backups.cozystack.io CRD spec field names.
type planSpecGuard struct {
	ApplicationRef  map[string]any `json:"applicationRef"`
	BackupClassName string         `json:"backupClassName"`
	Schedule        map[string]any `json:"schedule"`
}

type restoreJobSpecGuard struct {
	BackupRef            map[string]any `json:"backupRef"`
	TargetApplicationRef map[string]any `json:"targetApplicationRef,omitempty"`
	Options              map[string]any `json:"options,omitempty"`
}

func appRefValue(group, kind, name string) types.Object {
	return types.ObjectValueMust(appRefObjectType(), map[string]attr.Value{
		"api_group": types.StringValue(group),
		"kind":      types.StringValue(kind),
		"name":      types.StringValue(name),
	})
}

func fullBackupPlanModel() backupPlanModel {
	return backupPlanModel{
		Name:            types.StringValue("daily"),
		Namespace:       types.StringValue("tenant-root"),
		ApplicationRef:  appRefValue("apps.cozystack.io", "Postgres", "db"),
		BackupClassName: types.StringValue("s3-daily"),
		Schedule: types.ObjectValueMust(scheduleObjectType(), map[string]attr.Value{
			"cron": types.StringValue("0 3 * * *"),
			"type": types.StringValue("Cron"),
		}),
	}
}

func TestBackupPlanExpandFlatten(t *testing.T) {
	t.Parallel()

	model := fullBackupPlanModel()

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	ref, _ := got.Spec["applicationRef"].(map[string]any)
	if ref["kind"] != "Postgres" || got.Spec["backupClassName"] != "s3-daily" {
		t.Errorf("spec = %v, want Postgres ref + s3-daily class", got.Spec)
	}

	app := &client.Application{
		Name: "daily", Namespace: "tenant-root",
		Spec: map[string]any{
			"applicationRef":  map[string]any{"apiGroup": "apps.cozystack.io", "kind": "Postgres", "name": "db"},
			"backupClassName": "s3-daily",
			"schedule":        map[string]any{"cron": "0 3 * * *", "type": "Cron"},
		},
	}

	var out backupPlanModel
	if diags := out.flatten(app); diags.HasError() {
		t.Fatalf("flatten diagnostics: %v", diags)
	}

	if out.BackupClassName.ValueString() != "s3-daily" {
		t.Errorf("backup_class_name = %q, want s3-daily", out.BackupClassName.ValueString())
	}
}

func TestBackupPlanGuard(t *testing.T) {
	t.Parallel()

	model := fullBackupPlanModel()

	got, _ := model.expand(context.Background())

	emitted := map[string]bool{}
	for key := range got.Spec {
		emitted[key] = true
	}

	assertSpecCoverage(t, emitted, planSpecGuard{})
}

func TestRestoreJobExpandGuard(t *testing.T) {
	t.Parallel()

	model := restoreJobModel{
		Name:                 types.StringValue("restore-1"),
		Namespace:            types.StringValue("tenant-root"),
		BackupName:           types.StringValue("daily-20260602"),
		TargetApplicationRef: appRefValue("apps.cozystack.io", "Postgres", "db"),
		Options:              jsontypes.NewNormalizedValue(`{"overwrite":true}`),
	}

	got, diags := model.expand(context.Background())
	if diags.HasError() {
		t.Fatalf("expand diagnostics: %v", diags)
	}

	backupRef, _ := got.Spec["backupRef"].(map[string]any)
	if backupRef["name"] != "daily-20260602" {
		t.Errorf("backupRef.name = %v, want daily-20260602", backupRef["name"])
	}

	emitted := map[string]bool{}
	for key := range got.Spec {
		emitted[key] = true
	}

	assertSpecCoverage(t, emitted, restoreJobSpecGuard{})
}
