package provider

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// assertSpecCoverage compares emitted paths with the pinned API's json tags.
// Structs and pointers to structs are checked recursively. Maps and slices are
// leaves: their dynamic keys and elements need separate, explicit guards.
// An omission covers exactly that dotted path and its descendants, not siblings.
func assertSpecCoverage(t *testing.T, emitted map[string]any, spec any, omit ...string) {
	t.Helper()

	for _, err := range specCoverageErrors(emitted, spec, omit...) {
		t.Error(err)
	}
}

// specCoverageErrors sorts diagnostics so negative coverage assertions remain
// deterministic despite map iteration order.
func specCoverageErrors(emitted map[string]any, spec any, omit ...string) []string {
	tags := configSpecJSONTags(spec)
	keys := emittedSpecJSONTags(emitted, reflect.TypeOf(spec))
	var errors []string

	for key := range keys {
		if !tags[key] {
			errors = append(errors, fmt.Sprintf("expand emits %q, which is not a ConfigSpec json tag", key))
		}
	}

	for tag := range tags {
		omitted := slices.ContainsFunc(omit, func(path string) bool {
			return tag == path || strings.HasPrefix(tag, path+".")
		})
		if !keys[tag] && !omitted {
			errors = append(errors, fmt.Sprintf("ConfigSpec has %q but expand does not emit it", tag))
		}
	}

	slices.Sort(errors)
	return errors
}

// configSpecJSONTags derives dotted paths from types so nil optional blocks
// retain coverage of their nested fields.
func configSpecJSONTags(spec any) map[string]bool {
	tags := map[string]bool{}

	typ := reflect.TypeOf(spec)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return tags
	}

	for i := range typ.NumField() {
		field := typ.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if !field.IsExported() || name == "" || name == "-" {
			continue
		}

		tags[name] = true
		for nested := range configSpecJSONTags(reflect.Zero(field.Type).Interface()) {
			tags[name+"."+nested] = true
		}
	}

	return tags
}

// Follow only struct fields from the API, rather than arbitrary emitted maps.
// For example, users is a map with user-chosen keys, not a nested struct block.
func emittedSpecJSONTags(emitted map[string]any, typ reflect.Type) map[string]bool {
	keys := make(map[string]bool, len(emitted))
	for key := range emitted {
		keys[key] = true
	}

	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return keys
	}

	for i := range typ.NumField() {
		field := typ.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if !field.IsExported() || name == "" || name == "-" {
			continue
		}

		fieldType := field.Type
		if fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		if fieldType.Kind() != reflect.Struct {
			continue
		}

		block, _ := emitted[name].(map[string]any)
		for nested := range emittedSpecJSONTags(block, fieldType) {
			keys[name+"."+nested] = true
		}
	}

	return keys
}

// TestConfigSpecJSONTags requires value and nil-pointer specs to yield the same
// tagged paths. Ignored fields are excluded, and map/slice elements stay outside
// recursive traversal.
func TestConfigSpecJSONTags(t *testing.T) {
	t.Parallel()

	type block struct {
		Name string `json:"name,omitempty"`
	}
	type spec struct {
		Value    block            `json:"value"`
		Pointer  *block           `json:"pointer,omitempty"`
		Map      map[string]block `json:"map"`
		Slice    []block          `json:"slice"`
		Ignored  block            `json:"-"`
		Untagged block
	}

	want := map[string]bool{
		"value": true, "value.name": true,
		"pointer": true, "pointer.name": true,
		"map": true, "slice": true,
	}

	for _, input := range []any{spec{}, (*spec)(nil)} {
		if got := configSpecJSONTags(input); !reflect.DeepEqual(got, want) {
			t.Errorf("configSpecJSONTags(%T) = %v, want %v", input, got, want)
		}
	}
}

// TestSpecCoverageNestedPaths limits omissions to missing paths and their
// descendants. Similarly named siblings and unexpected emitted keys must still
// produce diagnostics, even inside omitted blocks.
func TestSpecCoverageNestedPaths(t *testing.T) {
	t.Parallel()

	type bootstrap struct {
		ServerName    string `json:"serverName"`
		NewServerName string `json:"newServerName,omitempty"`
	}
	type spec struct {
		Bootstrap      *bootstrap `json:"bootstrap,omitempty"`
		BootstrapExtra bootstrap  `json:"bootstrapExtra"`
	}

	tests := []struct {
		name    string
		emitted map[string]any
		omit    []string
		want    []string
	}{
		{
			name: "new nested field is detected",
			emitted: map[string]any{
				"bootstrap": map[string]any{"serverName": "old"},
			},
			omit: []string{"bootstrapExtra"},
			want: []string{`ConfigSpec has "bootstrap.newServerName" but expand does not emit it`},
		},
		{
			name: "dotted omission leaves siblings checked",
			emitted: map[string]any{
				"bootstrap": map[string]any{},
			},
			omit: []string{"bootstrap.newServerName", "bootstrapExtra"},
			want: []string{`ConfigSpec has "bootstrap.serverName" but expand does not emit it`},
		},
		{
			name: "dotted omission allows the known gap",
			emitted: map[string]any{
				"bootstrap": map[string]any{"serverName": "old"},
			},
			omit: []string{"bootstrap.newServerName", "bootstrapExtra"},
		},
		{
			name: "whole block omission stops at a dot boundary",
			emitted: map[string]any{
				"bootstrapExtra": map[string]any{"serverName": "other"},
			},
			omit: []string{"bootstrap"},
			want: []string{`ConfigSpec has "bootstrapExtra.newServerName" but expand does not emit it`},
		},
		{
			name: "unexpected nested field is detected even in an omitted block",
			emitted: map[string]any{
				"bootstrap": map[string]any{"removed": true},
			},
			omit: []string{"bootstrap", "bootstrapExtra"},
			want: []string{`expand emits "bootstrap.removed", which is not a ConfigSpec json tag`},
		},
		{
			name: "nil block does not cover its fields",
			emitted: map[string]any{
				"bootstrap": nil,
			},
			omit: []string{"bootstrapExtra"},
			want: []string{
				`ConfigSpec has "bootstrap.newServerName" but expand does not emit it`,
				`ConfigSpec has "bootstrap.serverName" but expand does not emit it`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := specCoverageErrors(tt.emitted, spec{}, tt.omit...)
			if !slices.Equal(got, tt.want) {
				t.Errorf("coverage errors = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpecCoverageMapAndSliceBoundaries(t *testing.T) {
	t.Parallel()

	type entry struct {
		Name string `json:"name"`
	}
	type spec struct {
		Users map[string]entry `json:"users"`
		Items []entry          `json:"items"`
	}

	emitted := map[string]any{
		"users": map[string]any{"arbitrary-user": map[string]any{"custom": true}},
		"items": []any{map[string]any{"custom": true}},
	}
	assertSpecCoverage(t, emitted, spec{})
}
