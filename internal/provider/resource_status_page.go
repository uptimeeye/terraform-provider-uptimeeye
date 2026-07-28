package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"gitlab.com/uptimeeye/terraform-provider-uptimeeye/internal/apiclient"
)

var (
	_ resource.Resource                = &StatusPageResource{}
	_ resource.ResourceWithConfigure   = &StatusPageResource{}
	_ resource.ResourceWithImportState = &StatusPageResource{}
)

func NewStatusPageResource() resource.Resource {
	return &StatusPageResource{}
}

type StatusPageResource struct {
	client *apiclient.ClientWithResponses
}

type StatusPageResourceModel struct {
	Id                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	Slug                 types.String `tfsdk:"slug"`
	Tags                 types.List   `tfsdk:"tags"`
	Theme                types.String `tfsdk:"theme"`
	LogoUrl              types.String `tfsdk:"logo_url"`
	BrandColor           types.String `tfsdk:"brand_color"`
	HidePoweredBy        types.Bool   `tfsdk:"hide_powered_by"`
	SubscriptionsEnabled types.Bool   `tfsdk:"subscriptions_enabled"`
	Sections             types.List   `tfsdk:"sections"`
}

type statusPageSectionModel struct {
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	Monitors       types.List   `tfsdk:"monitors"`
	ScheduledTasks types.List   `tfsdk:"scheduled_tasks"`
}

type statusPageSectionMonitorModel struct {
	MonitorId        types.String `tfsdk:"monitor_id"`
	ShowUptime       types.Bool   `tfsdk:"show_uptime"`
	ShowResponseTime types.Bool   `tfsdk:"show_response_time"`
}

type statusPageSectionScheduledTaskModel struct {
	ScheduledTaskId types.String `tfsdk:"scheduled_task_id"`
}

func statusPageSectionMonitorAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"monitor_id":         types.StringType,
		"show_uptime":        types.BoolType,
		"show_response_time": types.BoolType,
	}
}

func statusPageSectionScheduledTaskAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"scheduled_task_id": types.StringType,
	}
}

func statusPageSectionAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":            types.StringType,
		"description":     types.StringType,
		"monitors":        types.ListType{ElemType: types.ObjectType{AttrTypes: statusPageSectionMonitorAttrTypes()}},
		"scheduled_tasks": types.ListType{ElemType: types.ObjectType{AttrTypes: statusPageSectionScheduledTaskAttrTypes()}},
	}
}

// emptyStatusPageObjectList builds an empty list of objects with the given
// attribute types, used as schema default for nested lists.
func emptyStatusPageObjectList(attrTypes map[string]attr.Type) types.List {
	return types.ListValueMust(types.ObjectType{AttrTypes: attrTypes}, []attr.Value{})
}

func (r *StatusPageResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_status_page"
}

func (r *StatusPageResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A public or private status page. Sections (with their monitors and scheduled tasks) " +
			"are part of the page document and are upserted atomically with the page — they have no API of their own. " +
			"Server-managed and plan-gated settings (subscribers, white-label, custom-domain flags) are not modeled here.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Status page identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the status page.",
				Required:            true,
			},
			"slug": schema.StringAttribute{
				MarkdownDescription: "URL slug under which the page is served.",
				Required:            true,
			},
			"tags": schema.ListAttribute{
				MarkdownDescription: "Tags for organizing status pages.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(emptyStringList()),
			},
			"theme": schema.StringAttribute{
				MarkdownDescription: "Color theme of the page. One of `dark`, `light`, `auto`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(string(apiclient.Dark)),
				Validators: []validator.String{
					stringvalidator.OneOf(string(apiclient.Dark), string(apiclient.Light), string(apiclient.Auto)),
				},
			},
			"logo_url": schema.StringAttribute{
				MarkdownDescription: "URL of the logo shown on the page.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
			},
			"brand_color": schema.StringAttribute{
				MarkdownDescription: "Brand accent color (e.g. `#1a2b3c`).",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
			},
			"hide_powered_by": schema.BoolAttribute{
				MarkdownDescription: "Hide the \"powered by UptimeEye\" footer (requires a plan that allows it).",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"subscriptions_enabled": schema.BoolAttribute{
				MarkdownDescription: "Allow visitors to subscribe to status updates.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"sections": schema.ListNestedAttribute{
				MarkdownDescription: "Ordered sections of the page. Each section groups monitors and scheduled tasks.",
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(emptyStatusPageObjectList(statusPageSectionAttrTypes())),
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "Section heading.",
							Required:            true,
						},
						"description": schema.StringAttribute{
							MarkdownDescription: "Section description shown under the heading.",
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString(""),
						},
						"monitors": schema.ListNestedAttribute{
							MarkdownDescription: "Monitors displayed in this section, in order.",
							Optional:            true,
							Computed:            true,
							Default:             listdefault.StaticValue(emptyStatusPageObjectList(statusPageSectionMonitorAttrTypes())),
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"monitor_id": schema.StringAttribute{
										MarkdownDescription: "Id of the monitor to display.",
										Required:            true,
									},
									"show_uptime": schema.BoolAttribute{
										MarkdownDescription: "Show the uptime percentage for this monitor.",
										Optional:            true,
										Computed:            true,
										Default:             booldefault.StaticBool(true),
									},
									"show_response_time": schema.BoolAttribute{
										MarkdownDescription: "Show the response time chart for this monitor.",
										Optional:            true,
										Computed:            true,
										Default:             booldefault.StaticBool(false),
									},
								},
							},
						},
						"scheduled_tasks": schema.ListNestedAttribute{
							MarkdownDescription: "Scheduled tasks displayed in this section, in order.",
							Optional:            true,
							Computed:            true,
							Default:             listdefault.StaticValue(emptyStatusPageObjectList(statusPageSectionScheduledTaskAttrTypes())),
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"scheduled_task_id": schema.StringAttribute{
										MarkdownDescription: "Id of the scheduled task to display.",
										Required:            true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *StatusPageResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

// buildStatusPageBody converts the fully-known plan into the API payload. The
// API upserts the whole page document atomically; section (and nested) ids are
// never sent — the server re-creates them. pageId is empty on create and the
// real page id on update (each section carries its page id in the DTO).
func buildStatusPageBody(ctx context.Context, diags *diag.Diagnostics, plan *StatusPageResourceModel, pageId string) apiclient.StatusPage {
	var sectionModels []statusPageSectionModel
	diags.Append(plan.Sections.ElementsAs(ctx, &sectionModels, false)...)
	if diags.HasError() {
		return apiclient.StatusPage{}
	}

	sections := make([]apiclient.StatusPageSection, 0, len(sectionModels))
	for _, s := range sectionModels {
		var monitorModels []statusPageSectionMonitorModel
		diags.Append(s.Monitors.ElementsAs(ctx, &monitorModels, false)...)

		var taskModels []statusPageSectionScheduledTaskModel
		diags.Append(s.ScheduledTasks.ElementsAs(ctx, &taskModels, false)...)
		if diags.HasError() {
			return apiclient.StatusPage{}
		}

		monitors := make([]apiclient.StatusPageSectionMonitor, 0, len(monitorModels))
		for _, m := range monitorModels {
			monitors = append(monitors, apiclient.StatusPageSectionMonitor{
				MonitorId:        m.MonitorId.ValueString(),
				ShowUptime:       m.ShowUptime.ValueBool(),
				ShowResponseTime: m.ShowResponseTime.ValueBool(),
			})
		}

		tasks := make([]apiclient.StatusPageSectionScheduledTask, 0, len(taskModels))
		for _, t := range taskModels {
			tasks = append(tasks, apiclient.StatusPageSectionScheduledTask{
				ScheduledTaskId: t.ScheduledTaskId.ValueString(),
			})
		}

		sections = append(sections, apiclient.StatusPageSection{
			Name:           s.Name.ValueString(),
			Description:    s.Description.ValueString(),
			Monitors:       ptr(monitors),
			ScheduledTasks: ptr(tasks),
			StatusPageId:   pageId,
		})
	}

	return apiclient.StatusPage{
		Name: plan.Name.ValueString(),
		Slug: plan.Slug.ValueString(),
		// The backend forces status pages to be public; the DTO field only
		// remains for API compatibility and is not part of the schema.
		IsPublic: true,
		Tags:                 ptr(listToStringSlice(ctx, diags, plan.Tags)),
		Theme:                ptr(apiclient.StatusPageTheme(plan.Theme.ValueString())),
		LogoUrl:              ptr(plan.LogoUrl.ValueString()),
		BrandColor:           ptr(plan.BrandColor.ValueString()),
		HidePoweredBy:        ptr(plan.HidePoweredBy.ValueBool()),
		SubscriptionsEnabled: ptr(plan.SubscriptionsEnabled.ValueBool()),
		Sections:             ptr(sections),
	}
}

// statusPageToState maps an API status page onto the resource model, keeping
// the sections in the order the API returned them.
func statusPageToState(ctx context.Context, diags *diag.Diagnostics, page *apiclient.StatusPage, state *StatusPageResourceModel) {
	if page.Id != nil {
		state.Id = types.StringValue(*page.Id)
	}
	state.Name = types.StringValue(page.Name)
	state.Slug = types.StringValue(page.Slug)
	state.Tags = stringSliceToList(ctx, diags, page.Tags)
	state.Theme = types.StringValue(string(deref(page.Theme, apiclient.Dark)))
	state.LogoUrl = types.StringValue(deref(page.LogoUrl, ""))
	state.BrandColor = types.StringValue(deref(page.BrandColor, ""))
	state.HidePoweredBy = types.BoolValue(deref(page.HidePoweredBy, false))
	state.SubscriptionsEnabled = types.BoolValue(deref(page.SubscriptionsEnabled, true))

	apiSections := []apiclient.StatusPageSection{}
	if page.Sections != nil {
		apiSections = *page.Sections
	}

	sectionModels := make([]statusPageSectionModel, 0, len(apiSections))
	for _, s := range apiSections {
		monitorModels := []statusPageSectionMonitorModel{}
		if s.Monitors != nil {
			for _, m := range *s.Monitors {
				monitorModels = append(monitorModels, statusPageSectionMonitorModel{
					MonitorId:        types.StringValue(m.MonitorId),
					ShowUptime:       types.BoolValue(m.ShowUptime),
					ShowResponseTime: types.BoolValue(m.ShowResponseTime),
				})
			}
		}
		monitorList, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: statusPageSectionMonitorAttrTypes()}, monitorModels)
		diags.Append(d...)

		taskModels := []statusPageSectionScheduledTaskModel{}
		if s.ScheduledTasks != nil {
			for _, t := range *s.ScheduledTasks {
				taskModels = append(taskModels, statusPageSectionScheduledTaskModel{
					ScheduledTaskId: types.StringValue(t.ScheduledTaskId),
				})
			}
		}
		taskList, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: statusPageSectionScheduledTaskAttrTypes()}, taskModels)
		diags.Append(d...)

		sectionModels = append(sectionModels, statusPageSectionModel{
			Name:           types.StringValue(s.Name),
			Description:    types.StringValue(s.Description),
			Monitors:       monitorList,
			ScheduledTasks: taskList,
		})
	}

	sectionList, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: statusPageSectionAttrTypes()}, sectionModels)
	diags.Append(d...)
	state.Sections = sectionList
}

func (r *StatusPageResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan StatusPageResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := buildStatusPageBody(ctx, &resp.Diagnostics, &plan, "")
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.PostV1StatusPagesWithResponse(ctx, body)
	if apiCallFailed(&resp.Diagnostics, "Could not create status page", err) {
		return
	}
	if !checkStatus(&resp.Diagnostics, "Could not create status page", res.HTTPResponse, res.Body) {
		return
	}
	if res.JSON200 == nil || res.JSON200.StatusPage.Id == nil {
		resp.Diagnostics.AddError("Could not create status page", "API response contained no status page id")
		return
	}

	plan.Id = types.StringValue(*res.JSON200.StatusPage.Id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *StatusPageResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state StatusPageResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.GetV1StatusPagesStatusPageIdWithResponse(ctx, state.Id.ValueString())
	if apiCallFailed(&resp.Diagnostics, "Could not read status page", err) {
		return
	}
	if isNotFound(res.HTTPResponse) {
		resp.State.RemoveResource(ctx)
		return
	}
	if !checkStatus(&resp.Diagnostics, "Could not read status page", res.HTTPResponse, res.Body) {
		return
	}
	if res.JSON200 == nil {
		resp.Diagnostics.AddError("Could not read status page", "empty response body")
		return
	}

	statusPageToState(ctx, &resp.Diagnostics, res.JSON200, &state)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *StatusPageResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan StatusPageResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := buildStatusPageBody(ctx, &resp.Diagnostics, &plan, plan.Id.ValueString())
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.PutV1StatusPagesStatusPageIdWithResponse(ctx, plan.Id.ValueString(), body)
	if apiCallFailed(&resp.Diagnostics, "Could not update status page", err) {
		return
	}
	if !checkStatus(&resp.Diagnostics, "Could not update status page", res.HTTPResponse, res.Body) {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *StatusPageResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state StatusPageResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.DeleteV1StatusPagesStatusPageIdWithResponse(ctx, state.Id.ValueString())
	if apiCallFailed(&resp.Diagnostics, "Could not delete status page", err) {
		return
	}
	if isNotFound(res.HTTPResponse) {
		return // already gone
	}
	checkStatus(&resp.Diagnostics, "Could not delete status page", res.HTTPResponse, res.Body)
}

func (r *StatusPageResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
