package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lexfrei/terraform-provider-cozystack/internal/client"
)

// defaultWaitTimeout bounds how long Create/Update block on readiness.
const defaultWaitTimeout = 10 * time.Minute

// readModel is the behaviour a data source model provides to the shared read
// helper: it can be flattened from an Application and report its identity.
type readModel interface {
	flatten(app *client.Application) diag.Diagnostics
	identity() (namespace, name string)
}

// planModel additionally supports being expanded to an Application and exposing
// its create/update wait settings. Resource models implement it.
type planModel interface {
	readModel

	expand(ctx context.Context) (*client.Application, diag.Diagnostics)
	waitConfig() (waitForReady types.Bool, timeout types.String)
}

// outputsReader is an optional behaviour: a model that has server-generated
// outputs (connection details, credentials, addresses) materialised outside the
// aggregated API reads them from the related Secrets/Services/status here. It is
// invoked after flatten, once identity is populated. A missing artifact is not
// an error — outputs are asynchronous, so the model leaves the field null.
type outputsReader interface {
	readOutputs(ctx context.Context, api *client.Client) diag.Diagnostics
}

// readModelOutputs invokes the optional outputs hook when the model implements it.
func readModelOutputs(ctx context.Context, model any, api *client.Client, diags *diag.Diagnostics) {
	if reader, ok := model.(outputsReader); ok {
		diags.Append(reader.readOutputs(ctx, api)...)
	}
}

// readModelPtr / planModelPtr bind a concrete struct M to its pointer methods so
// the generic helpers can allocate a model and operate on it.
type readModelPtr[M any] interface {
	*M
	readModel
}

type planModelPtr[M any] interface {
	*M
	planModel
}

// createOrUpdate is the shared Create/Update implementation for every kind.
func createOrUpdate[M any, PM planModelPtr[M]](
	ctx context.Context,
	api *client.Client,
	res client.Resource,
	create bool,
	plan tfsdk.Plan,
	state *tfsdk.State,
	diags *diag.Diagnostics,
) {
	var model M

	pm := PM(&model)

	diags.Append(plan.Get(ctx, &model)...)

	if diags.HasError() {
		return
	}

	app, expandDiags := pm.expand(ctx)
	diags.Append(expandDiags...)

	if diags.HasError() {
		return
	}

	waitFor, timeout := pm.waitConfig()

	result, ok := persistApplication(ctx, api, res, create, app, waitFor, timeout, diags)
	if !ok {
		return
	}

	diags.Append(pm.flatten(&result)...)
	readModelOutputs(ctx, pm, api, diags)
	diags.Append(state.Set(ctx, &model)...)
}

// readResource is the shared resource Read implementation. A missing object is
// removed from state.
func readResource[M any, PM planModelPtr[M]](
	ctx context.Context,
	api *client.Client,
	res client.Resource,
	state *tfsdk.State,
	diags *diag.Diagnostics,
) {
	var model M

	pm := PM(&model)

	diags.Append(state.Get(ctx, &model)...)

	if diags.HasError() {
		return
	}

	namespace, name := pm.identity()

	app, err := api.Get(ctx, res, namespace, name)
	if err != nil {
		if client.IsNotFound(err) {
			state.RemoveResource(ctx)

			return
		}

		diags.AddError("Unable to read "+res.Kind, err.Error())

		return
	}

	diags.Append(pm.flatten(&app)...)
	readModelOutputs(ctx, pm, api, diags)
	diags.Append(state.Set(ctx, &model)...)
}

// deleteResource is the shared resource Delete implementation.
func deleteResource[M any, PM planModelPtr[M]](
	ctx context.Context,
	api *client.Client,
	res client.Resource,
	state *tfsdk.State,
	diags *diag.Diagnostics,
) {
	var model M

	pm := PM(&model)

	diags.Append(state.Get(ctx, &model)...)

	if diags.HasError() {
		return
	}

	namespace, name := pm.identity()

	err := api.Delete(ctx, res, namespace, name)
	if err != nil {
		diags.AddError("Unable to delete "+res.Kind, err.Error())
	}
}

// readDataSource is the shared data source Read implementation.
func readDataSource[M any, PM readModelPtr[M]](
	ctx context.Context,
	api *client.Client,
	res client.Resource,
	config tfsdk.Config,
	state *tfsdk.State,
	diags *diag.Diagnostics,
) {
	var model M

	pm := PM(&model)

	diags.Append(config.Get(ctx, &model)...)

	if diags.HasError() {
		return
	}

	namespace, name := pm.identity()

	app, err := api.Get(ctx, res, namespace, name)
	if err != nil {
		diags.AddError("Unable to read "+res.Kind, err.Error())

		return
	}

	diags.Append(pm.flatten(&app)...)
	readModelOutputs(ctx, pm, api, diags)
	diags.Append(state.Set(ctx, &model)...)
}

// persistApplication creates or updates an application and, when requested,
// blocks until it is Ready. It reports false (with diagnostics appended) on
// failure so callers can return early.
func persistApplication(
	ctx context.Context,
	api *client.Client,
	res client.Resource,
	create bool,
	app *client.Application,
	waitFor types.Bool,
	timeout types.String,
	diags *diag.Diagnostics,
) (client.Application, bool) {
	action := "update"
	persist := api.Update

	if create {
		action = "create"
		persist = api.Create
	}

	result, err := persist(ctx, res, app)
	if err != nil {
		diags.AddError("Unable to "+action+" "+res.Kind, err.Error())

		return client.Application{}, false
	}

	if waitFor.ValueBool() {
		ready, waitDiags := waitForApplicationReady(ctx, api, res, &result, timeout)
		diags.Append(waitDiags...)

		if diags.HasError() {
			return client.Application{}, false
		}

		result = ready
	}

	return result, true
}

// waitForApplicationReady blocks until the application reports a Ready condition
// or the configured timeout elapses.
func waitForApplicationReady(
	ctx context.Context,
	api *client.Client,
	res client.Resource,
	app *client.Application,
	timeout types.String,
) (client.Application, diag.Diagnostics) {
	var diags diag.Diagnostics

	duration, parseDiags := parseWaitTimeout(timeout)
	diags.Append(parseDiags...)

	if diags.HasError() {
		return client.Application{}, diags
	}

	ready, err := api.WaitForReady(ctx, res, app.Namespace, app.Name, duration)
	if err != nil {
		diags.AddError("Timed out waiting for "+res.Kind+" to become ready", err.Error())

		return client.Application{}, diags
	}

	return ready, diags
}

// parseWaitTimeout parses the wait_timeout attribute as a Go duration.
func parseWaitTimeout(value types.String) (time.Duration, diag.Diagnostics) {
	var diags diag.Diagnostics

	raw := value.ValueString()
	if raw == "" {
		return defaultWaitTimeout, diags
	}

	duration, err := time.ParseDuration(raw)
	if err != nil {
		diags.AddError("Invalid wait_timeout", fmt.Sprintf("%q is not a valid Go duration: %s", raw, err))

		return 0, diags
	}

	return duration, diags
}

// parseImportID splits a "namespace/name" import identifier, reporting false
// when it is malformed.
func parseImportID(id string) (string, string, bool) {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}

	return parts[0], parts[1], true
}
