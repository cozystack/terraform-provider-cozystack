package provider

import (
	"context"

	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// postgresqlModel maps the cozystack_postgresql schema to Go types. The
// postgresql tuning, quorum, and bootstrap blocks are not managed (they use
// server defaults), and of the backup block only the system-bucket opt-in is:
// every other backup field is deprecated upstream in favour of the
// platform-managed default BackupClass.
type postgresqlModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Namespace       types.String `tfsdk:"namespace"`
	Replicas        types.Int64  `tfsdk:"replicas"`
	Resources       types.Object `tfsdk:"resources"`
	ResourcesPreset types.String `tfsdk:"resources_preset"`
	Size            types.String `tfsdk:"size"`
	StorageClass    types.String `tfsdk:"storage_class"`
	External        types.Bool   `tfsdk:"external"`
	TLS             types.Object `tfsdk:"tls"`
	Backup          types.Object `tfsdk:"backup"`
	Version         types.String `tfsdk:"version"`
	Users           types.Map    `tfsdk:"users"`
	Databases       types.Map    `tfsdk:"databases"`
	Ready           types.Bool   `tfsdk:"ready"`
	ChartVersion    types.String `tfsdk:"chart_version"`
	UID             types.String `tfsdk:"uid"`
	Endpoints       types.Object `tfsdk:"endpoints"`
}

func pgConnectionObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"host":      types.StringType,
		"read_host": types.StringType,
		"port":      types.Int64Type,
	}
}

type postgresqlResourceModel struct {
	postgresqlModel

	WaitForReady types.Bool   `tfsdk:"wait_for_ready"`
	WaitTimeout  types.String `tfsdk:"wait_timeout"`
}

func (m *postgresqlResourceModel) waitConfig() (types.Bool, types.String) {
	return m.WaitForReady, m.WaitTimeout
}

func (m *postgresqlModel) identity() (string, string) {
	return m.Namespace.ValueString(), m.Name.ValueString()
}

func pgUserObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"password":    types.StringType,
		"replication": types.BoolType,
	}
}

func pgDatabaseObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"extensions": types.ListType{ElemType: types.StringType},
		"roles":      types.ObjectType{AttrTypes: rolesObjectType()},
	}
}

type pgUserData struct {
	Password    types.String `tfsdk:"password"`
	Replication types.Bool   `tfsdk:"replication"`
}

type pgDatabaseData struct {
	Extensions []string  `tfsdk:"extensions"`
	Roles      rolesData `tfsdk:"roles"`
}

func expandPgUsers(ctx context.Context, value types.Map) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	out := map[string]any{}

	if value.IsNull() || value.IsUnknown() {
		return out, diags
	}

	elements := map[string]pgUserData{}
	diags.Append(value.ElementsAs(ctx, &elements, false)...)

	if diags.HasError() {
		return out, diags
	}

	for name, user := range elements {
		entry := map[string]any{"replication": user.Replication.ValueBool()}
		if password := user.Password.ValueString(); password != "" {
			entry["password"] = password
		}

		out[name] = entry
	}

	return out, diags
}

func flattenPgUsers(raw any) (types.Map, diag.Diagnostics) {
	return flattenObjectMap(raw, pgUserObjectType(), func(user map[string]any) map[string]attr.Value {
		password, _ := user["password"].(string)
		replication, _ := user["replication"].(bool)

		return map[string]attr.Value{
			"password":    types.StringValue(password),
			"replication": types.BoolValue(replication),
		}
	})
}

func expandPgDatabases(ctx context.Context, value types.Map) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	out := map[string]any{}

	if value.IsNull() || value.IsUnknown() {
		return out, diags
	}

	elements := map[string]pgDatabaseData{}
	diags.Append(value.ElementsAs(ctx, &elements, false)...)

	if diags.HasError() {
		return out, diags
	}

	for name, database := range elements {
		entry := map[string]any{}
		if len(database.Extensions) > 0 {
			entry["extensions"] = stringsToAny(database.Extensions)
		}

		roles := map[string]any{}
		if len(database.Roles.Admin) > 0 {
			roles["admin"] = stringsToAny(database.Roles.Admin)
		}

		if len(database.Roles.Readonly) > 0 {
			roles["readonly"] = stringsToAny(database.Roles.Readonly)
		}

		entry["roles"] = roles
		out[name] = entry
	}

	return out, diags
}

func flattenPgDatabases(raw any) (types.Map, diag.Diagnostics) {
	return flattenObjectMap(raw, pgDatabaseObjectType(), func(database map[string]any) map[string]attr.Value {
		roles, _ := database["roles"].(map[string]any)

		rolesObject := types.ObjectValueMust(rolesObjectType(), map[string]attr.Value{
			"admin":    stringListOrNull(roles["admin"]),
			"readonly": stringListOrNull(roles["readonly"]),
		})

		return map[string]attr.Value{
			"extensions": stringListOrNull(database["extensions"]),
			"roles":      rolesObject,
		}
	})
}

func (m *postgresqlModel) expand(ctx context.Context) (*client.Application, diag.Diagnostics) {
	var diags diag.Diagnostics

	resources, rDiags := expandResources(ctx, m.Resources)
	diags.Append(rDiags...)

	users, uDiags := expandPgUsers(ctx, m.Users)
	diags.Append(uDiags...)

	databases, dDiags := expandPgDatabases(ctx, m.Databases)
	diags.Append(dDiags...)

	if diags.HasError() {
		return nil, diags
	}

	spec := map[string]any{
		attrReplicas:        m.Replicas.ValueInt64(),
		attrResources:       resources,
		specResourcesPreset: m.ResourcesPreset.ValueString(),
		attrSize:            m.Size.ValueString(),
		specStorageClass:    m.StorageClass.ValueString(),
		attrExternal:        m.External.ValueBool(),
		attrVersion:         m.Version.ValueString(),
		"users":             users,
		"databases":         databases,
	}

	setOptionalTLS(spec, m.TLS)
	setOptionalBackup(spec, m.Backup)

	return &client.Application{
		Name:      m.Name.ValueString(),
		Namespace: m.Namespace.ValueString(),
		Spec:      spec,
	}, diags
}

// readOutputs reads the CNPG connection endpoints, which the operator exposes as
// the Services `postgres-<name>-rw` (primary) and `postgres-<name>-ro` (replicas).
// They appear asynchronously, so an absent primary Service leaves endpoints null.
func (m *postgresqlModel) readOutputs(ctx context.Context, api *client.Client) diag.Diagnostics {
	var diags diag.Diagnostics

	m.Endpoints = types.ObjectNull(pgConnectionObjectType())

	namespace, name := m.identity()
	release := "postgres-" + name

	primary, found, err := api.GetServiceEndpoint(ctx, namespace, release+"-rw")
	if err != nil {
		diags.AddError("Unable to read PostgreSQL connection endpoint", err.Error())

		return diags
	}

	if !found {
		return diags
	}

	readHost := types.StringNull()

	replica, replicaFound, replicaErr := api.GetServiceEndpoint(ctx, namespace, release+"-ro")
	if replicaErr != nil {
		diags.AddError("Unable to read PostgreSQL read endpoint", replicaErr.Error())

		return diags
	}

	if replicaFound {
		readHost = types.StringValue(replica.Host)
	}

	m.Endpoints = types.ObjectValueMust(pgConnectionObjectType(), map[string]attr.Value{
		"host":      types.StringValue(primary.Host),
		"read_host": readHost,
		"port":      types.Int64Value(primary.Port),
	})

	return diags
}

func (m *postgresqlModel) flatten(app *client.Application) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(app.Namespace + "/" + app.Name)
	m.Name = types.StringValue(app.Name)
	m.Namespace = types.StringValue(app.Namespace)
	m.Replicas = types.Int64Value(specInt64(app.Spec, attrReplicas))
	m.ResourcesPreset = types.StringValue(specString(app.Spec, specResourcesPreset))
	m.Size = types.StringValue(specString(app.Spec, attrSize))
	m.StorageClass = types.StringValue(specString(app.Spec, specStorageClass))
	m.External = types.BoolValue(specBool(app.Spec, attrExternal))
	m.TLS = flattenTLS(app.Spec[specTLS])
	m.Backup = flattenBackup(app.Spec[specBackup])
	m.Version = types.StringValue(specString(app.Spec, attrVersion))

	resources, rDiags := flattenResources(app.Spec[attrResources])
	diags.Append(rDiags...)

	m.Resources = resources

	users, uDiags := flattenPgUsers(app.Spec["users"])
	diags.Append(uDiags...)

	m.Users = users

	databases, dDiags := flattenPgDatabases(app.Spec["databases"])
	diags.Append(dDiags...)

	m.Databases = databases

	m.Ready = types.BoolValue(app.Status.Ready)
	m.ChartVersion = types.StringValue(app.Status.Version)
	m.UID = types.StringValue(app.UID)

	return diags
}
