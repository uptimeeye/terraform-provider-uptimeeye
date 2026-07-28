package provider

import (
	"context"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"gitlab.com/uptimeeye/terraform-provider-uptimeeye/internal/apiclient"
)

var (
	_ resource.Resource                = &MonitorResource{}
	_ resource.ResourceWithConfigure   = &MonitorResource{}
	_ resource.ResourceWithImportState = &MonitorResource{}
)

const monitorPausedStatus = "PAUSED"

func NewMonitorResource() resource.Resource {
	return &MonitorResource{}
}

type MonitorResource struct {
	client *apiclient.ClientWithResponses
}

type MonitorResourceModel struct {
	Id                    types.String `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	NotificationChannelId types.String `tfsdk:"notification_channel_id"`
	Locations             types.List   `tfsdk:"locations"`
	Tags                  types.List   `tfsdk:"tags"`
	Paused                types.Bool   `tfsdk:"paused"`
	Steps                 types.List   `tfsdk:"steps"`
	Options               types.Object `tfsdk:"options"`
}

type monitorStepModel struct {
	Name            types.String `tfsdk:"name"`
	Type            types.String `tfsdk:"type"`
	Request         types.Object `tfsdk:"request"`
	Asserts         types.List   `tfsdk:"asserts"`
	ExtractedValues types.List   `tfsdk:"extracted_values"`
}

type monitorRequestModel struct {
	Url    types.String `tfsdk:"url"`
	Method types.String `tfsdk:"method"`
	Body   types.String `tfsdk:"body"`
	Header types.List   `tfsdk:"header"`
}

type monitorHeaderModel struct {
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

type monitorAssertModel struct {
	Type     types.String `tfsdk:"type"`
	Property types.String `tfsdk:"property"`
	Operator types.String `tfsdk:"operator"`
	Expected types.String `tfsdk:"expected"`
}

type monitorExtractedValueModel struct {
	Name     types.String `tfsdk:"name"`
	Type     types.String `tfsdk:"type"`
	Location types.String `tfsdk:"location"`
}

type monitorOptionsModel struct {
	TickEvery         types.Int64 `tfsdk:"tick_every"`
	Timeout           types.Int64 `tfsdk:"timeout"`
	FailureThreshold  types.Int64 `tfsdk:"failure_threshold"`
	FollowRedirects   types.Bool  `tfsdk:"follow_redirects"`
	LocationThreshold types.Int64 `tfsdk:"location_threshold"`
}

func monitorHeaderAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"key":   types.StringType,
		"value": types.StringType,
	}
}

func monitorRequestAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"url":    types.StringType,
		"method": types.StringType,
		"body":   types.StringType,
		"header": types.ListType{ElemType: types.ObjectType{AttrTypes: monitorHeaderAttrTypes()}},
	}
}

func monitorAssertAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":     types.StringType,
		"property": types.StringType,
		"operator": types.StringType,
		"expected": types.StringType,
	}
}

func monitorExtractedValueAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":     types.StringType,
		"type":     types.StringType,
		"location": types.StringType,
	}
}

func monitorStepAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":             types.StringType,
		"type":             types.StringType,
		"request":          types.ObjectType{AttrTypes: monitorRequestAttrTypes()},
		"asserts":          types.ListType{ElemType: types.ObjectType{AttrTypes: monitorAssertAttrTypes()}},
		"extracted_values": types.ListType{ElemType: types.ObjectType{AttrTypes: monitorExtractedValueAttrTypes()}},
	}
}

func monitorOptionsAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"tick_every":         types.Int64Type,
		"timeout":            types.Int64Type,
		"failure_threshold":  types.Int64Type,
		"follow_redirects":   types.BoolType,
		"location_threshold": types.Int64Type,
	}
}

// emptyMonitorObjectList builds an empty list of objects with the given
// attribute types, used as schema default for nested lists.
func emptyMonitorObjectList(attrTypes map[string]attr.Type) types.List {
	return types.ListValueMust(types.ObjectType{AttrTypes: attrTypes}, []attr.Value{})
}

func (r *MonitorResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_monitor"
}

func (r *MonitorResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A monitor runs one or more test steps from the configured locations and alerts the " +
			"associated notification channel on failure.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Monitor identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the monitor.",
				Required:            true,
			},
			"notification_channel_id": schema.StringAttribute{
				MarkdownDescription: "ID of the notification channel that receives alerts for this monitor.",
				Required:            true,
			},
			"locations": schema.ListAttribute{
				MarkdownDescription: "Locations the monitor runs from.",
				ElementType:         types.StringType,
				Required:            true,
				Validators:          []validator.List{listvalidator.SizeAtLeast(1)},
			},
			"tags": schema.ListAttribute{
				MarkdownDescription: "Tags for organizing monitors.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(emptyStringList()),
			},
			"paused": schema.BoolAttribute{
				MarkdownDescription: "Whether the monitor is paused. Paused monitors do not run checks.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"steps": schema.ListNestedAttribute{
				MarkdownDescription: "Test steps executed in order.",
				Required:            true,
				Validators:          []validator.List{listvalidator.SizeAtLeast(1)},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "Display name of the step.",
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString(""),
						},
						"type": schema.StringAttribute{
							MarkdownDescription: "Step type.",
							Required:            true,
							Validators:          []validator.String{stringvalidator.OneOf("http", "dns", "icmp")},
						},
						"request": schema.SingleNestedAttribute{
							MarkdownDescription: "Request executed by this step.",
							Required:            true,
							Attributes: map[string]schema.Attribute{
								"url": schema.StringAttribute{
									MarkdownDescription: "Target URL (or host for dns/icmp steps).",
									Required:            true,
								},
								"method": schema.StringAttribute{
									MarkdownDescription: "HTTP method.",
									Optional:            true,
									Computed:            true,
									Default:             stringdefault.StaticString("GET"),
									Validators:          []validator.String{stringvalidator.OneOf("HEAD", "GET", "POST", "PUT", "DELETE")},
								},
								"body": schema.StringAttribute{
									MarkdownDescription: "Request body.",
									Optional:            true,
									Computed:            true,
									Default:             stringdefault.StaticString(""),
								},
								"header": schema.ListNestedAttribute{
									MarkdownDescription: "Request headers.",
									Optional:            true,
									Computed:            true,
									Default:             listdefault.StaticValue(emptyMonitorObjectList(monitorHeaderAttrTypes())),
									NestedObject: schema.NestedAttributeObject{
										Attributes: map[string]schema.Attribute{
											"key": schema.StringAttribute{
												MarkdownDescription: "Header name.",
												Required:            true,
											},
											"value": schema.StringAttribute{
												MarkdownDescription: "Header value.",
												Required:            true,
											},
										},
									},
								},
							},
						},
						"asserts": schema.ListNestedAttribute{
							MarkdownDescription: "Assertions evaluated against the step response.",
							Optional:            true,
							Computed:            true,
							Default:             listdefault.StaticValue(emptyMonitorObjectList(monitorAssertAttrTypes())),
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"type": schema.StringAttribute{
										MarkdownDescription: "What part of the response to assert on.",
										Required:            true,
										Validators: []validator.String{stringvalidator.OneOf(
											"body", "bodyJsonPath", "header", "statusCode",
											"dnsRecord", "responseTime", "sslCertificate",
										)},
									},
									"property": schema.StringAttribute{
										MarkdownDescription: "Property the assertion refers to (e.g. header name or JSON path), depending on type.",
										Optional:            true,
										Computed:            true,
										Default:             stringdefault.StaticString(""),
									},
									"operator": schema.StringAttribute{
										MarkdownDescription: "Comparison operator.",
										Required:            true,
										Validators: []validator.String{stringvalidator.OneOf(
											"contains", "doesNotContain", "is", "isNot",
											"lessThan", "lessThanOrEqual", "moreThan", "moreThanOrEqual",
										)},
									},
									"expected": schema.StringAttribute{
										MarkdownDescription: "Expected value.",
										Required:            true,
									},
								},
							},
						},
						"extracted_values": schema.ListNestedAttribute{
							MarkdownDescription: "Values extracted from the step response for use in subsequent steps.",
							Optional:            true,
							Computed:            true,
							Default:             listdefault.StaticValue(emptyMonitorObjectList(monitorExtractedValueAttrTypes())),
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										MarkdownDescription: "Name under which the extracted value is available.",
										Required:            true,
									},
									"type": schema.StringAttribute{
										MarkdownDescription: "Where the value is extracted from.",
										Required:            true,
										Validators: []validator.String{stringvalidator.OneOf(
											"http_body_raw", "http_body_json", "http_header", "status_code",
										)},
									},
									"location": schema.StringAttribute{
										MarkdownDescription: "Header name or JSON path depending on type. Not used with http_body_raw.",
										Optional:            true,
										Computed:            true,
										Default:             stringdefault.StaticString(""),
									},
								},
							},
						},
					},
				},
			},
			"options": schema.SingleNestedAttribute{
				MarkdownDescription: "Scheduling and alerting options.",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"tick_every": schema.Int64Attribute{
						MarkdownDescription: "Check frequency in seconds.",
						Required:            true,
					},
					"timeout": schema.Int64Attribute{
						MarkdownDescription: "How long to wait for a response in seconds.",
						Required:            true,
					},
					"failure_threshold": schema.Int64Attribute{
						MarkdownDescription: "Number of consecutive failures before triggering an alarm.",
						Optional:            true,
						Computed:            true,
						Default:             int64default.StaticInt64(1),
					},
					"follow_redirects": schema.BoolAttribute{
						MarkdownDescription: "Whether to follow HTTP redirects.",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(true),
					},
					"location_threshold": schema.Int64Attribute{
						MarkdownDescription: "Number of locations that must fail before triggering an alarm.",
						Optional:            true,
						Computed:            true,
						Default:             int64default.StaticInt64(1),
					},
				},
			},
		},
	}
}

func (r *MonitorResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

// buildMonitorBody converts the fully-known plan into the API payload. The
// exec number of each step is derived from its list position; step ids are
// never sent — the server replaces steps on update.
func buildMonitorBody(ctx context.Context, diags *diag.Diagnostics, plan *MonitorResourceModel) apiclient.Monitor {
	var stepModels []monitorStepModel
	diags.Append(plan.Steps.ElementsAs(ctx, &stepModels, false)...)

	var optModel monitorOptionsModel
	diags.Append(plan.Options.As(ctx, &optModel, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return apiclient.Monitor{}
	}

	steps := make([]apiclient.MonitorTestStep, 0, len(stepModels))
	for i, step := range stepModels {
		var reqModel monitorRequestModel
		diags.Append(step.Request.As(ctx, &reqModel, basetypes.ObjectAsOptions{})...)

		var headerModels []monitorHeaderModel
		diags.Append(reqModel.Header.ElementsAs(ctx, &headerModels, false)...)

		var assertModels []monitorAssertModel
		diags.Append(step.Asserts.ElementsAs(ctx, &assertModels, false)...)

		var extractedModels []monitorExtractedValueModel
		diags.Append(step.ExtractedValues.ElementsAs(ctx, &extractedModels, false)...)
		if diags.HasError() {
			return apiclient.Monitor{}
		}

		headers := make([]apiclient.Header, 0, len(headerModels))
		for _, h := range headerModels {
			headers = append(headers, apiclient.Header{
				Key:   h.Key.ValueString(),
				Value: h.Value.ValueString(),
			})
		}

		asserts := make([]apiclient.Assert, 0, len(assertModels))
		for _, a := range assertModels {
			asserts = append(asserts, apiclient.Assert{
				Type:     apiclient.AssertType(a.Type.ValueString()),
				Property: ptr(a.Property.ValueString()),
				Operator: apiclient.AssertOperator(a.Operator.ValueString()),
				Expected: a.Expected.ValueString(),
			})
		}

		extracted := make([]apiclient.ExtractedValues, 0, len(extractedModels))
		for _, e := range extractedModels {
			extracted = append(extracted, apiclient.ExtractedValues{
				Name:     e.Name.ValueString(),
				Type:     apiclient.ExtractedValuesType(e.Type.ValueString()),
				Location: e.Location.ValueString(),
			})
		}

		steps = append(steps, apiclient.MonitorTestStep{
			ExecNumber: int64(i + 1),
			Name:       ptr(step.Name.ValueString()),
			Type:       apiclient.MonitorTestStepType(step.Type.ValueString()),
			Request: apiclient.RequestTemplate{
				Url:    reqModel.Url.ValueString(),
				Method: ptr(reqModel.Method.ValueString()),
				Body:   ptr(reqModel.Body.ValueString()),
				Header: ptr(headers),
			},
			Asserts:         ptr(asserts),
			ExtractedValues: ptr(extracted),
		})
	}

	return apiclient.Monitor{
		Name:                  plan.Name.ValueString(),
		NotificationChannelId: plan.NotificationChannelId.ValueString(),
		Locations:             ptr(listToStringSlice(ctx, diags, plan.Locations)),
		Tags:                  ptr(listToStringSlice(ctx, diags, plan.Tags)),
		Steps:                 ptr(steps),
		Options: apiclient.MonitorOptions{
			TickEvery:         optModel.TickEvery.ValueInt64(),
			Timeout:           optModel.Timeout.ValueInt64(),
			FailureThreshold:  optModel.FailureThreshold.ValueInt64(),
			FollowRedirects:   optModel.FollowRedirects.ValueBool(),
			LocationThreshold: optModel.LocationThreshold.ValueInt64(),
		},
		// The DTO requires a status value; the server ignores it (real status
		// is tracked separately). Never send message/favicon.
		Status: apiclient.MonitorStatus("SUCCESS"),
	}
}

// monitorToState maps an API monitor onto the resource model. Steps are
// ordered by their exec number so the list matches the configured order.
func monitorToState(ctx context.Context, diags *diag.Diagnostics, monitor *apiclient.Monitor, state *MonitorResourceModel) {
	state.Name = types.StringValue(monitor.Name)
	state.NotificationChannelId = types.StringValue(monitor.NotificationChannelId)
	state.Locations = stringSliceToList(ctx, diags, monitor.Locations)
	state.Tags = stringSliceToList(ctx, diags, monitor.Tags)
	state.Paused = types.BoolValue(monitor.Status == apiclient.MonitorStatus(monitorPausedStatus))

	optObj, d := types.ObjectValueFrom(ctx, monitorOptionsAttrTypes(), monitorOptionsModel{
		TickEvery:         types.Int64Value(monitor.Options.TickEvery),
		Timeout:           types.Int64Value(monitor.Options.Timeout),
		FailureThreshold:  types.Int64Value(monitor.Options.FailureThreshold),
		FollowRedirects:   types.BoolValue(monitor.Options.FollowRedirects),
		LocationThreshold: types.Int64Value(monitor.Options.LocationThreshold),
	})
	diags.Append(d...)
	state.Options = optObj

	apiSteps := []apiclient.MonitorTestStep{}
	if monitor.Steps != nil {
		apiSteps = append(apiSteps, *monitor.Steps...)
	}
	sort.SliceStable(apiSteps, func(i, j int) bool {
		return apiSteps[i].ExecNumber < apiSteps[j].ExecNumber
	})

	stepModels := make([]monitorStepModel, 0, len(apiSteps))
	for _, s := range apiSteps {
		headerModels := []monitorHeaderModel{}
		if s.Request.Header != nil {
			for _, h := range *s.Request.Header {
				headerModels = append(headerModels, monitorHeaderModel{
					Key:   types.StringValue(h.Key),
					Value: types.StringValue(h.Value),
				})
			}
		}
		headerList, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: monitorHeaderAttrTypes()}, headerModels)
		diags.Append(d...)

		requestObj, d := types.ObjectValueFrom(ctx, monitorRequestAttrTypes(), monitorRequestModel{
			Url:    types.StringValue(s.Request.Url),
			Method: types.StringValue(deref(s.Request.Method, "GET")),
			Body:   types.StringValue(deref(s.Request.Body, "")),
			Header: headerList,
		})
		diags.Append(d...)

		assertModels := []monitorAssertModel{}
		if s.Asserts != nil {
			for _, a := range *s.Asserts {
				assertModels = append(assertModels, monitorAssertModel{
					Type:     types.StringValue(string(a.Type)),
					Property: types.StringValue(deref(a.Property, "")),
					Operator: types.StringValue(string(a.Operator)),
					Expected: types.StringValue(a.Expected),
				})
			}
		}
		assertList, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: monitorAssertAttrTypes()}, assertModels)
		diags.Append(d...)

		extractedModels := []monitorExtractedValueModel{}
		if s.ExtractedValues != nil {
			for _, e := range *s.ExtractedValues {
				extractedModels = append(extractedModels, monitorExtractedValueModel{
					Name:     types.StringValue(e.Name),
					Type:     types.StringValue(string(e.Type)),
					Location: types.StringValue(e.Location),
				})
			}
		}
		extractedList, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: monitorExtractedValueAttrTypes()}, extractedModels)
		diags.Append(d...)

		stepModels = append(stepModels, monitorStepModel{
			Name:            types.StringValue(deref(s.Name, "")),
			Type:            types.StringValue(string(s.Type)),
			Request:         requestObj,
			Asserts:         assertList,
			ExtractedValues: extractedList,
		})
	}

	stepList, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: monitorStepAttrTypes()}, stepModels)
	diags.Append(d...)
	state.Steps = stepList
}

func (r *MonitorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan MonitorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := buildMonitorBody(ctx, &resp.Diagnostics, &plan)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.PostV1MonitorsWithResponse(ctx, body)
	if apiCallFailed(&resp.Diagnostics, "Could not create monitor", err) {
		return
	}
	if !checkStatus(&resp.Diagnostics, "Could not create monitor", res.HTTPResponse, res.Body) {
		return
	}
	if res.JSON200 == nil || res.JSON200.Id == nil {
		resp.Diagnostics.AddError("Could not create monitor", "API response contained no monitor id")
		return
	}

	plan.Id = types.StringValue(*res.JSON200.Id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.Paused.ValueBool() {
		pauseRes, err := r.client.PutV1MonitorsMonitorIdPauseWithResponse(ctx, plan.Id.ValueString())
		if apiCallFailed(&resp.Diagnostics, "Could not pause monitor after create", err) {
			return
		}
		checkStatus(&resp.Diagnostics, "Could not pause monitor after create", pauseRes.HTTPResponse, pauseRes.Body)
	}
}

func (r *MonitorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state MonitorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.GetV1MonitorsMonitorIdWithResponse(ctx, state.Id.ValueString())
	if apiCallFailed(&resp.Diagnostics, "Could not read monitor", err) {
		return
	}
	if isNotFound(res.HTTPResponse) {
		resp.State.RemoveResource(ctx)
		return
	}
	if !checkStatus(&resp.Diagnostics, "Could not read monitor", res.HTTPResponse, res.Body) {
		return
	}
	if res.JSON200 == nil {
		resp.Diagnostics.AddError("Could not read monitor", "empty response body")
		return
	}

	monitorToState(ctx, &resp.Diagnostics, res.JSON200, &state)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *MonitorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state MonitorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := buildMonitorBody(ctx, &resp.Diagnostics, &plan)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.PutV1MonitorsMonitorIdWithResponse(ctx, plan.Id.ValueString(), body)
	if apiCallFailed(&resp.Diagnostics, "Could not update monitor", err) {
		return
	}
	if !checkStatus(&resp.Diagnostics, "Could not update monitor", res.HTTPResponse, res.Body) {
		return
	}

	// Pause state is managed through dedicated endpoints; only act on change.
	if !plan.Paused.Equal(state.Paused) {
		if plan.Paused.ValueBool() {
			pauseRes, err := r.client.PutV1MonitorsMonitorIdPauseWithResponse(ctx, plan.Id.ValueString())
			if apiCallFailed(&resp.Diagnostics, "Could not pause monitor", err) {
				return
			}
			if !checkStatus(&resp.Diagnostics, "Could not pause monitor", pauseRes.HTTPResponse, pauseRes.Body) {
				return
			}
		} else {
			resumeRes, err := r.client.PutV1MonitorsMonitorIdResumeWithResponse(ctx, plan.Id.ValueString())
			if apiCallFailed(&resp.Diagnostics, "Could not resume monitor", err) {
				return
			}
			if !checkStatus(&resp.Diagnostics, "Could not resume monitor", resumeRes.HTTPResponse, resumeRes.Body) {
				return
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *MonitorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state MonitorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.DeleteV1MonitorsMonitorIdWithResponse(ctx, state.Id.ValueString())
	if apiCallFailed(&resp.Diagnostics, "Could not delete monitor", err) {
		return
	}
	if isNotFound(res.HTTPResponse) {
		return // already gone
	}
	checkStatus(&resp.Diagnostics, "Could not delete monitor", res.HTTPResponse, res.Body)
}

func (r *MonitorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
