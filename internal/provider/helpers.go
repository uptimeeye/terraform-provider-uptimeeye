package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"gitlab.com/uptimeeye/terraform-provider-uptimeeye/internal/apiclient"
)

// clientFromResourceConfigure extracts the shared API client in a resource's
// Configure implementation. Returns nil (and records an error) on mismatch.
func clientFromResourceConfigure(req resource.ConfigureRequest, resp *resource.ConfigureResponse) *apiclient.ClientWithResponses {
	if req.ProviderData == nil {
		return nil // Configure is called twice; the first call has no data yet.
	}
	client, ok := req.ProviderData.(*apiclient.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected resource configure type",
			fmt.Sprintf("expected *apiclient.ClientWithResponses, got %T", req.ProviderData),
		)
		return nil
	}
	return client
}

// clientFromDataSourceConfigure is the data-source counterpart of
// clientFromResourceConfigure.
func clientFromDataSourceConfigure(req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) *apiclient.ClientWithResponses {
	if req.ProviderData == nil {
		return nil
	}
	client, ok := req.ProviderData.(*apiclient.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected data source configure type",
			fmt.Sprintf("expected *apiclient.ClientWithResponses, got %T", req.ProviderData),
		)
		return nil
	}
	return client
}

// apiCallFailed records a diagnostic for a transport-level error (err != nil).
// When it returns false, the response struct of the generated client is
// guaranteed to be non-nil.
func apiCallFailed(diags *diag.Diagnostics, summary string, err error) bool {
	if err != nil {
		diags.AddError(summary, err.Error())
		return true
	}
	return false
}

// checkStatus verifies the HTTP status is 2xx and records a diagnostic with
// the response body otherwise.
func checkStatus(diags *diag.Diagnostics, summary string, httpResp *http.Response, body []byte) bool {
	if httpResp == nil {
		diags.AddError(summary, "no HTTP response received")
		return false
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		diags.AddError(summary, fmt.Sprintf("unexpected status %d: %s", httpResp.StatusCode, string(body)))
		return false
	}
	return true
}

// isNotFound reports whether the response indicates a deleted/never-existing
// entity, which Terraform handles by removing the resource from state.
func isNotFound(httpResp *http.Response) bool {
	return httpResp != nil && httpResp.StatusCode == http.StatusNotFound
}

// stringSliceToList converts a Go string slice to a types.List, mapping nil to
// an empty list (the API normalizes nil slices to [] as well).
func stringSliceToList(ctx context.Context, diags *diag.Diagnostics, values *[]string) types.List {
	elems := []string{}
	if values != nil {
		elems = *values
	}
	list, d := types.ListValueFrom(ctx, types.StringType, elems)
	diags.Append(d...)
	return list
}

// listToStringSlice converts a types.List of strings back to a Go slice.
// Null/unknown lists become empty slices.
func listToStringSlice(ctx context.Context, diags *diag.Diagnostics, list types.List) []string {
	if list.IsNull() || list.IsUnknown() {
		return []string{}
	}
	var out []string
	diags.Append(list.ElementsAs(ctx, &out, false)...)
	if out == nil {
		out = []string{}
	}
	return out
}

// emptyStringList is the default value for optional tag/recipient lists.
func emptyStringList() types.List {
	list, _ := types.ListValue(types.StringType, []attr.Value{})
	return list
}

// ptr returns a pointer to v; the generated client models optional fields as
// pointers.
func ptr[T any](v T) *T {
	return &v
}

// deref returns the value p points to, or fallback when p is nil.
func deref[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}
	return *p
}
