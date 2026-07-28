package provider

import (
	"context"
	"net/http"
	"os"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"gitlab.com/uptimeeye/terraform-provider-uptimeeye/internal/apiclient"
)

const defaultEndpoint = "https://api.uptimeeye.com"

// Ensure UptimeEyeProvider satisfies the provider interface.
var _ provider.Provider = &UptimeEyeProvider{}

type UptimeEyeProvider struct {
	version string
}

type UptimeEyeProviderModel struct {
	ApiKey   types.String `tfsdk:"api_key"`
	Endpoint types.String `tfsdk:"endpoint"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &UptimeEyeProvider{version: version}
	}
}

func (p *UptimeEyeProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "uptimeeye"
	resp.Version = p.version
}

func (p *UptimeEyeProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage UptimeEye monitors, scheduled tasks, notification channels, status pages and variables declaratively.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				MarkdownDescription: "Organization API key (`ue_live_...`), created in the UptimeEye app under Settings → API Keys. Can also be set via the `UPTIMEEYE_API_KEY` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "UptimeEye management API endpoint. Defaults to `" + defaultEndpoint + "`. Can also be set via the `UPTIMEEYE_ENDPOINT` environment variable.",
				Optional:            true,
			},
		},
	}
}

func (p *UptimeEyeProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config UptimeEyeProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := os.Getenv("UPTIMEEYE_API_KEY")
	if !config.ApiKey.IsNull() {
		apiKey = config.ApiKey.ValueString()
	}
	if apiKey == "" {
		resp.Diagnostics.AddError(
			"Missing UptimeEye API key",
			"Set the provider attribute `api_key` or the environment variable UPTIMEEYE_API_KEY. "+
				"Keys are created in the UptimeEye app under Settings → API Keys.",
		)
		return
	}

	endpoint := os.Getenv("UPTIMEEYE_ENDPOINT")
	if !config.Endpoint.IsNull() {
		endpoint = config.Endpoint.ValueString()
	}
	if endpoint == "" {
		endpoint = defaultEndpoint
	}

	// Retry transient failures (429, 5xx, network errors) with exponential
	// backoff; go-retryablehttp honors Retry-After.
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 4
	retryClient.Logger = nil

	userAgent := "terraform-provider-uptimeeye/" + p.version

	client, err := apiclient.NewClientWithResponses(
		endpoint,
		apiclient.WithHTTPClient(retryClient.StandardClient()),
		apiclient.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer "+apiKey)
			req.Header.Set("User-Agent", userAgent)
			return nil
		}),
	)
	if err != nil {
		resp.Diagnostics.AddError("Could not create UptimeEye API client", err.Error())
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *UptimeEyeProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewMonitorResource,
		NewScheduledTaskResource,
		NewNotificationChannelResource,
		NewNotificationIntegrationResource,
		NewVariableResource,
		NewStatusPageResource,
		NewStatusPageDomainResource,
		NewReportScheduleResource,
	}
}

func (p *UptimeEyeProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewLocationsDataSource,
		NewNotificationChannelDataSource,
	}
}
