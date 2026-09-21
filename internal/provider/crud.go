package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cozystack/terraform-provider-cozystack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// defaultWaitTimeout bounds how long Create/Update block on readiness.
const defaultWaitTimeout = 10 * time.Minute

// outputsPollInterval sets how often Create/Update re-read outputs that have not
// materialised yet.
const outputsPollInterval = 2 * time.Second

// outputsReadAttempts bounds consecutive failing reads while polling. A single
// API hiccup on a converging cluster should not fail an apply whose object is
// already created, but a read that keeps failing is reported rather than sat on
// until the deadline.
const outputsReadAttempts = 3

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
// an error — outputs are asynchronous, so the model leaves the field null and
// reports outputsPending, which is what lets create and update wait for it.
type outputsReader interface {
	readOutputs(ctx context.Context, api *client.Client) diag.Diagnostics
	outputsPending() bool
}

// readModelOutputs invokes the optional outputs hook when the model implements it.
func readModelOutputs(ctx context.Context, model any, api *client.Client, diags *diag.Diagnostics) {
	if reader, ok := model.(outputsReader); ok {
		diags.Append(reader.readOutputs(ctx, api)...)
	}
}

// readOutputsAfterPersist reads a model's outputs once created or updated. When
// the practitioner asked to wait for readiness, outputs that are still missing
// are polled until the shared deadline, which the readiness wait has already
// consumed part of.
//
// Update polls as well: adding a bucket user materialises a new artifact the
// same way creating the object does, and returning outputs that grew is legal
// because a plan that changes the object marks every Computed attribute with a
// null config value unknown, prior state notwithstanding (fwserver's
// MarkComputedNilsAsUnknown).
func readOutputsAfterPersist(
	ctx context.Context,
	model any,
	api *client.Client,
	wait bool,
	deadline time.Time,
	diags *diag.Diagnostics,
) {
	reader, ok := model.(outputsReader)
	if !ok {
		return
	}

	if !wait {
		diags.Append(reader.readOutputs(ctx, api)...)

		return
	}

	diags.Append(pollOutputs(ctx, reader, api, deadline, outputsPollInterval)...)
}

// pollOutputs re-reads outputs until they appear, the deadline passes, or the
// read fails. Outputs that never appear are a warning, not an error: the object
// itself is already created and the next refresh picks them up.
func pollOutputs(
	ctx context.Context,
	reader outputsReader,
	api *client.Client,
	deadline time.Time,
	interval time.Duration,
) diag.Diagnostics {
	failures := 0

	for {
		diags := reader.readOutputs(ctx, api)

		switch {
		case diags.HasError():
			failures++
			if failures >= outputsReadAttempts {
				return diags
			}
		case !reader.outputsPending():
			return diags
		default:
			failures = 0
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			var timedOut diag.Diagnostics

			timedOut.AddWarning(
				"Timed out waiting for generated outputs",
				"The object is ready, but the Secrets, Services, or addresses carrying its outputs "+
					"have not appeared within wait_timeout. The attributes are null for now and are "+
					"populated by the next refresh.",
			)

			// A read that failed within its retry budget still explains the
			// timeout better than the timeout does.
			for _, failed := range diags.Errors() {
				timedOut.AddWarning(failed.Summary(), failed.Detail())
			}

			return timedOut
		}

		select {
		// A cancelled apply reports nothing: the last read's failure, if any,
		// describes the cluster, not the cancellation the practitioner asked for.
		case <-ctx.Done():
			return nil
		case <-time.After(min(interval, remaining)):
		}
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

	wait := waitFor.ValueBool()

	duration, parseDiags := waitDuration(wait, timeout)
	diags.Append(parseDiags...)

	if diags.HasError() {
		return
	}

	deadline := time.Now().Add(duration)

	result, ok := persistApplication(ctx, api, res, create, app, wait, duration, diags)
	if !ok {
		return
	}

	diags.Append(pm.flatten(&result)...)

	// An object that never became ready has no outputs to wait for, and the
	// timeout warning would contradict the readiness error already raised.
	readOutputsAfterPersist(ctx, pm, api, wait && !diags.HasError(), deadline, diags)
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
// blocks until it is Ready. It reports false only when nothing was written; a
// readiness timeout reports true with the last observation and an error
// diagnostic, so the caller still records the object that does exist instead of
// leaving it behind with no state.
func persistApplication(
	ctx context.Context,
	api *client.Client,
	res client.Resource,
	create bool,
	app *client.Application,
	wait bool,
	timeout time.Duration,
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

	if wait {
		ready, waitErr := api.WaitForReady(ctx, res, result.Namespace, result.Name, timeout)
		if ready.Name != "" {
			result = ready
		}

		if waitErr != nil {
			diags.AddError("Timed out waiting for "+res.Kind+" to become ready", waitErr.Error())
		}
	}

	return result, true
}

// waitDuration parses the wait_timeout attribute as a Go duration. It is parsed
// only when the practitioner asked to wait, so an unused malformed value stays
// as harmless as it was before.
func waitDuration(wait bool, timeout types.String) (time.Duration, diag.Diagnostics) {
	if !wait {
		return 0, nil
	}

	return parseWaitTimeout(timeout)
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
