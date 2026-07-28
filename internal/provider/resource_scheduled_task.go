package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"gitlab.com/uptimeeye/terraform-provider-uptimeeye/internal/apiclient"
)

var (
	_ resource.Resource                     = &ScheduledTaskResource{}
	_ resource.ResourceWithConfigure        = &ScheduledTaskResource{}
	_ resource.ResourceWithConfigValidators = &ScheduledTaskResource{}
	_ resource.ResourceWithImportState      = &ScheduledTaskResource{}
)

func NewScheduledTaskResource() resource.Resource {
	return &ScheduledTaskResource{}
}

type ScheduledTaskResource struct {
	client *apiclient.ClientWithResponses
}

type ScheduledTaskResourceModel struct {
	Id                    types.String `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	NotificationChannelId types.String `tfsdk:"notification_channel_id"`
	GracePeriod           types.Int64  `tfsdk:"grace_period"`
	Tags                  types.List   `tfsdk:"tags"`
	Paused                types.Bool   `tfsdk:"paused"`
	CronTask              types.Object `tfsdk:"cron_task"`
	SimpleTask            types.Object `tfsdk:"simple_task"`
}

type cronTaskModel struct {
	CronExpression types.String `tfsdk:"cron_expression"`
	Timezone       types.String `tfsdk:"timezone"`
}

type simpleTaskModel struct {
	IntervalType  types.String `tfsdk:"interval_type"`
	IntervalValue types.Int64  `tfsdk:"interval_value"`
}

var cronTaskAttrTypes = map[string]attr.Type{
	"cron_expression": types.StringType,
	"timezone":        types.StringType,
}

var simpleTaskAttrTypes = map[string]attr.Type{
	"interval_type":  types.StringType,
	"interval_value": types.Int64Type,
}

func (r *ScheduledTaskResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_scheduled_task"
}

func (r *ScheduledTaskResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A scheduled task (heartbeat/cron monitoring) expects periodic pings from your jobs " +
			"and alerts through a notification channel when a ping is missed. Exactly one of `cron_task` or " +
			"`simple_task` must be configured.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Scheduled task identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the scheduled task.",
				Required:            true,
			},
			"notification_channel_id": schema.StringAttribute{
				MarkdownDescription: "ID of the notification channel to alert when the task misses its schedule.",
				Required:            true,
			},
			"grace_period": schema.Int64Attribute{
				MarkdownDescription: "Grace period in minutes before a missed ping triggers an alert.",
				Required:            true,
			},
			"tags": schema.ListAttribute{
				MarkdownDescription: "Tags for organizing scheduled tasks.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(emptyStringList()),
			},
			"paused": schema.BoolAttribute{
				MarkdownDescription: "Whether monitoring of this task is paused.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"cron_task": schema.SingleNestedAttribute{
				MarkdownDescription: "Cron-based schedule. Conflicts with `simple_task`.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"cron_expression": schema.StringAttribute{
						MarkdownDescription: "Cron expression defining when pings are expected.",
						Required:            true,
					},
					"timezone": schema.StringAttribute{
						MarkdownDescription: "IANA timezone the cron expression is evaluated in.",
						Required:            true,
					},
				},
			},
			"simple_task": schema.SingleNestedAttribute{
				MarkdownDescription: "Fixed-interval schedule. Conflicts with `cron_task`.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"interval_type": schema.StringAttribute{
						MarkdownDescription: "Unit of the interval. One of `minute`, `hour`, `day`.",
						Required:            true,
						Validators: []validator.String{
							stringvalidator.OneOf("minute", "hour", "day"),
						},
					},
					"interval_value": schema.Int64Attribute{
						MarkdownDescription: "Expected time between pings, in units of `interval_type`.",
						Required:            true,
					},
				},
			},
		},
	}
}

func (r *ScheduledTaskResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("cron_task"),
			path.MatchRoot("simple_task"),
		),
	}
}

func (r *ScheduledTaskResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

// scheduledTaskBody converts the plan into an API request body, deriving the
// task type from whichever schedule attribute is set.
func scheduledTaskBody(ctx context.Context, diags *diag.Diagnostics, plan ScheduledTaskResourceModel) apiclient.ScheduledTask {
	body := apiclient.ScheduledTask{
		Name:                  plan.Name.ValueString(),
		NotificationChannelId: plan.NotificationChannelId.ValueString(),
		GracePeriod:           plan.GracePeriod.ValueInt64(),
		Tags:                  ptr(listToStringSlice(ctx, diags, plan.Tags)),
	}

	if !plan.CronTask.IsNull() && !plan.CronTask.IsUnknown() {
		var cron cronTaskModel
		diags.Append(plan.CronTask.As(ctx, &cron, basetypes.ObjectAsOptions{})...)
		body.Type = apiclient.Cron
		body.CronTask = &apiclient.CronTask{
			CronExpression: cron.CronExpression.ValueString(),
			Timezone:       cron.Timezone.ValueString(),
		}
	} else if !plan.SimpleTask.IsNull() && !plan.SimpleTask.IsUnknown() {
		var simple simpleTaskModel
		diags.Append(plan.SimpleTask.As(ctx, &simple, basetypes.ObjectAsOptions{})...)
		body.Type = apiclient.Simple
		body.SimpleTask = &apiclient.SimpleTask{
			IntervalType:  apiclient.SimpleTaskIntervalType(simple.IntervalType.ValueString()),
			IntervalValue: simple.IntervalValue.ValueInt64(),
		}
	} else {
		diags.AddError(
			"Invalid scheduled task configuration",
			"exactly one of cron_task or simple_task must be set",
		)
	}
	return body
}

// setPaused calls the pause or resume endpoint to bring the task into the
// desired state.
func (r *ScheduledTaskResource) setPaused(ctx context.Context, diags *diag.Diagnostics, taskId string, paused bool) {
	if paused {
		res, err := r.client.PutV1ScheduledTasksScheduledTaskIdPauseWithResponse(ctx, taskId)
		if apiCallFailed(diags, "Could not pause scheduled task", err) {
			return
		}
		checkStatus(diags, "Could not pause scheduled task", res.HTTPResponse, res.Body)
		return
	}
	res, err := r.client.PutV1ScheduledTasksScheduledTaskIdResumeWithResponse(ctx, taskId)
	if apiCallFailed(diags, "Could not resume scheduled task", err) {
		return
	}
	checkStatus(diags, "Could not resume scheduled task", res.HTTPResponse, res.Body)
}

func (r *ScheduledTaskResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ScheduledTaskResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := scheduledTaskBody(ctx, &resp.Diagnostics, plan)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.PostV1ScheduledTasksWithResponse(ctx, body)
	if apiCallFailed(&resp.Diagnostics, "Could not create scheduled task", err) {
		return
	}
	if !checkStatus(&resp.Diagnostics, "Could not create scheduled task", res.HTTPResponse, res.Body) {
		return
	}
	if res.JSON200 == nil || res.JSON200.Id == nil {
		resp.Diagnostics.AddError("Could not create scheduled task", "API response contained no scheduled task id")
		return
	}
	plan.Id = types.StringValue(*res.JSON200.Id)

	// New tasks start unpaused; only an explicit paused = true needs a call.
	if plan.Paused.ValueBool() {
		r.setPaused(ctx, &resp.Diagnostics, plan.Id.ValueString(), true)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ScheduledTaskResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ScheduledTaskResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.GetV1ScheduledTasksScheduledTaskIdWithResponse(ctx, state.Id.ValueString())
	if apiCallFailed(&resp.Diagnostics, "Could not read scheduled task", err) {
		return
	}
	if isNotFound(res.HTTPResponse) {
		resp.State.RemoveResource(ctx)
		return
	}
	if !checkStatus(&resp.Diagnostics, "Could not read scheduled task", res.HTTPResponse, res.Body) {
		return
	}
	task := res.JSON200
	if task == nil {
		resp.Diagnostics.AddError("Could not read scheduled task", "empty response body")
		return
	}

	state.Name = types.StringValue(task.Name)
	state.NotificationChannelId = types.StringValue(task.NotificationChannelId)
	state.GracePeriod = types.Int64Value(task.GracePeriod)
	state.Tags = stringSliceToList(ctx, &resp.Diagnostics, task.Tags)
	state.Paused = types.BoolValue(task.Status != nil && *task.Status == apiclient.ScheduledTaskStatusPAUSED)

	switch {
	case task.CronTask != nil:
		obj, d := types.ObjectValueFrom(ctx, cronTaskAttrTypes, cronTaskModel{
			CronExpression: types.StringValue(task.CronTask.CronExpression),
			Timezone:       types.StringValue(task.CronTask.Timezone),
		})
		resp.Diagnostics.Append(d...)
		state.CronTask = obj
		state.SimpleTask = types.ObjectNull(simpleTaskAttrTypes)
	case task.SimpleTask != nil:
		obj, d := types.ObjectValueFrom(ctx, simpleTaskAttrTypes, simpleTaskModel{
			IntervalType:  types.StringValue(string(task.SimpleTask.IntervalType)),
			IntervalValue: types.Int64Value(task.SimpleTask.IntervalValue),
		})
		resp.Diagnostics.Append(d...)
		state.SimpleTask = obj
		state.CronTask = types.ObjectNull(cronTaskAttrTypes)
	}
	// Defensive: if the API returned neither schedule (unexpected), keep the
	// state values as they are instead of clearing both.

	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ScheduledTaskResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state ScheduledTaskResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := scheduledTaskBody(ctx, &resp.Diagnostics, plan)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.PutV1ScheduledTasksScheduledTaskIdWithResponse(ctx, plan.Id.ValueString(), body)
	if apiCallFailed(&resp.Diagnostics, "Could not update scheduled task", err) {
		return
	}
	if !checkStatus(&resp.Diagnostics, "Could not update scheduled task", res.HTTPResponse, res.Body) {
		return
	}

	if plan.Paused.ValueBool() != state.Paused.ValueBool() {
		r.setPaused(ctx, &resp.Diagnostics, plan.Id.ValueString(), plan.Paused.ValueBool())
		if resp.Diagnostics.HasError() {
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ScheduledTaskResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ScheduledTaskResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.DeleteV1ScheduledTasksScheduledTaskIdWithResponse(ctx, state.Id.ValueString())
	if apiCallFailed(&resp.Diagnostics, "Could not delete scheduled task", err) {
		return
	}
	if isNotFound(res.HTTPResponse) {
		return // already gone
	}
	checkStatus(&resp.Diagnostics, "Could not delete scheduled task", res.HTTPResponse, res.Body)
}

func (r *ScheduledTaskResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
