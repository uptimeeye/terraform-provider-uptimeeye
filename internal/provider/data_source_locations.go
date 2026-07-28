package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"gitlab.com/uptimeeye/terraform-provider-uptimeeye/internal/apiclient"
)

var (
	_ datasource.DataSource              = &LocationsDataSource{}
	_ datasource.DataSourceWithConfigure = &LocationsDataSource{}
)

func NewLocationsDataSource() datasource.DataSource {
	return &LocationsDataSource{}
}

type LocationsDataSource struct {
	client *apiclient.ClientWithResponses
}

type LocationsDataSourceModel struct {
	Locations types.List `tfsdk:"locations"`
}

func (d *LocationsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_locations"
}

func (d *LocationsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The pinger locations (regions) available to run monitor checks from. Use this to reference valid `locations` values for `uptimeeye_monitor`.",
		Attributes: map[string]schema.Attribute{
			"locations": schema.ListAttribute{
				MarkdownDescription: "Identifiers of the available pinger locations.",
				ElementType:         types.StringType,
				Computed:            true,
			},
		},
	}
}

func (d *LocationsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *LocationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state LocationsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := d.client.GetV1LocationsWithResponse(ctx)
	if apiCallFailed(&resp.Diagnostics, "Could not read locations", err) {
		return
	}
	if !checkStatus(&resp.Diagnostics, "Could not read locations", res.HTTPResponse, res.Body) {
		return
	}
	if res.JSON200 == nil {
		resp.Diagnostics.AddError("Could not read locations", "empty response body")
		return
	}

	state.Locations = stringSliceToList(ctx, &resp.Diagnostics, res.JSON200.Locations)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
