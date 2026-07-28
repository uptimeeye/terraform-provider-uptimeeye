package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"gitlab.com/uptimeeye/terraform-provider-uptimeeye/internal/apiclient"
)

var (
	_ resource.Resource                = &VariableResource{}
	_ resource.ResourceWithConfigure   = &VariableResource{}
	_ resource.ResourceWithImportState = &VariableResource{}
)

func NewVariableResource() resource.Resource {
	return &VariableResource{}
}

type VariableResource struct {
	client *apiclient.ClientWithResponses
}

type VariableResourceModel struct {
	Id     types.String `tfsdk:"id"`
	Key    types.String `tfsdk:"key"`
	Value  types.String `tfsdk:"value"`
	Secure types.Bool   `tfsdk:"secure"`
}

func (r *VariableResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_variable"
}

func (r *VariableResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A variable usable in monitor steps (e.g. `{{MY_TOKEN}}`). " +
			"Secure variables are write-only on the API: their value is stored in the Terraform state " +
			"but never returned by UptimeEye, so drift on the value itself cannot be detected.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Variable identifier.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"key": schema.StringAttribute{
				MarkdownDescription: "Variable name as referenced in monitor steps.",
				Required:            true,
			},
			"value": schema.StringAttribute{
				MarkdownDescription: "Variable value. Always stored encrypted at rest by UptimeEye; additionally marked sensitive in Terraform output.",
				Required:            true,
				Sensitive:           true,
			},
			"secure": schema.BoolAttribute{
				MarkdownDescription: "Secure variables are masked in the UI and never returned by the API. Changing this forces a new variable.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *VariableResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *VariableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan VariableResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.PostV1VariablesWithResponse(ctx, apiclient.PostV1VariablesJSONRequestBody{
		Key:    plan.Key.ValueString(),
		Value:  plan.Value.ValueString(),
		Secure: plan.Secure.ValueBool(),
	})
	if apiCallFailed(&resp.Diagnostics, "Could not create variable", err) {
		return
	}
	if !checkStatus(&resp.Diagnostics, "Could not create variable", res.HTTPResponse, res.Body) {
		return
	}
	if res.JSON200 == nil || res.JSON200.Id == nil {
		resp.Diagnostics.AddError("Could not create variable", "API response contained no variable id")
		return
	}

	plan.Id = types.StringValue(*res.JSON200.Id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *VariableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state VariableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.GetV1VariablesVariableIdWithResponse(ctx, state.Id.ValueString())
	if apiCallFailed(&resp.Diagnostics, "Could not read variable", err) {
		return
	}
	if isNotFound(res.HTTPResponse) {
		resp.State.RemoveResource(ctx)
		return
	}
	if !checkStatus(&resp.Diagnostics, "Could not read variable", res.HTTPResponse, res.Body) {
		return
	}
	if res.JSON200 == nil {
		resp.Diagnostics.AddError("Could not read variable", "empty response body")
		return
	}
	variable := res.JSON200

	state.Key = types.StringValue(variable.Key)
	state.Secure = types.BoolValue(variable.Secure)
	// The API never returns the value of secure variables — keep the state
	// value in that case instead of producing a false diff.
	if !variable.Secure && variable.Value != nil {
		state.Value = types.StringValue(*variable.Value)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *VariableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan VariableResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.PutV1VariablesVariableIdWithResponse(ctx, plan.Id.ValueString(), apiclient.PutV1VariablesVariableIdJSONRequestBody{
		Key:    ptr(plan.Key.ValueString()),
		Value:  ptr(plan.Value.ValueString()),
		Secure: ptr(plan.Secure.ValueBool()),
	})
	if apiCallFailed(&resp.Diagnostics, "Could not update variable", err) {
		return
	}
	if !checkStatus(&resp.Diagnostics, "Could not update variable", res.HTTPResponse, res.Body) {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *VariableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state VariableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.DeleteV1VariablesVariableIdWithResponse(ctx, state.Id.ValueString())
	if apiCallFailed(&resp.Diagnostics, "Could not delete variable", err) {
		return
	}
	if isNotFound(res.HTTPResponse) {
		return
	}
	checkStatus(&resp.Diagnostics, "Could not delete variable", res.HTTPResponse, res.Body)
}

func (r *VariableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	// Secure variables cannot restore their value from the API; the user must
	// set it in config after import (the first plan will show a value change).
}
