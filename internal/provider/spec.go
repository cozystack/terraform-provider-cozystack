package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// Attribute and spec-key names shared across resource kinds. Where a spec key
// equals the Terraform attribute name, one constant serves schema, expand, and
// flatten alike.
const (
	attrID        = "id"
	attrName      = "name"
	attrNamespace = "namespace"
	attrReady     = "ready"
	attrReplicas  = "replicas"
	attrSize      = "size"
	attrExternal  = "external"
	attrVersion   = "version"
	attrResources = "resources"
	attrCPU       = "cpu"
	attrMemory    = "memory"
)

// Attribute names (snake_case) and their differing camelCase spec keys, shared
// by the managed stateful-workload kinds (redis, qdrant, …).
const (
	attrStorageClass    = "storage_class"
	attrResourcesPreset = "resources_preset"
	attrChartVersion    = "chart_version"
	attrUID             = "uid"
	attrAuthEnabled     = "auth_enabled"

	specStorageClass    = "storageClass"
	specResourcesPreset = "resourcesPreset"
	specAuthEnabled     = "authEnabled"
)

// Attribute and spec-key names of the shared tls and backup blocks.
const (
	attrEnabled         = "enabled"
	attrUseSystemBucket = "use_system_bucket"

	specTLS             = "tls"
	specBackup          = "backup"
	specUseSystemBucket = "useSystemBucket"
)

// Shared helpers for reading and writing the free-form application spec across
// Cozystack resource kinds.

func specString(spec map[string]any, key string) string {
	if v, ok := spec[key].(string); ok {
		return v
	}

	return ""
}

func specBool(spec map[string]any, key string) bool {
	if v, ok := spec[key].(bool); ok {
		return v
	}

	return false
}

// specInt64 reads an integer spec field, tolerating the int64/float64 forms that
// JSON decoding and the unstructured converter may produce.
func specInt64(spec map[string]any, key string) int64 {
	switch v := spec[key].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return 0
	}
}

// rawStatusString reads a string field from an application's raw status object.
func rawStatusString(raw map[string]any, key string) string {
	if v, ok := raw[key].(string); ok {
		return v
	}

	return ""
}

// resourcesObjectType is the attribute schema of the resources nested object
// shared by sized application kinds.
func resourcesObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		attrCPU:    types.StringType,
		attrMemory: types.StringType,
	}
}

// expandResources renders the resources nested object into a spec submap,
// omitting empty sub-fields so the server's resourcesPreset stays in effect.
func expandResources(ctx context.Context, obj types.Object) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	out := map[string]any{}

	if obj.IsNull() || obj.IsUnknown() {
		return out, diags
	}

	var data struct {
		CPU    types.String `tfsdk:"cpu"`
		Memory types.String `tfsdk:"memory"`
	}

	diags.Append(obj.As(ctx, &data, basetypes.ObjectAsOptions{})...)

	if diags.HasError() {
		return out, diags
	}

	if v := data.CPU.ValueString(); v != "" {
		out[attrCPU] = v
	}

	if v := data.Memory.ValueString(); v != "" {
		out[attrMemory] = v
	}

	return out, diags
}

// flattenResources builds the resources nested object from a spec submap. An
// empty or absent block flattens to null so an unset block does not drift.
func flattenResources(raw any) (types.Object, diag.Diagnostics) {
	quantities, ok := raw.(map[string]any)
	if !ok || len(quantities) == 0 {
		return types.ObjectNull(resourcesObjectType()), nil
	}

	attributes := map[string]attr.Value{
		attrCPU:    optionalSpecString(quantities, attrCPU),
		attrMemory: optionalSpecString(quantities, attrMemory),
	}

	return types.ObjectValue(resourcesObjectType(), attributes)
}

func optionalSpecString(spec map[string]any, key string) types.String {
	if v, ok := spec[key].(string); ok && v != "" {
		return types.StringValue(v)
	}

	return types.StringNull()
}

// expandStringList renders a list of strings into a spec value.
func expandStringList(ctx context.Context, value types.List) ([]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	out := []any{}

	if value.IsNull() || value.IsUnknown() {
		return out, diags
	}

	var items []string
	diags.Append(value.ElementsAs(ctx, &items, false)...)

	for _, item := range items {
		out = append(out, item)
	}

	return out, diags
}

// flattenStringList builds a string list from a spec value. An empty or absent
// list flattens to null so an unset list does not drift.
func flattenStringList(raw any) (types.List, diag.Diagnostics) {
	return stringListOrNull(raw), nil
}

// stringListOrNull converts a spec []any into a Terraform string list, or null
// when empty. It is diagnostic-free for use inside object builders.
func stringListOrNull(raw any) types.List {
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return types.ListNull(types.StringType)
	}

	elements := make([]attr.Value, 0, len(items))
	for _, item := range items {
		text, _ := item.(string)
		elements = append(elements, types.StringValue(text))
	}

	return types.ListValueMust(types.StringType, elements)
}

// rolesObjectType is the {admin,readonly} object shared by database/vhost roles.
func rolesObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"admin":    types.ListType{ElemType: types.StringType},
		"readonly": types.ListType{ElemType: types.StringType},
	}
}

// rolesEntryObjectType is the {roles} map-value object shared by databases and
// vhosts.
func rolesEntryObjectType() map[string]attr.Type {
	return map[string]attr.Type{"roles": types.ObjectType{AttrTypes: rolesObjectType()}}
}

type rolesData struct {
	Admin    []string `tfsdk:"admin"`
	Readonly []string `tfsdk:"readonly"`
}

type rolesEntry struct {
	Roles rolesData `tfsdk:"roles"`
}

// expandRolesMap renders a map of {roles{admin,readonly}} entries into a spec
// submap, omitting empty role lists.
func expandRolesMap(ctx context.Context, value types.Map) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	out := map[string]any{}

	if value.IsNull() || value.IsUnknown() {
		return out, diags
	}

	elements := map[string]rolesEntry{}
	diags.Append(value.ElementsAs(ctx, &elements, false)...)

	if diags.HasError() {
		return out, diags
	}

	for name, entry := range elements {
		roles := map[string]any{}
		if len(entry.Roles.Admin) > 0 {
			roles["admin"] = stringsToAny(entry.Roles.Admin)
		}

		if len(entry.Roles.Readonly) > 0 {
			roles["readonly"] = stringsToAny(entry.Roles.Readonly)
		}

		out[name] = map[string]any{"roles": roles}
	}

	return out, diags
}

// flattenRolesMap builds a roles map from a spec submap.
func flattenRolesMap(raw any) (types.Map, diag.Diagnostics) {
	return flattenObjectMap(raw, rolesEntryObjectType(), func(entry map[string]any) map[string]attr.Value {
		roles, _ := entry["roles"].(map[string]any)

		rolesObject := types.ObjectValueMust(rolesObjectType(), map[string]attr.Value{
			"admin":    stringListOrNull(roles["admin"]),
			"readonly": stringListOrNull(roles["readonly"]),
		})

		return map[string]attr.Value{"roles": rolesObject}
	})
}

func stringsToAny(items []string) []any {
	out := make([]any, len(items))
	for i, item := range items {
		out[i] = item
	}

	return out
}

// passwordUserObjectType is the object type of a {password} user.
func passwordUserObjectType() map[string]attr.Type {
	return map[string]attr.Type{"password": types.StringType}
}

type passwordUserModel struct {
	Password types.String `tfsdk:"password"`
}

// expandPasswordUsers renders a map of {password} users into a spec submap,
// omitting empty passwords so the server can autogenerate them.
func expandPasswordUsers(ctx context.Context, value types.Map) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	out := map[string]any{}

	if value.IsNull() || value.IsUnknown() {
		return out, diags
	}

	elements := map[string]passwordUserModel{}
	diags.Append(value.ElementsAs(ctx, &elements, false)...)

	if diags.HasError() {
		return out, diags
	}

	for name, user := range elements {
		entry := map[string]any{}
		if password := user.Password.ValueString(); password != "" {
			entry["password"] = password
		}

		out[name] = entry
	}

	return out, diags
}

// flattenPasswordUsers builds the users map from a spec submap. An empty or
// absent map flattens to null.
func flattenPasswordUsers(raw any) (types.Map, diag.Diagnostics) {
	return flattenObjectMap(raw, passwordUserObjectType(), func(user map[string]any) map[string]attr.Value {
		password, _ := user["password"].(string)

		return map[string]attr.Value{"password": types.StringValue(password)}
	})
}

// flattenObjectMap builds a Terraform map of named objects from a spec submap,
// delegating per-element conversion to build. An empty or absent map flattens
// to null so an unset block does not drift.
func flattenObjectMap(
	raw any,
	objectType map[string]attr.Type,
	build func(map[string]any) map[string]attr.Value,
) (types.Map, diag.Diagnostics) {
	var diags diag.Diagnostics

	elementType := types.ObjectType{AttrTypes: objectType}

	items, ok := raw.(map[string]any)
	if !ok || len(items) == 0 {
		return types.MapNull(elementType), diags
	}

	elements := make(map[string]attr.Value, len(items))

	for name, raw := range items {
		item, _ := raw.(map[string]any)

		object, objectDiags := types.ObjectValue(objectType, build(item))
		diags.Append(objectDiags...)

		elements[name] = object
	}

	value, mapDiags := types.MapValue(elementType, elements)
	diags.Append(mapDiags...)

	return value, diags
}

// expandObjectList decodes a Terraform list of nested objects into a []any of
// spec maps, delegating per-element conversion to build. A null or unknown list
// yields an empty slice.
func expandObjectList[T any](
	ctx context.Context,
	value types.List,
	build func(T) map[string]any,
) ([]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	out := []any{}

	if value.IsNull() || value.IsUnknown() {
		return out, diags
	}

	var items []T

	diags.Append(value.ElementsAs(ctx, &items, false)...)

	if diags.HasError() {
		return out, diags
	}

	for _, item := range items {
		out = append(out, build(item))
	}

	return out, diags
}

// expandIntList decodes a Terraform list of integers into a []any. A null or
// unknown list yields an empty slice.
func expandIntList(ctx context.Context, value types.List) ([]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	out := []any{}

	if value.IsNull() || value.IsUnknown() {
		return out, diags
	}

	var items []int64

	diags.Append(value.ElementsAs(ctx, &items, false)...)

	for _, item := range items {
		out = append(out, item)
	}

	return out, diags
}

// flattenIntList builds an integer list from a spec value. An empty or absent
// list flattens to null so an unset list does not drift.
func flattenIntList(raw any) types.List {
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return types.ListNull(types.Int64Type)
	}

	elements := make([]attr.Value, 0, len(items))
	for _, item := range items {
		elements = append(elements, types.Int64Value(anyToInt64(item)))
	}

	return types.ListValueMust(types.Int64Type, elements)
}

// flattenObjectList builds a Terraform list of objects from a spec []any,
// delegating per-element conversion to build. An empty or absent list flattens
// to null so an unset block does not drift.
func flattenObjectList(
	raw any,
	objectType map[string]attr.Type,
	build func(map[string]any) map[string]attr.Value,
) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	elementType := types.ObjectType{AttrTypes: objectType}

	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return types.ListNull(elementType), diags
	}

	elements := make([]attr.Value, 0, len(items))

	for _, raw := range items {
		item, _ := raw.(map[string]any)

		object, objectDiags := types.ObjectValue(objectType, build(item))
		diags.Append(objectDiags...)

		elements = append(elements, object)
	}

	value, listDiags := types.ListValue(elementType, elements)
	diags.Append(listDiags...)

	return value, diags
}

// Presence-preserving spec fields.
//
// Most optional spec keys treat "absent" and "empty" alike, so the helpers above
// deliberately collapse the two: a null attribute expands to an empty value and
// an empty value flattens back to null. A few upstream keys instead give the
// empty form its own meaning — the chart substitutes its own value when the key
// is absent, and an explicitly empty value opts out of that substitution. For
// those, collapsing the states silently reinstates the substitution the operator
// opted out of, and flattening a stored empty value back to null makes every
// apply report drift. The helpers below carry the distinction through both
// directions: they write a key only when the attribute is set, and they read a
// key back as null only when it is absent.

// setOptionalString writes a string into spec under key. A null or unknown
// attribute leaves the key out; an explicit empty string is written.
func setOptionalString(spec map[string]any, key string, value types.String) {
	if value.IsNull() || value.IsUnknown() {
		return
	}

	spec[key] = value.ValueString()
}

// specStringOrNull reads a string from spec, flattening an absent key to null
// and a present empty string to an empty string value.
func specStringOrNull(spec map[string]any, key string) types.String {
	text, ok := spec[key].(string)
	if !ok {
		return types.StringNull()
	}

	return types.StringValue(text)
}

// setOptionalStringList writes a string list into spec under key. A null or
// unknown attribute leaves the key out; an empty list writes an empty list.
func setOptionalStringList(
	ctx context.Context,
	spec map[string]any,
	key string,
	value types.List,
) diag.Diagnostics {
	var diags diag.Diagnostics

	if value.IsNull() || value.IsUnknown() {
		return diags
	}

	items, itemDiags := expandStringList(ctx, value)
	diags.Append(itemDiags...)

	if diags.HasError() {
		return diags
	}

	spec[key] = items

	return diags
}

// specStringListOrNull builds a string list from a spec value, flattening an
// absent key to null and a present empty list to an empty list.
func specStringListOrNull(raw any) types.List {
	items, ok := raw.([]any)
	if !ok {
		return types.ListNull(types.StringType)
	}

	elements := make([]attr.Value, 0, len(items))
	for _, item := range items {
		text, _ := item.(string)
		elements = append(elements, types.StringValue(text))
	}

	return types.ListValueMust(types.StringType, elements)
}

// setOptionalObjectList writes a nested-object list into spec under key,
// delegating per-element conversion to build. A null or unknown attribute leaves
// the key out; an empty list writes an empty list.
func setOptionalObjectList[T any](
	ctx context.Context,
	spec map[string]any,
	key string,
	value types.List,
	build func(T) map[string]any,
) diag.Diagnostics {
	var diags diag.Diagnostics

	if value.IsNull() || value.IsUnknown() {
		return diags
	}

	items, itemDiags := expandObjectList(ctx, value, build)
	diags.Append(itemDiags...)

	if diags.HasError() {
		return diags
	}

	spec[key] = items

	return diags
}

// specObjectListOrNull builds a nested-object list from a spec value,
// delegating per-element conversion to build. An absent key flattens to null; a
// present empty list flattens to an empty list.
func specObjectListOrNull(
	raw any,
	objectType map[string]attr.Type,
	build func(map[string]any) map[string]attr.Value,
) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	elementType := types.ObjectType{AttrTypes: objectType}

	items, ok := raw.([]any)
	if !ok {
		return types.ListNull(elementType), diags
	}

	elements := make([]attr.Value, 0, len(items))

	for _, item := range items {
		entry, _ := item.(map[string]any)

		object, objectDiags := types.ObjectValue(objectType, build(entry))
		diags.Append(objectDiags...)

		elements = append(elements, object)
	}

	value, listDiags := types.ListValue(elementType, elements)
	diags.Append(listDiags...)

	return value, diags
}

// Shared tri-state blocks.
//
// The tls and backup blocks below hold a single flag whose absence carries its
// own meaning upstream: an absent tls key inherits `external`, and an absent
// backup key leaves the chart's own backup defaults in place instead of pinning
// a partial block. Both therefore write their spec key only when the flag is
// set, and read an empty block back as null so the schema default the
// aggregated API may echo does not drift.

// tlsObjectType is the {enabled} object of the tls block.
func tlsObjectType() map[string]attr.Type {
	return map[string]attr.Type{attrEnabled: types.BoolType}
}

// setOptionalTLS writes the tls block into spec. An unset block, or one whose
// enabled flag is unset, leaves the key out so the chart keeps inheriting
// `external`.
func setOptionalTLS(spec map[string]any, value types.Object) {
	if enabled, ok := optionalBlockFlag(value, attrEnabled); ok {
		spec[specTLS] = map[string]any{attrEnabled: enabled}
	}
}

// flattenTLS builds the tls block from a spec value. An absent block, and the
// empty one the upstream schema default produces, both flatten to null.
func flattenTLS(raw any) types.Object {
	return flattenBlockFlag(raw, tlsObjectType(), attrEnabled, attrEnabled)
}

// backupObjectType is the {use_system_bucket} object of the backup block.
func backupObjectType() map[string]attr.Type {
	return map[string]attr.Type{attrUseSystemBucket: types.BoolType}
}

// setOptionalBackup writes the backup block into spec. Only the system-bucket
// opt-in is managed, so an unset block leaves the key out entirely rather than
// overwriting the unmanaged backup fields of an existing release.
func setOptionalBackup(spec map[string]any, value types.Object) {
	if useSystemBucket, ok := optionalBlockFlag(value, attrUseSystemBucket); ok {
		spec[specBackup] = map[string]any{specUseSystemBucket: useSystemBucket}
	}
}

// flattenBackup builds the backup block from a spec value, keeping the
// unmanaged sibling fields out of state.
func flattenBackup(raw any) types.Object {
	return flattenBlockFlag(raw, backupObjectType(), attrUseSystemBucket, specUseSystemBucket)
}

// optionalBlockFlag reads the single boolean of a one-flag block, reporting
// whether it is set.
func optionalBlockFlag(value types.Object, name string) (bool, bool) {
	if value.IsNull() || value.IsUnknown() {
		return false, false
	}

	flag, ok := value.Attributes()[name].(types.Bool)
	if !ok || flag.IsNull() || flag.IsUnknown() {
		return false, false
	}

	return flag.ValueBool(), true
}

// flattenBlockFlag builds a one-flag block object from a spec submap, returning
// null unless the flag itself is present.
func flattenBlockFlag(raw any, objectType map[string]attr.Type, name, specKey string) types.Object {
	block, ok := raw.(map[string]any)
	if !ok {
		return types.ObjectNull(objectType)
	}

	flag, ok := block[specKey].(bool)
	if !ok {
		return types.ObjectNull(objectType)
	}

	return types.ObjectValueMust(objectType, map[string]attr.Value{name: types.BoolValue(flag)})
}

// setOptionalJSONList writes a list of JSON documents into spec under key. The
// upstream fields are free-form core/v1 objects, so they travel as normalized
// JSON strings rather than a hand-modelled Volume schema.
func setOptionalJSONList(
	ctx context.Context,
	spec map[string]any,
	key string,
	value types.List,
) diag.Diagnostics {
	var diags diag.Diagnostics

	if value.IsNull() || value.IsUnknown() {
		return diags
	}

	var items []jsontypes.Normalized

	diags.Append(value.ElementsAs(ctx, &items, false)...)

	if diags.HasError() {
		return diags
	}

	out := make([]any, 0, len(items))

	for index, item := range items {
		var document any

		if err := json.Unmarshal([]byte(item.ValueString()), &document); err != nil {
			diags.AddError(
				"Invalid JSON document in "+key,
				fmt.Sprintf("Element %d is not a JSON document: %s", index, err),
			)

			return diags
		}

		out = append(out, document)
	}

	spec[key] = out

	return diags
}

// specJSONListOrNull builds a list of JSON documents from a spec value. An
// absent key flattens to null; a present empty list stays an empty list.
func specJSONListOrNull(raw any) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	items, ok := raw.([]any)
	if !ok {
		return types.ListNull(jsontypes.NormalizedType{}), diags
	}

	elements := make([]attr.Value, 0, len(items))

	for _, item := range items {
		encoded, err := json.Marshal(item)
		if err != nil {
			diags.AddError("Unable to encode a spec document", err.Error())

			return types.ListNull(jsontypes.NormalizedType{}), diags
		}

		elements = append(elements, jsontypes.NewNormalizedValue(string(encoded)))
	}

	value, listDiags := types.ListValue(jsontypes.NormalizedType{}, elements)
	diags.Append(listDiags...)

	return value, diags
}
