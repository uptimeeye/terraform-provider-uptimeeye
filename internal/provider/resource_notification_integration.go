package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"gitlab.com/uptimeeye/terraform-provider-uptimeeye/internal/apiclient"
)

var (
	_ resource.Resource                     = &NotificationIntegrationResource{}
	_ resource.ResourceWithConfigure        = &NotificationIntegrationResource{}
	_ resource.ResourceWithConfigValidators = &NotificationIntegrationResource{}
	_ resource.ResourceWithImportState      = &NotificationIntegrationResource{}
)

func NewNotificationIntegrationResource() resource.Resource {
	return &NotificationIntegrationResource{}
}

type NotificationIntegrationResource struct {
	client *apiclient.ClientWithResponses
}

type NotificationIntegrationResourceModel struct {
	Id        types.String `tfsdk:"id"`
	ChannelId types.String `tfsdk:"channel_id"`
	Name      types.String `tfsdk:"name"`
	Slack     types.Object `tfsdk:"slack"`
	Teams     types.Object `tfsdk:"teams"`
	Discord   types.Object `tfsdk:"discord"`
	Email     types.Object `tfsdk:"email"`
	Webhook   types.Object `tfsdk:"webhook"`
	Telegram  types.Object `tfsdk:"telegram"`
	Pagerduty types.Object `tfsdk:"pagerduty"`
}

type slackIntegrationModel struct {
	WebhookUrl types.String `tfsdk:"webhook_url"`
	Channel    types.String `tfsdk:"channel"`
}

type teamsIntegrationModel struct {
	WebhookUrl types.String `tfsdk:"webhook_url"`
}

type discordIntegrationModel struct {
	WebhookUrl types.String `tfsdk:"webhook_url"`
}

type emailIntegrationModel struct {
	To types.List `tfsdk:"to"`
}

type webhookIntegrationModel struct {
	WebhookUrl types.String `tfsdk:"webhook_url"`
}

type telegramIntegrationModel struct {
	ChatId types.String `tfsdk:"chat_id"`
}

type pagerdutyIntegrationModel struct {
	RoutingKey types.String `tfsdk:"routing_key"`
	Source     types.String `tfsdk:"source"`
}

var slackIntegrationAttrTypes = map[string]attr.Type{
	"webhook_url": types.StringType,
	"channel":     types.StringType,
}

var teamsIntegrationAttrTypes = map[string]attr.Type{
	"webhook_url": types.StringType,
}

var discordIntegrationAttrTypes = map[string]attr.Type{
	"webhook_url": types.StringType,
}

var emailIntegrationAttrTypes = map[string]attr.Type{
	"to": types.ListType{ElemType: types.StringType},
}

var webhookIntegrationAttrTypes = map[string]attr.Type{
	"webhook_url": types.StringType,
}

var telegramIntegrationAttrTypes = map[string]attr.Type{
	"chat_id": types.StringType,
}

var pagerdutyIntegrationAttrTypes = map[string]attr.Type{
	"routing_key": types.StringType,
	"source":      types.StringType,
}

func (r *NotificationIntegrationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notification_integration"
}

func (r *NotificationIntegrationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A notification integration delivers alerts of a notification channel to a specific " +
			"target (Slack, Teams, Discord, email, webhook, Telegram, or PagerDuty). Exactly one integration " +
			"type attribute must be configured.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Integration identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"channel_id": schema.StringAttribute{
				MarkdownDescription: "ID of the notification channel this integration belongs to. Changing this forces a new integration.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the integration.",
				Required:            true,
			},
			"slack": schema.SingleNestedAttribute{
				MarkdownDescription: "Slack integration. The API never returns the webhook URL, so drift on it cannot be detected.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"webhook_url": schema.StringAttribute{
						MarkdownDescription: "Slack incoming webhook URL.",
						Required:            true,
						Sensitive:           true,
					},
					"channel": schema.StringAttribute{
						MarkdownDescription: "Slack channel to post into.",
						Required:            true,
					},
				},
			},
			"teams": schema.SingleNestedAttribute{
				MarkdownDescription: "Microsoft Teams integration.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"webhook_url": schema.StringAttribute{
						MarkdownDescription: "Teams incoming webhook URL.",
						Required:            true,
						Sensitive:           true,
					},
				},
			},
			"discord": schema.SingleNestedAttribute{
				MarkdownDescription: "Discord integration.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"webhook_url": schema.StringAttribute{
						MarkdownDescription: "Discord webhook URL.",
						Required:            true,
						Sensitive:           true,
					},
				},
			},
			"email": schema.SingleNestedAttribute{
				MarkdownDescription: "Email integration.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"to": schema.ListAttribute{
						MarkdownDescription: "Recipient email addresses.",
						ElementType:         types.StringType,
						Required:            true,
						Validators: []validator.List{
							listvalidator.SizeAtLeast(1),
						},
					},
				},
			},
			"webhook": schema.SingleNestedAttribute{
				MarkdownDescription: "Generic webhook integration.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"webhook_url": schema.StringAttribute{
						MarkdownDescription: "URL that receives alert payloads.",
						Required:            true,
					},
				},
			},
			"telegram": schema.SingleNestedAttribute{
				MarkdownDescription: "Telegram integration.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"chat_id": schema.StringAttribute{
						MarkdownDescription: "Telegram chat ID that receives alerts.",
						Required:            true,
					},
				},
			},
			"pagerduty": schema.SingleNestedAttribute{
				MarkdownDescription: "PagerDuty integration.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"routing_key": schema.StringAttribute{
						MarkdownDescription: "PagerDuty Events API v2 routing key.",
						Required:            true,
						Sensitive:           true,
					},
					"source": schema.StringAttribute{
						MarkdownDescription: "Source reported to PagerDuty.",
						Optional:            true,
						Computed:            true,
						Default:             stringdefault.StaticString(""),
					},
				},
			},
		},
	}
}

func (r *NotificationIntegrationResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("slack"),
			path.MatchRoot("teams"),
			path.MatchRoot("discord"),
			path.MatchRoot("email"),
			path.MatchRoot("webhook"),
			path.MatchRoot("telegram"),
			path.MatchRoot("pagerduty"),
		),
	}
}

func (r *NotificationIntegrationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

// isSet reports whether an optional nested object attribute carries a value.
func isSet(obj types.Object) bool {
	return !obj.IsNull() && !obj.IsUnknown()
}

// notificationIntegrationBody converts the plan into an API request body,
// deriving the integration type from whichever nested attribute is set.
func notificationIntegrationBody(ctx context.Context, diags *diag.Diagnostics, plan NotificationIntegrationResourceModel) apiclient.NotificationIntegration {
	body := apiclient.NotificationIntegration{
		Name: plan.Name.ValueString(),
	}

	switch {
	case isSet(plan.Slack):
		var m slackIntegrationModel
		diags.Append(plan.Slack.As(ctx, &m, basetypes.ObjectAsOptions{})...)
		body.Type = apiclient.Slack
		body.Configuration.Slack = &apiclient.SlackConfiguration{
			WebhookUrl:   m.WebhookUrl.ValueString(),
			Channel:      m.Channel.ValueString(),
			SlackWebhook: nil,
		}
	case isSet(plan.Teams):
		var m teamsIntegrationModel
		diags.Append(plan.Teams.As(ctx, &m, basetypes.ObjectAsOptions{})...)
		body.Type = apiclient.Teams
		body.Configuration.Teams = &apiclient.TeamsConfiguration{
			WebhookUrl: m.WebhookUrl.ValueString(),
		}
	case isSet(plan.Discord):
		var m discordIntegrationModel
		diags.Append(plan.Discord.As(ctx, &m, basetypes.ObjectAsOptions{})...)
		body.Type = apiclient.Discord
		body.Configuration.Discord = &apiclient.DiscordConfiguration{
			WebhookUrl: m.WebhookUrl.ValueString(),
		}
	case isSet(plan.Email):
		var m emailIntegrationModel
		diags.Append(plan.Email.As(ctx, &m, basetypes.ObjectAsOptions{})...)
		body.Type = apiclient.Email
		body.Configuration.Email = &apiclient.EmailConfiguration{
			To: ptr(listToStringSlice(ctx, diags, m.To)),
		}
	case isSet(plan.Webhook):
		var m webhookIntegrationModel
		diags.Append(plan.Webhook.As(ctx, &m, basetypes.ObjectAsOptions{})...)
		body.Type = apiclient.Webhook
		body.Configuration.Webhook = &apiclient.WebhookConfiguration{
			WebhookUrl: m.WebhookUrl.ValueString(),
		}
	case isSet(plan.Telegram):
		var m telegramIntegrationModel
		diags.Append(plan.Telegram.As(ctx, &m, basetypes.ObjectAsOptions{})...)
		body.Type = apiclient.Telegram
		body.Configuration.Telegram = &apiclient.TelegramConfiguration{
			ChatId: m.ChatId.ValueString(),
		}
	case isSet(plan.Pagerduty):
		var m pagerdutyIntegrationModel
		diags.Append(plan.Pagerduty.As(ctx, &m, basetypes.ObjectAsOptions{})...)
		body.Type = apiclient.Pagerduty
		body.Configuration.Pagerduty = &apiclient.PagerDutyConfiguration{
			RoutingKey: m.RoutingKey.ValueString(),
			Source:     ptr(m.Source.ValueString()),
		}
	default:
		diags.AddError(
			"Invalid notification integration configuration",
			"exactly one integration type attribute (slack, teams, discord, email, webhook, telegram, pagerduty) must be set",
		)
	}
	return body
}

// clearIntegrationBlocks nulls all seven type attributes; Read re-fills the
// one matching the API response afterwards.
func clearIntegrationBlocks(state *NotificationIntegrationResourceModel) {
	state.Slack = types.ObjectNull(slackIntegrationAttrTypes)
	state.Teams = types.ObjectNull(teamsIntegrationAttrTypes)
	state.Discord = types.ObjectNull(discordIntegrationAttrTypes)
	state.Email = types.ObjectNull(emailIntegrationAttrTypes)
	state.Webhook = types.ObjectNull(webhookIntegrationAttrTypes)
	state.Telegram = types.ObjectNull(telegramIntegrationAttrTypes)
	state.Pagerduty = types.ObjectNull(pagerdutyIntegrationAttrTypes)
}

func (r *NotificationIntegrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan NotificationIntegrationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := notificationIntegrationBody(ctx, &resp.Diagnostics, plan)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.PostV1NotificationChannelsNotificationChannelIdIntegrationsWithResponse(ctx, plan.ChannelId.ValueString(), body)
	if apiCallFailed(&resp.Diagnostics, "Could not create notification integration", err) {
		return
	}
	if !checkStatus(&resp.Diagnostics, "Could not create notification integration", res.HTTPResponse, res.Body) {
		return
	}
	if res.JSON200 == nil || res.JSON200.Id == nil {
		resp.Diagnostics.AddError("Could not create notification integration", "API response contained no integration id")
		return
	}

	plan.Id = types.StringValue(*res.JSON200.Id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *NotificationIntegrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state NotificationIntegrationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.GetV1NotificationChannelsNotificationChannelIdIntegrationsIntegrationIdWithResponse(ctx, state.ChannelId.ValueString(), state.Id.ValueString())
	if apiCallFailed(&resp.Diagnostics, "Could not read notification integration", err) {
		return
	}
	if isNotFound(res.HTTPResponse) {
		resp.State.RemoveResource(ctx)
		return
	}
	if !checkStatus(&resp.Diagnostics, "Could not read notification integration", res.HTTPResponse, res.Body) {
		return
	}
	integration := res.JSON200
	if integration == nil {
		resp.Diagnostics.AddError("Could not read notification integration", "empty response body")
		return
	}

	state.Name = types.StringValue(integration.Name)

	// For each type: refresh the state attribute from the response, but only
	// when the matching configuration sub-struct is present — otherwise keep
	// the prior state block untouched.
	switch integration.Type {
	case apiclient.Slack:
		if cfg := integration.Configuration.Slack; cfg != nil {
			// API quirk: reads omit the Slack webhook URL. Keep the prior
			// state value (null after import) and only refresh the channel.
			var prior slackIntegrationModel
			if isSet(state.Slack) {
				resp.Diagnostics.Append(state.Slack.As(ctx, &prior, basetypes.ObjectAsOptions{})...)
			}
			clearIntegrationBlocks(&state)
			obj, d := types.ObjectValueFrom(ctx, slackIntegrationAttrTypes, slackIntegrationModel{
				WebhookUrl: prior.WebhookUrl,
				Channel:    types.StringValue(cfg.Channel),
			})
			resp.Diagnostics.Append(d...)
			state.Slack = obj
		}
	case apiclient.Teams:
		if cfg := integration.Configuration.Teams; cfg != nil {
			clearIntegrationBlocks(&state)
			obj, d := types.ObjectValueFrom(ctx, teamsIntegrationAttrTypes, teamsIntegrationModel{
				WebhookUrl: types.StringValue(cfg.WebhookUrl),
			})
			resp.Diagnostics.Append(d...)
			state.Teams = obj
		}
	case apiclient.Discord:
		if cfg := integration.Configuration.Discord; cfg != nil {
			clearIntegrationBlocks(&state)
			obj, d := types.ObjectValueFrom(ctx, discordIntegrationAttrTypes, discordIntegrationModel{
				WebhookUrl: types.StringValue(cfg.WebhookUrl),
			})
			resp.Diagnostics.Append(d...)
			state.Discord = obj
		}
	case apiclient.Email:
		if cfg := integration.Configuration.Email; cfg != nil {
			clearIntegrationBlocks(&state)
			obj, d := types.ObjectValueFrom(ctx, emailIntegrationAttrTypes, emailIntegrationModel{
				To: stringSliceToList(ctx, &resp.Diagnostics, cfg.To),
			})
			resp.Diagnostics.Append(d...)
			state.Email = obj
		}
	case apiclient.Webhook:
		if cfg := integration.Configuration.Webhook; cfg != nil {
			clearIntegrationBlocks(&state)
			obj, d := types.ObjectValueFrom(ctx, webhookIntegrationAttrTypes, webhookIntegrationModel{
				WebhookUrl: types.StringValue(cfg.WebhookUrl),
			})
			resp.Diagnostics.Append(d...)
			state.Webhook = obj
		}
	case apiclient.Telegram:
		if cfg := integration.Configuration.Telegram; cfg != nil {
			clearIntegrationBlocks(&state)
			obj, d := types.ObjectValueFrom(ctx, telegramIntegrationAttrTypes, telegramIntegrationModel{
				ChatId: types.StringValue(cfg.ChatId),
			})
			resp.Diagnostics.Append(d...)
			state.Telegram = obj
		}
	case apiclient.Pagerduty:
		if cfg := integration.Configuration.Pagerduty; cfg != nil {
			clearIntegrationBlocks(&state)
			obj, d := types.ObjectValueFrom(ctx, pagerdutyIntegrationAttrTypes, pagerdutyIntegrationModel{
				RoutingKey: types.StringValue(cfg.RoutingKey),
				Source:     types.StringValue(deref(cfg.Source, "")),
			})
			resp.Diagnostics.Append(d...)
			state.Pagerduty = obj
		}
	}

	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *NotificationIntegrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan NotificationIntegrationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := notificationIntegrationBody(ctx, &resp.Diagnostics, plan)
	if resp.Diagnostics.HasError() {
		return
	}
	// The update endpoint identifies the integration by the id in the body.
	body.Id = ptr(plan.Id.ValueString())

	res, err := r.client.PutV1NotificationChannelsNotificationChannelIdIntegrationsWithResponse(ctx, plan.ChannelId.ValueString(), body)
	if apiCallFailed(&resp.Diagnostics, "Could not update notification integration", err) {
		return
	}
	if !checkStatus(&resp.Diagnostics, "Could not update notification integration", res.HTTPResponse, res.Body) {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *NotificationIntegrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state NotificationIntegrationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.DeleteV1NotificationChannelsNotificationChannelIdIntegrationsIntegrationIdWithResponse(ctx, state.ChannelId.ValueString(), state.Id.ValueString())
	if apiCallFailed(&resp.Diagnostics, "Could not delete notification integration", err) {
		return
	}
	if isNotFound(res.HTTPResponse) {
		return // already gone
	}
	checkStatus(&resp.Diagnostics, "Could not delete notification integration", res.HTTPResponse, res.Body)
}

func (r *NotificationIntegrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	channelId, integrationId, found := strings.Cut(req.ID, "/")
	if !found || channelId == "" || integrationId == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("expected import ID in the form \"channel_id/integration_id\", got: %q", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("channel_id"), channelId)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), integrationId)...)
}
