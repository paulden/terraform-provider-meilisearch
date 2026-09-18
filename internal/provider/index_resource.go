package provider

import (
	"context"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/meilisearch/meilisearch-go"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &indexResource{}
	_ resource.ResourceWithConfigure   = &indexResource{}
	_ resource.ResourceWithImportState = &indexResource{}
)

// NewIndexResource is a helper function to simplify the provider implementation.
func NewIndexResource() resource.Resource {
	return &indexResource{}
}

// indexResource is the resource implementation.
type indexResource struct {
	client meilisearch.ServiceManager
}

type indexResourceModel struct {
	UID                  types.String `tfsdk:"uid"`
	PrimaryKey           types.String `tfsdk:"primary_key"`
	CreatedAt            types.String `tfsdk:"created_at"`
	UpdatedAt            types.String `tfsdk:"updated_at"`
	ID                   types.String `tfsdk:"id"`
	RankingRules         types.List   `tfsdk:"ranking_rules"`
	DistinctAttribute    types.String `tfsdk:"distinct_attribute"`
	SearchableAttributes types.Set    `tfsdk:"searchable_attributes"`
	DisplayedAttributes  types.Set    `tfsdk:"displayed_attributes"`
	FilterableAttributes types.Set    `tfsdk:"filterable_attributes"`
	SortableAttributes   types.Set    `tfsdk:"sortable_attributes"`
	StopWords            types.Set    `tfsdk:"stop_words"`
	Synonyms             types.Map    `tfsdk:"synonyms"`
	Dictionary           types.Set    `tfsdk:"dictionary"`
	SearchCutoffMs       types.Int64  `tfsdk:"search_cutoff_ms"`
	ProximityPrecision   types.String `tfsdk:"proximity_precision"`
	SeparatorTokens      types.Set    `tfsdk:"separator_tokens"`
	NonSeparatorTokens   types.Set    `tfsdk:"non_separator_tokens"`
	TypoTolerance        types.Object `tfsdk:"typo_tolerance"`
	Pagination           types.Object `tfsdk:"pagination"`
	Faceting             types.Object `tfsdk:"faceting"`
	LocalizedAttributes  types.List   `tfsdk:"localized_attributes"`
	Embedders            types.Map    `tfsdk:"embedders"`
}

// Metadata returns the resource type name.
func (r *indexResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_index"
}

// Schema defines the schema for the resource.
func (r *indexResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attributes := map[string]schema.Attribute{
		"uid": schema.StringAttribute{
			Description: "Unique identifier of the index.",
			Required:    true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"primary_key": schema.StringAttribute{
			Description: "Primary key of the index (`null` if not specified and if no documents have been added yet, see [official documentation](https://www.meilisearch.com/docs/learn/core_concepts/primary_key#meilisearch-guesses-your-primary-key) for more details).",
			Required:    true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"created_at": schema.StringAttribute{
			Description: "Date and time when the key was created (RFC3339)",
			Computed:    true,
		},
		"updated_at": schema.StringAttribute{
			Description: "Date and time when the key was last updated (RFC3339)",
			Computed:    true,
		},
		"id": schema.StringAttribute{
			Description: "Placeholder identifier attribute.",
			Computed:    true,
		},
	}

	for name, attribute := range indexSettingsSchemaAttributes() {
		attributes[name] = attribute
	}

	resp.Schema = schema.Schema{
		Description: "Manages a Meilisearch Index and its settings.",
		Attributes:  attributes,
	}
}

// Configure adds the provider configured client to the resource.
func (r *indexResource) Configure(ctx context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	var ok bool

	r.client, ok = req.ProviderData.(meilisearch.ServiceManager)

	if !ok {
		tflog.Error(ctx, "Type assertion failed when adding configured client to the resource")
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *indexResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan indexResourceModel

	diags := req.Plan.Get(ctx, &plan)

	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createIndexConfig := meilisearch.IndexConfig{
		Uid:        plan.UID.ValueString(),
		PrimaryKey: plan.PrimaryKey.ValueString(),
	}
	task, err := r.client.CreateIndex(&createIndexConfig)

	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating index",
			"Could not create index, unexpected error: "+err.Error(),
		)
		return
	}

	waitTask, err := r.client.WaitForTask(task.TaskUID, time.Duration(5)*time.Second)

	if err != nil {
		resp.Diagnostics.AddError(
			"Error fetching index creation task",
			"unexpected error: "+err.Error(),
		)
		return
	}

	if waitTask.Status != meilisearch.TaskStatusSucceeded {
		resp.Diagnostics.AddError(
			"Error creating index",
			"Index creation task finished with status "+string(waitTask.Status)+": "+waitTask.Error.Message,
		)
		return
	}

	index, err := r.client.GetIndex(createIndexConfig.Uid)

	if err != nil {
		resp.Diagnostics.AddError(
			"Error fetching index data",
			"unexpected error: "+err.Error(),
		)
		return
	}

	plan.UID = types.StringValue(index.UID)
	plan.PrimaryKey = types.StringValue(index.PrimaryKey)
	plan.CreatedAt = types.StringValue(index.CreatedAt.Format(time.RFC3339))
	plan.UpdatedAt = types.StringValue(index.UpdatedAt.Format(time.RFC3339))

	r.applyPlannedSettings(ctx, index.UID, &plan, nil, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = types.StringValue("placeholder")

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *indexResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state indexResourceModel

	diags := req.State.Get(ctx, &state)

	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get refreshed index value from Meilisearch
	index, err := r.client.GetIndex(state.UID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "index_not_found,") {
			resp.State.RemoveResource(ctx)
			return
		} else {
			resp.Diagnostics.AddError(
				"Error Reading Meilisearch Index",
				"Could not read Meilisearch index ID "+state.UID.ValueString()+": "+err.Error(),
			)
			return
		}
	}

	// Overwrite items with refreshed state. The settings attributes are updated
	// in place by readSettings rather than replaced wholesale: it uses the prior
	// state to tell which settings this resource manages, and a fresh model would
	// report every one of them as unmanaged.
	state.UID = types.StringValue(index.UID)
	state.PrimaryKey = types.StringValue(index.PrimaryKey)
	state.CreatedAt = types.StringValue(index.CreatedAt.Format(time.RFC3339))
	state.UpdatedAt = types.StringValue(index.UpdatedAt.Format(time.RFC3339))

	r.readSettings(ctx, index.UID, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	state.ID = types.StringValue("placeholder")

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *indexResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan indexResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state indexResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.applyPlannedSettings(ctx, state.UID.ValueString(), &plan, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	index, err := r.client.GetIndex(state.UID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading index after update",
			"Could not read index: "+err.Error(),
		)
		return
	}

	plan.UID = types.StringValue(index.UID)
	plan.PrimaryKey = types.StringValue(index.PrimaryKey)
	plan.CreatedAt = types.StringValue(index.CreatedAt.Format(time.RFC3339))
	plan.UpdatedAt = types.StringValue(index.UpdatedAt.Format(time.RFC3339))

	plan.ID = types.StringValue("placeholder")

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *indexResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state indexResourceModel

	diags := req.State.Get(ctx, &state)

	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete existing index
	_, err := r.client.DeleteIndex(state.UID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Meilisearch Index",
			"Could not delete index, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *indexResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import UID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("uid"), req, resp)
}
