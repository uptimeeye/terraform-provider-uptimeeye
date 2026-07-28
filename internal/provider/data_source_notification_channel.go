package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"gitlab.com/uptimeeye/terraform-provider-uptimeeye/internal/apiclient"
)

var (
	_ datasource.DataSource              = &NotificationChannelDataSource{}
	_ datasource.DataSourceWithConfigure = &NotificationChannelDataSource{}
)

func NewNotificationChannelDataSource() datasource.DataSource {
	return &NotificationChannelDataSource{}
}

type NotificationChannelDataSource struct {
	client *apiclient.ClientWithResponses
}

type NotificationChannelDataSourceModel struct {
	Id   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	Tags types.List   `tfsdk:"tags"`
}

func (d *NotificationChannelDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notification_channel"
}

func (d *NotificationChannelDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up an existing notification channel by its exact name. Useful when channels are managed outside of Terraform but need to be referenced by monitors or scheduled tasks.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Channel identifier.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the channel to look up. Must match exactly one channel.",
				Required:            true,
			},
			"tags": schema.ListAttribute{
				MarkdownDescription: "Tags for organizing channels.",
				ElementType:         types.StringType,
				Computed:            true,
			},
		},
	}
}

func (d *NotificationChannelDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *NotificationChannelDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state NotificationChannelDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := d.client.GetV1NotificationChannelsWithResponse(ctx)
	if apiCallFailed(&resp.Diagnostics, "Could not read notification channels", err) {
		return
	}
	if !checkStatus(&resp.Diagnostics, "Could not read notification channels", res.HTTPResponse, res.Body) {
		return
	}
	if res.JSON200 == nil {
		resp.Diagnostics.AddError("Could not read notification channels", "empty response body")
		return
	}

	name := state.Name.ValueString()
	var matches []apiclient.NotificationChannel
	if res.JSON200.NotificationChannels != nil {
		for _, channel := range *res.JSON200.NotificationChannels {
			if channel.Name == name {
				matches = append(matches, channel)
			}
		}
	}

	switch len(matches) {
	case 0:
		resp.Diagnostics.AddError(
			"Notification channel not found",
			fmt.Sprintf("no notification channel with name %q exists", name),
		)
		return
	case 1:
		// fall through to mapping below
	default:
		resp.Diagnostics.AddError(
			"Notification channel name is ambiguous",
			fmt.Sprintf("%d notification channels are named %q; look-up by name requires a unique name. Consider importing the uptimeeye_notification_channel resource by id instead.", len(matches), name),
		)
		return
	}

	channel := matches[0]
	state.Id = types.StringValue(deref(channel.Id, ""))
	state.Tags = stringSliceToList(ctx, &resp.Diagnostics, channel.Tags)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
