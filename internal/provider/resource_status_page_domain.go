package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"gitlab.com/uptimeeye/terraform-provider-uptimeeye/internal/apiclient"
)

var (
	_ resource.Resource                = &StatusPageDomainResource{}
	_ resource.ResourceWithConfigure   = &StatusPageDomainResource{}
	_ resource.ResourceWithImportState = &StatusPageDomainResource{}
)

func NewStatusPageDomainResource() resource.Resource {
	return &StatusPageDomainResource{}
}

type StatusPageDomainResource struct {
	client *apiclient.ClientWithResponses
}

type StatusPageDomainResourceModel struct {
	Id           types.String `tfsdk:"id"`
	StatusPageId types.String `tfsdk:"status_page_id"`
	Hostname     types.String `tfsdk:"hostname"`
	Status       types.String `tfsdk:"status"`
	LastError    types.String `tfsdk:"last_error"`
}

func (r *StatusPageDomainResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_status_page_domain"
}

func (r *StatusPageDomainResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A custom domain attached to a status page. The API has no update operation for " +
			"domains, so every configuration change replaces the domain. `status` and `last_error` reflect the " +
			"server-side DNS/TLS provisioning state and change over time. Import with `<status_page_id>/<domain_id>`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Domain identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"status_page_id": schema.StringAttribute{
				MarkdownDescription: "Id of the status page the domain belongs to. Changing this forces a new domain.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"hostname": schema.StringAttribute{
				MarkdownDescription: "Fully qualified hostname (e.g. `status.example.com`). Changing this forces a new domain.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			// status and last_error deliberately have no UseStateForUnknown:
			// they legitimately change server-side, so each refresh re-reads them.
			"status": schema.StringAttribute{
				MarkdownDescription: "DNS/provisioning state of the domain (server-managed, changes over time).",
				Computed:            true,
			},
			"last_error": schema.StringAttribute{
				MarkdownDescription: "Last provisioning error, if any (server-managed).",
				Computed:            true,
			},
		},
	}
}

func (r *StatusPageDomainResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *StatusPageDomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan StatusPageDomainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.PostV1StatusPagesStatusPageIdDomainsWithResponse(ctx, plan.StatusPageId.ValueString(),
		apiclient.PostV1StatusPagesStatusPageIdDomainsJSONRequestBody{
			Hostname: plan.Hostname.ValueString(),
		})
	if apiCallFailed(&resp.Diagnostics, "Could not create status page domain", err) {
		return
	}
	if !checkStatus(&resp.Diagnostics, "Could not create status page domain", res.HTTPResponse, res.Body) {
		return
	}
	if res.JSON200 == nil {
		resp.Diagnostics.AddError("Could not create status page domain", "empty response body")
		return
	}
	domain := res.JSON200.Domain

	plan.Id = types.StringValue(domain.Id)
	plan.Hostname = types.StringValue(domain.Hostname)
	plan.Status = types.StringValue(domain.Status)
	plan.LastError = types.StringPointerValue(domain.LastError)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *StatusPageDomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state StatusPageDomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// There is no single-domain GET — list the page's domains and pick ours.
	res, err := r.client.GetV1StatusPagesStatusPageIdDomainsWithResponse(ctx, state.StatusPageId.ValueString())
	if apiCallFailed(&resp.Diagnostics, "Could not read status page domain", err) {
		return
	}
	if isNotFound(res.HTTPResponse) {
		resp.State.RemoveResource(ctx) // the whole status page is gone
		return
	}
	if !checkStatus(&resp.Diagnostics, "Could not read status page domain", res.HTTPResponse, res.Body) {
		return
	}
	if res.JSON200 == nil {
		resp.Diagnostics.AddError("Could not read status page domain", "empty response body")
		return
	}

	var domain *apiclient.StatusPageDomain
	if res.JSON200.Domains != nil {
		for i := range *res.JSON200.Domains {
			if (*res.JSON200.Domains)[i].Id == state.Id.ValueString() {
				domain = &(*res.JSON200.Domains)[i]
				break
			}
		}
	}
	if domain == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.Hostname = types.StringValue(domain.Hostname)
	state.Status = types.StringValue(domain.Status)
	state.LastError = types.StringPointerValue(domain.LastError)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *StatusPageDomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Unreachable: every configurable attribute forces replacement, so
	// Terraform never plans an in-place update for this resource.
	var plan StatusPageDomainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *StatusPageDomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state StatusPageDomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.DeleteV1StatusPagesStatusPageIdDomainsDomainIdWithResponse(ctx, state.StatusPageId.ValueString(), state.Id.ValueString())
	if apiCallFailed(&resp.Diagnostics, "Could not delete status page domain", err) {
		return
	}
	if isNotFound(res.HTTPResponse) {
		return // already gone
	}
	checkStatus(&resp.Diagnostics, "Could not delete status page domain", res.HTTPResponse, res.Body)
}

func (r *StatusPageDomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	statusPageId, domainId, ok := strings.Cut(req.ID, "/")
	if !ok || statusPageId == "" || domainId == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("expected \"<status_page_id>/<domain_id>\", got %q", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("status_page_id"), statusPageId)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), domainId)...)
}
