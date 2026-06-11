package provider

import (
	"context"
	"encoding/json"

	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// appRefObjectType is the {apiGroup,kind,name} reference shared by the backups
// kinds to point at a managed application.
func appRefObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"api_group": types.StringType,
		"kind":      types.StringType,
		"name":      types.StringType,
	}
}

type appRefData struct {
	APIGroup types.String `tfsdk:"api_group"`
	Kind     types.String `tfsdk:"kind"`
	Name     types.String `tfsdk:"name"`
}

func expandAppRef(ctx context.Context, value types.Object) (map[string]any, diag.Diagnostics) {
	var data appRefData

	diags := value.As(ctx, &data, basetypes.ObjectAsOptions{})

	return map[string]any{
		"apiGroup": data.APIGroup.ValueString(),
		"kind":     data.Kind.ValueString(),
		"name":     data.Name.ValueString(),
	}, diags
}

func flattenAppRef(raw any) types.Object {
	ref, ok := raw.(map[string]any)
	if !ok {
		return types.ObjectNull(appRefObjectType())
	}

	return types.ObjectValueMust(appRefObjectType(), map[string]attr.Value{
		"api_group": types.StringValue(specString(ref, "apiGroup")),
		"kind":      types.StringValue(specString(ref, "kind")),
		"name":      types.StringValue(specString(ref, "name")),
	})
}

// backupPlanModel maps cozystack_backup_plan: a backup schedule for an app.
type backupPlanModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Namespace       types.String `tfsdk:"namespace"`
	ApplicationRef  types.Object `tfsdk:"application_ref"`
	BackupClassName types.String `tfsdk:"backup_class_name"`
	Schedule        types.Object `tfsdk:"schedule"`
	Ready           types.Bool   `tfsdk:"ready"`
	UID             types.String `tfsdk:"uid"`
	ChartVersion    types.String `tfsdk:"chart_version"`
}

type backupPlanResourceModel struct {
	backupPlanModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

func (m *backupPlanResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

func scheduleObjectType() map[string]attr.Type {
	return map[string]attr.Type{"cron": types.StringType, "type": types.StringType}
}

type scheduleData struct {
	Cron types.String `tfsdk:"cron"`
	Type types.String `tfsdk:"type"`
}

func (m *backupPlanModel) identity() (string, string) {
	return m.Namespace.ValueString(), m.Name.ValueString()
}

func (m *backupPlanModel) expand(ctx context.Context) (*client.Application, diag.Diagnostics) {
	var diags diag.Diagnostics

	appRef, aDiags := expandAppRef(ctx, m.ApplicationRef)
	diags.Append(aDiags...)

	var schedule scheduleData

	diags.Append(m.Schedule.As(ctx, &schedule, basetypes.ObjectAsOptions{})...)

	if diags.HasError() {
		return nil, diags
	}

	spec := map[string]any{
		"applicationRef":  appRef,
		"backupClassName": m.BackupClassName.ValueString(),
		"schedule":        map[string]any{"cron": schedule.Cron.ValueString(), "type": schedule.Type.ValueString()},
	}

	return &client.Application{Name: m.Name.ValueString(), Namespace: m.Namespace.ValueString(), Spec: spec}, diags
}

func (m *backupPlanModel) flatten(app *client.Application) diag.Diagnostics {
	m.ID = types.StringValue(app.Namespace + "/" + app.Name)
	m.Name = types.StringValue(app.Name)
	m.Namespace = types.StringValue(app.Namespace)
	m.ApplicationRef = flattenAppRef(app.Spec["applicationRef"])
	m.BackupClassName = types.StringValue(specString(app.Spec, "backupClassName"))

	schedule, _ := app.Spec["schedule"].(map[string]any)
	m.Schedule = types.ObjectValueMust(scheduleObjectType(), map[string]attr.Value{
		"cron": types.StringValue(specString(schedule, "cron")),
		"type": types.StringValue(specString(schedule, "type")),
	})

	m.Ready = types.BoolValue(app.Status.Ready)
	m.UID = types.StringValue(app.UID)
	m.ChartVersion = types.StringValue(app.Status.Version)

	return nil
}

// restoreJobModel maps cozystack_restore_job: restore a backup into an app.
type restoreJobModel struct {
	ID                   types.String         `tfsdk:"id"`
	Name                 types.String         `tfsdk:"name"`
	Namespace            types.String         `tfsdk:"namespace"`
	BackupName           types.String         `tfsdk:"backup_name"`
	TargetApplicationRef types.Object         `tfsdk:"target_application_ref"`
	Options              jsontypes.Normalized `tfsdk:"options"`
	Ready                types.Bool           `tfsdk:"ready"`
	UID                  types.String         `tfsdk:"uid"`
	ChartVersion         types.String         `tfsdk:"chart_version"`
}

type restoreJobResourceModel struct {
	restoreJobModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

func (m *restoreJobResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

func (m *restoreJobModel) identity() (string, string) {
	return m.Namespace.ValueString(), m.Name.ValueString()
}

func (m *restoreJobModel) expand(ctx context.Context) (*client.Application, diag.Diagnostics) {
	var diags diag.Diagnostics

	spec := map[string]any{"backupRef": map[string]any{"name": m.BackupName.ValueString()}}

	if !m.TargetApplicationRef.IsNull() && !m.TargetApplicationRef.IsUnknown() {
		target, tDiags := expandAppRef(ctx, m.TargetApplicationRef)
		diags.Append(tDiags...)

		spec["targetApplicationRef"] = target
	}

	if !m.Options.IsNull() && !m.Options.IsUnknown() {
		var options map[string]any
		if err := json.Unmarshal([]byte(m.Options.ValueString()), &options); err != nil {
			diags.AddError("Invalid options JSON", err.Error())

			return nil, diags
		}

		spec["options"] = options
	}

	return &client.Application{Name: m.Name.ValueString(), Namespace: m.Namespace.ValueString(), Spec: spec}, diags
}

func (m *restoreJobModel) flatten(app *client.Application) diag.Diagnostics {
	m.ID = types.StringValue(app.Namespace + "/" + app.Name)
	m.Name = types.StringValue(app.Name)
	m.Namespace = types.StringValue(app.Namespace)

	backupRef, _ := app.Spec["backupRef"].(map[string]any)
	m.BackupName = types.StringValue(specString(backupRef, "name"))
	m.TargetApplicationRef = flattenAppRef(app.Spec["targetApplicationRef"])
	m.Options = flattenJSONMap(app.Spec["options"])

	m.Ready = types.BoolValue(app.Status.Ready)
	m.UID = types.StringValue(app.UID)
	m.ChartVersion = types.StringValue(app.Status.Version)

	return nil
}
