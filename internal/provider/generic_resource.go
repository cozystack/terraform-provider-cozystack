package provider

import (
	"context"

	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// appResource is the generic managed-resource implementation shared by every
// Cozystack kind. A kind supplies its API descriptor, type-name suffix, schema,
// and model type; all CRUD/import behaviour is handled by the shared helpers.
type appResource[M any, PM planModelPtr[M]] struct {
	res           client.Resource
	typeSuffix    string
	schemaFunc    func() rschema.Schema
	clusterScoped bool
	client        *client.Client
}

// newAppResource builds a resource factory for a namespaced kind.
func newAppResource[M any, PM planModelPtr[M]](
	res client.Resource,
	typeSuffix string,
	schemaFunc func() rschema.Schema,
) func() resource.Resource {
	return func() resource.Resource {
		return &appResource[M, PM]{res: res, typeSuffix: typeSuffix, schemaFunc: schemaFunc}
	}
}

// newClusterResource builds a resource factory for a cluster-scoped kind. It
// behaves like newAppResource but imports by name (no namespace).
func newClusterResource[M any, PM planModelPtr[M]](
	res client.Resource,
	typeSuffix string,
	schemaFunc func() rschema.Schema,
) func() resource.Resource {
	return func() resource.Resource {
		return &appResource[M, PM]{res: res, typeSuffix: typeSuffix, schemaFunc: schemaFunc, clusterScoped: true}
	}
}

func (r *appResource[M, PM]) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_" + r.typeSuffix
}

func (r *appResource[M, PM]) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = r.schemaFunc()
}

func (r *appResource[M, PM]) Configure(
	_ context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	r.client = providerClient(req.ProviderData, &resp.Diagnostics)
}

func (r *appResource[M, PM]) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	createOrUpdateWithConfig[M, PM](ctx, r.client, r.res, true, &req.Config, req.Plan, &resp.State, &resp.Diagnostics)
}

func (r *appResource[M, PM]) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	createOrUpdateWithConfig[M, PM](ctx, r.client, r.res, false, &req.Config, req.Plan, &resp.State, &resp.Diagnostics)
}

func (r *appResource[M, PM]) Read(
	ctx context.Context,
	_ resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	readResource[M, PM](ctx, r.client, r.res, &resp.State, &resp.Diagnostics)
}

func (r *appResource[M, PM]) Delete(
	ctx context.Context,
	_ resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	deleteResource[M, PM](ctx, r.client, r.res, &resp.State, &resp.Diagnostics)
}

func (r *appResource[M, PM]) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	if r.clusterScoped {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(attrName), req.ID)...)

		return
	}

	namespace, name, ok := parseImportID(req.ID)
	if !ok {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			`expected "namespace/name", for example "tenant-root/example"`,
		)

		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(attrNamespace), namespace)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(attrName), name)...)
}

// appDataSource is the generic data-source implementation shared by every kind.
type appDataSource[M any, PM readModelPtr[M]] struct {
	res        client.Resource
	typeSuffix string
	schemaFunc func() dsschema.Schema
	client     *client.Client
}

// newAppDataSource builds a data-source factory for a kind.
func newAppDataSource[M any, PM readModelPtr[M]](
	res client.Resource,
	typeSuffix string,
	schemaFunc func() dsschema.Schema,
) func() datasource.DataSource {
	return func() datasource.DataSource {
		return &appDataSource[M, PM]{res: res, typeSuffix: typeSuffix, schemaFunc: schemaFunc}
	}
}

func (d *appDataSource[M, PM]) Metadata(
	_ context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_" + d.typeSuffix
}

func (d *appDataSource[M, PM]) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = d.schemaFunc()
}

func (d *appDataSource[M, PM]) Configure(
	_ context.Context,
	req datasource.ConfigureRequest,
	resp *datasource.ConfigureResponse,
) {
	d.client = providerClient(req.ProviderData, &resp.Diagnostics)
}

func (d *appDataSource[M, PM]) Read(
	ctx context.Context,
	req datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	readDataSource[M, PM](ctx, d.client, d.res, req.Config, &resp.State, &resp.Diagnostics)
}
