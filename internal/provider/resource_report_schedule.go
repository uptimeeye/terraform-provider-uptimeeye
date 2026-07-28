package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"gitlab.com/uptimeeye/terraform-provider-uptimeeye/internal/apiclient"
)

var (
	_ resource.Resource                = &ReportScheduleResource{}
	_ resource.ResourceWithConfigure   = &ReportScheduleResource{}
	_ resource.ResourceWithImportState = &ReportScheduleResource{}
)

func NewReportScheduleResource() resource.Resource {
	return &ReportScheduleResource{}
}

type ReportScheduleResource struct {
	client *apiclient.ClientWithResponses
}

type ReportScheduleResourceModel struct {
	Id           types.String `tfsdk:"id"`
	StatusPageId types.String `tfsdk:"status_page_id"`
	Cadence      types.String `tfsdk:"cadence"`
	Recipients   types.List   `tfsdk:"recipients"`
	Enabled      types.Bool   `tfsdk:"enabled"`
}

func (r *ReportScheduleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_report_schedule"
}

func (r *ReportScheduleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The uptime report schedule of a status page. Each status page has at most one " +
			"schedule (singleton, upserted via PUT). The API has no delete operation: destroying this resource " +
			"disables the report schedule (`enabled = false`) rather than deleting it. Import with the status page id.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Schedule identifier; equal to `status_page_id` (one schedule per page).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"status_page_id": schema.StringAttribute{
				MarkdownDescription: "Id of the status page the reports are generated for. Changing this forces a new schedule.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"cadence": schema.StringAttribute{
				MarkdownDescription: "How often reports are sent. One of `weekly`, `monthly`.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("weekly", "monthly"),
				},
			},
			"recipients": schema.ListAttribute{
				MarkdownDescription: "Email addresses the report is sent to. At least one recipient is required.",
				ElementType:         types.StringType,
				Required:            true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether reports are actually sent.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
		},
	}
}

func (r *ReportScheduleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

// upsertReportSchedule PUTs the schedule of the plan's status page — the API
// call is identical for create and update. Returns false when a diagnostic was
// recorded.
func (r *ReportScheduleResource) upsertReportSchedule(ctx context.Context, diags *diag.Diagnostics, plan *ReportScheduleResourceModel, summary string) bool {
	body := apiclient.PutV1StatusPagesStatusPageIdReportScheduleJSONRequestBody{
		Cadence:    plan.Cadence.ValueString(),
		Enabled:    plan.Enabled.ValueBool(),
		Recipients: ptr(listToStringSlice(ctx, diags, plan.Recipients)),
	}
	if diags.HasError() {
		return false
	}

	res, err := r.client.PutV1StatusPagesStatusPageIdReportScheduleWithResponse(ctx, plan.StatusPageId.ValueString(), body)
	if apiCallFailed(diags, summary, err) {
		return false
	}
	return checkStatus(diags, summary, res.HTTPResponse, res.Body)
}

func (r *ReportScheduleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ReportScheduleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.upsertReportSchedule(ctx, &resp.Diagnostics, &plan, "Could not create report schedule") {
		return
	}

	plan.Id = types.StringValue(plan.StatusPageId.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ReportScheduleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ReportScheduleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.GetV1StatusPagesStatusPageIdReportScheduleWithResponse(ctx, state.StatusPageId.ValueString())
	if apiCallFailed(&resp.Diagnostics, "Could not read report schedule", err) {
		return
	}
	if isNotFound(res.HTTPResponse) {
		resp.State.RemoveResource(ctx)
		return
	}
	if !checkStatus(&resp.Diagnostics, "Could not read report schedule", res.HTTPResponse, res.Body) {
		return
	}
	if res.JSON200 == nil {
		resp.Diagnostics.AddError("Could not read report schedule", "empty response body")
		return
	}
	schedule := res.JSON200.Schedule

	state.Id = types.StringValue(state.StatusPageId.ValueString())
	state.Cadence = types.StringValue(schedule.Cadence)
	state.Recipients = stringSliceToList(ctx, &resp.Diagnostics, schedule.Recipients)
	state.Enabled = types.BoolValue(schedule.Enabled)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ReportScheduleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ReportScheduleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.upsertReportSchedule(ctx, &resp.Diagnostics, &plan, "Could not update report schedule") {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ReportScheduleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ReportScheduleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// There is no DELETE endpoint — destroying disables reporting while keeping
	// the last cadence/recipients on the server.
	body := apiclient.PutV1StatusPagesStatusPageIdReportScheduleJSONRequestBody{
		Cadence:    state.Cadence.ValueString(),
		Enabled:    false,
		Recipients: ptr(listToStringSlice(ctx, &resp.Diagnostics, state.Recipients)),
	}
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.PutV1StatusPagesStatusPageIdReportScheduleWithResponse(ctx, state.StatusPageId.ValueString(), body)
	if apiCallFailed(&resp.Diagnostics, "Could not disable report schedule", err) {
		return
	}
	if isNotFound(res.HTTPResponse) {
		return // status page already gone
	}
	checkStatus(&resp.Diagnostics, "Could not disable report schedule", res.HTTPResponse, res.Body)
}

func (r *ReportScheduleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// The import ID is the status page id; the schedule id equals it.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("status_page_id"), req.ID)...)
}
