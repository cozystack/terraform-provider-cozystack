package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

// bucketModel maps the cozystack_bucket schema to Go types. The spec attributes
// mirror the json tags of the pinned bucket.ConfigSpec one-to-one.
type bucketModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Namespace    types.String `tfsdk:"namespace"`
	Locking      types.Bool   `tfsdk:"locking"`
	StoragePool  types.String `tfsdk:"storage_pool"`
	Users        types.Map    `tfsdk:"users"`
	Ready        types.Bool   `tfsdk:"ready"`
	ChartVersion types.String `tfsdk:"chart_version"`
	Credentials  types.Map    `tfsdk:"credentials"`
}

func bucketCredentialsObjectType() map[string]attr.Type {
	return map[string]attr.Type{
		"bucket_name": types.StringType,
		"endpoint":    types.StringType,
		"region":      types.StringType,
		"access_key":  types.StringType,
		"secret_key":  types.StringType,
	}
}

// bucketInfo is the COSI BucketInfo blob stored under the `BucketInfo` key of a
// bucket user's credentials Secret.
type bucketInfo struct {
	Spec struct {
		BucketName string `json:"bucketName"`
		SecretS3   struct {
			Endpoint        string `json:"endpoint"`
			Region          string `json:"region"`
			AccessKeyID     string `json:"accessKeyID"` //nolint:tagliatelle // COSI BucketInfo uses accessKeyID verbatim
			AccessSecretKey string `json:"accessSecretKey"`
		} `json:"secretS3"`
	} `json:"spec"`
}

// readOutputs reads each user's S3 credentials, which the chart materialises as
// the Secret `bucket-<name>-<user>` (key `BucketInfo`, a COSI blob). Secrets
// appear asynchronously, so users without one are simply omitted from credentials.
func (m *bucketModel) readOutputs(ctx context.Context, api *client.Client) diag.Diagnostics {
	var diags diag.Diagnostics

	credentialsType := types.ObjectType{AttrTypes: bucketCredentialsObjectType()}
	m.Credentials = types.MapNull(credentialsType)

	if m.Users.IsNull() || m.Users.IsUnknown() {
		return diags
	}

	namespace, name := m.identity()
	entries := map[string]attr.Value{}

	for user := range m.Users.Elements() {
		data, found, err := api.GetSecretData(ctx, namespace, "bucket-"+name+"-"+user)
		if err != nil {
			diags.AddError("Unable to read bucket credentials", err.Error())

			return diags
		}

		if !found {
			continue
		}

		var info bucketInfo
		if jsonErr := json.Unmarshal(data["BucketInfo"], &info); jsonErr != nil {
			diags.AddError("Unable to parse bucket credentials", jsonErr.Error())

			return diags
		}

		entries[user] = types.ObjectValueMust(bucketCredentialsObjectType(), map[string]attr.Value{
			"bucket_name": types.StringValue(info.Spec.BucketName),
			"endpoint":    types.StringValue(info.Spec.SecretS3.Endpoint),
			"region":      types.StringValue(info.Spec.SecretS3.Region),
			"access_key":  types.StringValue(info.Spec.SecretS3.AccessKeyID),
			"secret_key":  types.StringValue(info.Spec.SecretS3.AccessSecretKey),
		})
	}

	if len(entries) > 0 {
		m.Credentials = types.MapValueMust(credentialsType, entries)
	}

	return diags
}

type bucketUserModel struct {
	Readonly types.Bool `tfsdk:"readonly"`
}

func bucketUserObjectType() map[string]attr.Type {
	return map[string]attr.Type{"readonly": types.BoolType}
}

// identity returns the bucket's namespace and name.
func (m *bucketModel) identity() (string, string) {
	return m.Namespace.ValueString(), m.Name.ValueString()
}

// expand converts the Terraform model into a client.Application ready to send.
func (m *bucketModel) expand(ctx context.Context) (*client.Application, diag.Diagnostics) {
	var diags diag.Diagnostics

	users, uDiags := expandBucketUsers(ctx, m.Users)
	diags.Append(uDiags...)

	if diags.HasError() {
		return nil, diags
	}

	spec := map[string]any{
		"locking":     m.Locking.ValueBool(),
		"storagePool": m.StoragePool.ValueString(),
		"users":       users,
	}

	return &client.Application{
		Name:      m.Name.ValueString(),
		Namespace: m.Namespace.ValueString(),
		Spec:      spec,
	}, diags
}

// flatten populates the Terraform model from the server view of a bucket.
func (m *bucketModel) flatten(app *client.Application) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(app.Namespace + "/" + app.Name)
	m.Name = types.StringValue(app.Name)
	m.Namespace = types.StringValue(app.Namespace)
	m.Locking = types.BoolValue(specBool(app.Spec, "locking"))
	m.StoragePool = types.StringValue(specString(app.Spec, "storagePool"))

	users, uDiags := flattenBucketUsers(app.Spec["users"])
	diags.Append(uDiags...)

	m.Users = users

	m.Ready = types.BoolValue(app.Status.Ready)
	m.ChartVersion = types.StringValue(app.Status.Version)

	return diags
}

// expandBucketUsers renders the users map into a spec submap.
func expandBucketUsers(ctx context.Context, value types.Map) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	out := map[string]any{}

	if value.IsNull() || value.IsUnknown() {
		return out, diags
	}

	elements := map[string]bucketUserModel{}
	diags.Append(value.ElementsAs(ctx, &elements, false)...)

	if diags.HasError() {
		return out, diags
	}

	for name, user := range elements {
		out[name] = map[string]any{"readonly": user.Readonly.ValueBool()}
	}

	return out, diags
}

// flattenBucketUsers builds the users map from a spec submap.
func flattenBucketUsers(raw any) (types.Map, diag.Diagnostics) {
	return flattenObjectMap(raw, bucketUserObjectType(), func(user map[string]any) map[string]attr.Value {
		readonly, _ := user["readonly"].(bool)

		return map[string]attr.Value{"readonly": types.BoolValue(readonly)}
	})
}
