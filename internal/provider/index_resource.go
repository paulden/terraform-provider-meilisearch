package provider

import (
	"context"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
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
}

// Metadata returns the resource type name.
func (r *indexResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_index"
}

// Schema defines the schema for the resource.
func (r *indexResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Meilisearch Index.",
		Attributes: map[string]schema.Attribute{
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
			"ranking_rules": schema.ListAttribute{
				Description: "List of ranking rules applied to search results. Default is [\"words\", \"typo\", \"proximity\", \"attribute\", \"sort\", \"exactness\"].",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
			},
			"distinct_attribute": schema.StringAttribute{
				Description: "Search returns documents with distinct (different) values of the given field. Only one document per value will be returned.",
				Optional:    true,
				Computed:    true,
			},
			"searchable_attributes": schema.SetAttribute{
				Description: "Set of attributes to search in. If empty or not set, all attributes are searchable. Default is [\"*\"].",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
			},
			"displayed_attributes": schema.SetAttribute{
				Description: "Set of attributes to display in search results. If empty or not set, all attributes are displayed. Default is [\"*\"].",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
			},
			"filterable_attributes": schema.SetAttribute{
				Description: "Set of attributes that can be used as filters in search queries.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
			},
			"sortable_attributes": schema.SetAttribute{
				Description: "Set of attributes that can be used to sort search results.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
			},
			"stop_words": schema.SetAttribute{
				Description: "Set of words that will be ignored in search queries.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
			},
			"synonyms": schema.MapAttribute{
				Description: "Map of synonyms where the key is a word and the value is a list of synonyms for that word.",
				Optional:    true,
				Computed:    true,
				ElementType: types.ListType{ElemType: types.StringType},
			},
		},
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

	if waitTask.Status == "succeeded" {
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

		// Apply settings if any are configured
		settings := r.buildSettingsFromPlan(ctx, &plan, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}

		if settings != nil {
			settingsTask, err := r.client.Index(index.UID).UpdateSettings(settings)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error updating index settings",
					"Could not update settings, unexpected error: "+err.Error(),
				)
				return
			}

			// Wait for settings update to complete
			waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			waitSettingsTask, err := r.client.WaitForTaskWithContext(waitCtx, settingsTask.TaskUID, 100*time.Millisecond)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error waiting for settings update task",
					"unexpected error: "+err.Error(),
				)
				return
			}

			if waitSettingsTask.Status != "succeeded" {
				resp.Diagnostics.AddError(
					"Settings update task failed",
					"Status: "+string(waitSettingsTask.Status),
				)
				return
			}
		}

		// Always read back the settings to populate all computed values
		r.readSettings(ctx, index.UID, &plan, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
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

	// Overwrite items with refreshed state
	indexState := indexResourceModel{
		UID:        types.StringValue(index.UID),
		PrimaryKey: types.StringValue(index.PrimaryKey),
		CreatedAt:  types.StringValue(index.CreatedAt.Format(time.RFC3339)),
		UpdatedAt:  types.StringValue(index.UpdatedAt.Format(time.RFC3339)),
	}

	state = indexState

	// Read settings
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
	// Get plan values
	var plan indexResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current state
	var state indexResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build settings from plan
	settings := r.buildSettingsFromPlan(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if settings != nil {
		// Update settings
		settingsTask, err := r.client.Index(state.UID.ValueString()).UpdateSettings(settings)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error updating index settings",
				"Could not update settings, unexpected error: "+err.Error(),
			)
			return
		}

		// Wait for settings update to complete
		waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		waitSettingsTask, err := r.client.WaitForTaskWithContext(waitCtx, settingsTask.TaskUID, 100*time.Millisecond)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error waiting for settings update task",
				"unexpected error: "+err.Error(),
			)
			return
		}

		if waitSettingsTask.Status != "succeeded" {
			resp.Diagnostics.AddError(
				"Settings update task failed",
				"Status: "+string(waitSettingsTask.Status),
			)
			return
		}
	}

	// Read the index to get updated metadata
	index, err := r.client.GetIndex(state.UID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading index after update",
			"Could not read index: "+err.Error(),
		)
		return
	}

	// Update state with index metadata
	plan.UID = types.StringValue(index.UID)
	plan.PrimaryKey = types.StringValue(index.PrimaryKey)
	plan.CreatedAt = types.StringValue(index.CreatedAt.Format(time.RFC3339))
	plan.UpdatedAt = types.StringValue(index.UpdatedAt.Format(time.RFC3339))

	// Read back settings to ensure state is in sync
	r.readSettings(ctx, index.UID, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = types.StringValue("placeholder")

	// Set updated state
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

// buildSettingsFromPlan constructs a Meilisearch Settings object from the Terraform plan
func (r *indexResource) buildSettingsFromPlan(ctx context.Context, plan *indexResourceModel, diags *diag.Diagnostics) *meilisearch.Settings {
	var hasSettings bool
	settings := &meilisearch.Settings{}

	// RankingRules
	if !plan.RankingRules.IsNull() && !plan.RankingRules.IsUnknown() {
		var rankingRules []string
		diags.Append(plan.RankingRules.ElementsAs(ctx, &rankingRules, false)...)
		if diags.HasError() {
			return nil
		}
		settings.RankingRules = rankingRules
		hasSettings = true
	}

	// DistinctAttribute
	if !plan.DistinctAttribute.IsNull() && !plan.DistinctAttribute.IsUnknown() {
		distinctAttr := plan.DistinctAttribute.ValueString()
		settings.DistinctAttribute = &distinctAttr
		hasSettings = true
	}

	// SearchableAttributes
	if !plan.SearchableAttributes.IsNull() && !plan.SearchableAttributes.IsUnknown() {
		var searchableAttrs []string
		diags.Append(plan.SearchableAttributes.ElementsAs(ctx, &searchableAttrs, false)...)
		if diags.HasError() {
			return nil
		}
		settings.SearchableAttributes = searchableAttrs
		hasSettings = true
	}

	// DisplayedAttributes
	if !plan.DisplayedAttributes.IsNull() && !plan.DisplayedAttributes.IsUnknown() {
		var displayedAttrs []string
		diags.Append(plan.DisplayedAttributes.ElementsAs(ctx, &displayedAttrs, false)...)
		if diags.HasError() {
			return nil
		}
		settings.DisplayedAttributes = displayedAttrs
		hasSettings = true
	}

	// FilterableAttributes
	if !plan.FilterableAttributes.IsNull() && !plan.FilterableAttributes.IsUnknown() {
		var filterableAttrs []string
		diags.Append(plan.FilterableAttributes.ElementsAs(ctx, &filterableAttrs, false)...)
		if diags.HasError() {
			return nil
		}
		settings.FilterableAttributes = filterableAttrs
		hasSettings = true
	}

	// SortableAttributes
	if !plan.SortableAttributes.IsNull() && !plan.SortableAttributes.IsUnknown() {
		var sortableAttrs []string
		diags.Append(plan.SortableAttributes.ElementsAs(ctx, &sortableAttrs, false)...)
		if diags.HasError() {
			return nil
		}
		settings.SortableAttributes = sortableAttrs
		hasSettings = true
	}

	// StopWords
	if !plan.StopWords.IsNull() && !plan.StopWords.IsUnknown() {
		var stopWords []string
		diags.Append(plan.StopWords.ElementsAs(ctx, &stopWords, false)...)
		if diags.HasError() {
			return nil
		}
		settings.StopWords = stopWords
		hasSettings = true
	}

	// Synonyms
	if !plan.Synonyms.IsNull() && !plan.Synonyms.IsUnknown() {
		var synonymsMap map[string][]string
		diags.Append(plan.Synonyms.ElementsAs(ctx, &synonymsMap, false)...)
		if diags.HasError() {
			return nil
		}
		settings.Synonyms = synonymsMap
		hasSettings = true
	}

	if hasSettings {
		return settings
	}
	return nil
}

// readSettings retrieves settings from Meilisearch and populates the plan
func (r *indexResource) readSettings(ctx context.Context, indexUID string, plan *indexResourceModel, diags *diag.Diagnostics) {
	settings, err := r.client.Index(indexUID).GetSettings()
	if err != nil {
		diags.AddError(
			"Error reading index settings",
			"Could not read settings for index "+indexUID+": "+err.Error(),
		)
		return
	}

	// RankingRules
	if settings.RankingRules != nil {
		rankingRulesList, d := types.ListValueFrom(ctx, types.StringType, settings.RankingRules)
		diags.Append(d...)
		plan.RankingRules = rankingRulesList
	} else {
		plan.RankingRules = types.ListNull(types.StringType)
	}

	// DistinctAttribute
	if settings.DistinctAttribute != nil {
		plan.DistinctAttribute = types.StringValue(*settings.DistinctAttribute)
	} else {
		plan.DistinctAttribute = types.StringNull()
	}

	// SearchableAttributes
	if settings.SearchableAttributes != nil {
		searchableAttrsSet, d := types.SetValueFrom(ctx, types.StringType, settings.SearchableAttributes)
		diags.Append(d...)
		plan.SearchableAttributes = searchableAttrsSet
	} else {
		plan.SearchableAttributes = types.SetNull(types.StringType)
	}

	// DisplayedAttributes
	if settings.DisplayedAttributes != nil {
		displayedAttrsSet, d := types.SetValueFrom(ctx, types.StringType, settings.DisplayedAttributes)
		diags.Append(d...)
		plan.DisplayedAttributes = displayedAttrsSet
	} else {
		plan.DisplayedAttributes = types.SetNull(types.StringType)
	}

	// FilterableAttributes
	if settings.FilterableAttributes != nil {
		filterableAttrsSet, d := types.SetValueFrom(ctx, types.StringType, settings.FilterableAttributes)
		diags.Append(d...)
		plan.FilterableAttributes = filterableAttrsSet
	} else {
		plan.FilterableAttributes = types.SetNull(types.StringType)
	}

	// SortableAttributes
	if settings.SortableAttributes != nil {
		sortableAttrsSet, d := types.SetValueFrom(ctx, types.StringType, settings.SortableAttributes)
		diags.Append(d...)
		plan.SortableAttributes = sortableAttrsSet
	} else {
		plan.SortableAttributes = types.SetNull(types.StringType)
	}

	// StopWords
	if settings.StopWords != nil {
		stopWordsSet, d := types.SetValueFrom(ctx, types.StringType, settings.StopWords)
		diags.Append(d...)
		plan.StopWords = stopWordsSet
	} else {
		plan.StopWords = types.SetNull(types.StringType)
	}

	// Synonyms
	if settings.Synonyms != nil {
		synonymsMap, d := types.MapValueFrom(ctx, types.ListType{ElemType: types.StringType}, settings.Synonyms)
		diags.Append(d...)
		plan.Synonyms = synonymsMap
	} else {
		plan.Synonyms = types.MapNull(types.ListType{ElemType: types.StringType})
	}
}
