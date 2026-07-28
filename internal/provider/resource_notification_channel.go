package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"gitlab.com/uptimeeye/terraform-provider-uptimeeye/internal/apiclient"
)

var (
	_ resource.Resource                = &NotificationChannelResource{}
	_ resource.ResourceWithConfigure   = &NotificationChannelResource{}
	_ resource.ResourceWithImportState = &NotificationChannelResource{}
)

func NewNotificationChannelResource() resource.Resource {
	return &NotificationChannelResource{}
}

type NotificationChannelResource struct {
	client *apiclient.ClientWithResponses
}

type NotificationChannelResourceModel struct {
	Id   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	Tags types.List   `tfsdk:"tags"`
}

func (r *NotificationChannelResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notification_channel"
}

func (r *NotificationChannelResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A notification channel groups one or more notification integrations (Slack, email, webhook, ...) and is referenced by monitors and scheduled tasks.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Channel identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the channel.",
				Required:            true,
			},
			"tags": schema.ListAttribute{
				MarkdownDescription: "Tags for organizing channels.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(emptyStringList()),
			},
		},
	}
}

func (r *NotificationChannelResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *NotificationChannelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan NotificationChannelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := apiclient.PostV1NotificationChannelsJSONRequestBody{
		Name: plan.Name.ValueString(),
		Tags: ptr(listToStringSlice(ctx, &resp.Diagnostics, plan.Tags)),
	}
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.PostV1NotificationChannelsWithResponse(ctx, body)
	if apiCallFailed(&resp.Diagnostics, "Could not create notification channel", err) {
		return
	}
	if !checkStatus(&resp.Diagnostics, "Could not create notification channel", res.HTTPResponse, res.Body) {
		return
	}
	if res.JSON200 == nil || res.JSON200.Id == nil {
		resp.Diagnostics.AddError("Could not create notification channel", "API response contained no channel id")
		return
	}

	plan.Id = types.StringValue(*res.JSON200.Id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *NotificationChannelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state NotificationChannelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.GetV1NotificationChannelsNotificationChannelIdWithResponse(ctx, state.Id.ValueString())
	if apiCallFailed(&resp.Diagnostics, "Could not read notification channel", err) {
		return
	}
	if isNotFound(res.HTTPResponse) {
		resp.State.RemoveResource(ctx)
		return
	}
	if !checkStatus(&resp.Diagnostics, "Could not read notification channel", res.HTTPResponse, res.Body) {
		return
	}
	channel := res.JSON200
	if channel == nil {
		resp.Diagnostics.AddError("Could not read notification channel", "empty response body")
		return
	}

	state.Name = types.StringValue(channel.Name)
	state.Tags = stringSliceToList(ctx, &resp.Diagnostics, channel.Tags)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *NotificationChannelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan NotificationChannelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := apiclient.PutV1NotificationChannelsNotificationChannelIdJSONRequestBody{
		Name: plan.Name.ValueString(),
		Tags: ptr(listToStringSlice(ctx, &resp.Diagnostics, plan.Tags)),
	}
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.PutV1NotificationChannelsNotificationChannelIdWithResponse(ctx, plan.Id.ValueString(), body)
	if apiCallFailed(&resp.Diagnostics, "Could not update notification channel", err) {
		return
	}
	if !checkStatus(&resp.Diagnostics, "Could not update notification channel", res.HTTPResponse, res.Body) {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *NotificationChannelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state NotificationChannelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.DeleteV1NotificationChannelsNotificationChannelIdWithResponse(ctx, state.Id.ValueString())
	if apiCallFailed(&resp.Diagnostics, "Could not delete notification channel", err) {
		return
	}
	if isNotFound(res.HTTPResponse) {
		return // already gone
	}
	checkStatus(&resp.Diagnostics, "Could not delete notification channel", res.HTTPResponse, res.Body)
}

func (r *NotificationChannelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
