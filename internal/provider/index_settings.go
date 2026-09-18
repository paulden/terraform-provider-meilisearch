package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/meilisearch/meilisearch-go"
)

// settingsTaskTimeout bounds how long we wait for a settings task to reach a
// terminal state. Embedder changes make Meilisearch re-index every document, so
// this is deliberately far more generous than a plain settings write needs.
const settingsTaskTimeout = 10 * time.Minute

// settingOp is a single per-setting API call, used for the changes the bulk
// settings endpoint cannot express (see indexSettingsOps).
type settingOp struct {
	attribute string
	run       func() (*meilisearch.TaskInfo, error)
}

// waitForSettingsTask blocks until a settings task finishes, turning a failed
// task into an error instead of letting the caller continue as if it succeeded.
func (r *indexResource) waitForSettingsTask(ctx context.Context, task *meilisearch.TaskInfo, what string) error {
	waitCtx, cancel := context.WithTimeout(ctx, settingsTaskTimeout)
	defer cancel()

	waitTask, err := r.client.WaitForTaskWithContext(waitCtx, task.TaskUID, 100*time.Millisecond)
	if err != nil {
		return fmt.Errorf("error waiting for %s task: %w", what, err)
	}

	if waitTask.Status != meilisearch.TaskStatusSucceeded {
		if waitTask.Error.Code != "" {
			return fmt.Errorf("%s task %s: %s (%s)", what, waitTask.Status, waitTask.Error.Message, waitTask.Error.Code)
		}
		return fmt.Errorf("%s task finished with status %q", what, waitTask.Status)
	}

	return nil
}

// applySettings sends the given settings to Meilisearch and waits for the
// resulting task to complete.
func (r *indexResource) applySettings(ctx context.Context, indexUID string, settings *meilisearch.Settings) error {
	task, err := r.client.Index(indexUID).UpdateSettingsWithContext(ctx, settings)
	if err != nil {
		return fmt.Errorf("could not update settings: %w", err)
	}

	return r.waitForSettingsTask(ctx, task, "settings update")
}

// applyPlannedSettings reconciles an index's settings with the plan: it writes
// everything the plan sets in one bulk call, then issues the per-setting calls
// needed to clear what the plan removed or emptied.
//
// state is the prior state, or nil on Create.
func (r *indexResource) applyPlannedSettings(ctx context.Context, indexUID string, plan *indexResourceModel, state *indexResourceModel, diags *diag.Diagnostics) {
	settings := r.buildSettingsFromPlan(ctx, plan, diags)
	if diags.HasError() {
		return
	}

	if settings != nil {
		if err := r.applySettings(ctx, indexUID, settings); err != nil {
			diags.AddError("Error updating index settings", err.Error())
			return
		}
	}

	if ops := r.indexSettingsOps(plan, state, indexUID); len(ops) > 0 {
		if err := r.applySettingsOps(ctx, ops); err != nil {
			diags.AddError("Error resetting index settings", err.Error())
		}
	}
}

// applySettingsOps runs the per-setting calls produced by indexSettingsOps,
// waiting for each task in turn.
func (r *indexResource) applySettingsOps(ctx context.Context, ops []settingOp) error {
	for _, op := range ops {
		task, err := op.run()
		if err != nil {
			return fmt.Errorf("could not reset %s: %w", op.attribute, err)
		}
		if err := r.waitForSettingsTask(ctx, task, "reset of "+op.attribute); err != nil {
			return err
		}
	}

	return nil
}

var minWordSizeForTyposAttrTypes = map[string]attr.Type{
	"one_typo":  types.Int64Type,
	"two_typos": types.Int64Type,
}

var typoToleranceAttrTypes = map[string]attr.Type{
	"enabled":                 types.BoolType,
	"min_word_size_for_typos": types.ObjectType{AttrTypes: minWordSizeForTyposAttrTypes},
	"disable_on_words":        types.SetType{ElemType: types.StringType},
	"disable_on_attributes":   types.SetType{ElemType: types.StringType},
}

var paginationAttrTypes = map[string]attr.Type{
	"max_total_hits": types.Int64Type,
}

var facetingAttrTypes = map[string]attr.Type{
	"max_values_per_facet": types.Int64Type,
	"sort_facet_values_by": types.MapType{ElemType: types.StringType},
}

var localizedAttributeAttrTypes = map[string]attr.Type{
	"locales":            types.SetType{ElemType: types.StringType},
	"attribute_patterns": types.SetType{ElemType: types.StringType},
}

var embedderAttrTypes = map[string]attr.Type{
	"source":            types.StringType,
	"model":             types.StringType,
	"api_key":           types.StringType,
	"document_template": types.StringType,
	"dimensions":        types.Int64Type,
	"url":               types.StringType,
	"revision":          types.StringType,
	"request":           types.StringType,
	"response":          types.StringType,
	"headers":           types.MapType{ElemType: types.StringType},
}

// embedderSources are the embedder sources Meilisearch accepts. Note that the
// fields each source accepts differ: `document_template`, for instance, is
// rejected outright for `userProvided`.
var embedderSources = []string{"openAi", "huggingFace", "userProvided", "rest", "ollama"}

// indexSettingsSchemaAttributes returns the schema attributes covering the
// Meilisearch index settings API, to be merged into the index resource schema.
//
// These attributes are Optional but deliberately NOT Computed. Marking them
// Computed would make Terraform fall back to the prior state whenever the
// attribute is absent from the configuration, so removing a setting from the
// configuration would produce no diff and the value would be stuck on the
// server forever. As plain Optional attributes, removing one produces a diff
// that Update turns into an explicit reset.
func indexSettingsSchemaAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"ranking_rules": schema.ListAttribute{
			Description: "Ordered list of ranking rules applied to search results. Defaults to [\"words\", \"typo\", \"proximity\", \"attribute\", \"sort\", \"exactness\"] when not managed here.",
			Optional:    true,
			ElementType: types.StringType,
		},
		"distinct_attribute": schema.StringAttribute{
			Description: "Search returns documents with distinct (different) values of the given field. Only one document per value will be returned.",
			Optional:    true,
		},
		"searchable_attributes": schema.SetAttribute{
			Description: "Set of attributes to search in. Defaults to [\"*\"] (all attributes) when not managed here.",
			Optional:    true,
			ElementType: types.StringType,
		},
		"displayed_attributes": schema.SetAttribute{
			Description: "Set of attributes to display in search results. Defaults to [\"*\"] (all attributes) when not managed here.",
			Optional:    true,
			ElementType: types.StringType,
		},
		"filterable_attributes": schema.SetAttribute{
			Description: "Set of attributes that can be used as filters in search queries.",
			Optional:    true,
			ElementType: types.StringType,
		},
		"sortable_attributes": schema.SetAttribute{
			Description: "Set of attributes that can be used to sort search results.",
			Optional:    true,
			ElementType: types.StringType,
		},
		"stop_words": schema.SetAttribute{
			Description: "Set of words that will be ignored in search queries.",
			Optional:    true,
			ElementType: types.StringType,
		},
		"synonyms": schema.MapAttribute{
			Description: "Map of synonyms where the key is a word and the value is a list of synonyms for that word.",
			Optional:    true,
			ElementType: types.ListType{ElemType: types.StringType},
		},
		"dictionary": schema.SetAttribute{
			Description: "Set of words considered as a single term by the tokenizer.",
			Optional:    true,
			ElementType: types.StringType,
		},
		"search_cutoff_ms": schema.Int64Attribute{
			Description: "Maximum duration, in milliseconds, of a search query.",
			Optional:    true,
		},
		"proximity_precision": schema.StringAttribute{
			Description: "Precision level when calculating the proximity ranking rule. One of \"byWord\" or \"byAttribute\".",
			Optional:    true,
			Validators: []validator.String{
				stringvalidator.OneOf("byWord", "byAttribute"),
			},
		},
		"separator_tokens": schema.SetAttribute{
			Description: "Set of characters or words treated as word separators by the tokenizer.",
			Optional:    true,
			ElementType: types.StringType,
		},
		"non_separator_tokens": schema.SetAttribute{
			Description: "Set of characters or words normally treated as separators that should instead be treated as normal characters.",
			Optional:    true,
			ElementType: types.StringType,
		},
		"typo_tolerance": schema.SingleNestedAttribute{
			Description: "Controls the typo tolerance feature. Requires a Meilisearch release that understands `disableOnNumbers`: the Go client always serialises that field, so older servers reject the whole settings payload.",
			Optional:    true,
			Attributes: map[string]schema.Attribute{
				"enabled": schema.BoolAttribute{
					Description: "Whether typo tolerance is enabled.",
					Optional:    true,
				},
				"min_word_size_for_typos": schema.SingleNestedAttribute{
					Description: "Minimum word size for accepting typos.",
					Optional:    true,
					Attributes: map[string]schema.Attribute{
						"one_typo": schema.Int64Attribute{
							Description: "Minimum word size to accept one typo.",
							Optional:    true,
						},
						"two_typos": schema.Int64Attribute{
							Description: "Minimum word size to accept two typos.",
							Optional:    true,
						},
					},
				},
				"disable_on_words": schema.SetAttribute{
					Description: "Set of words for which typo tolerance is disabled.",
					Optional:    true,
					ElementType: types.StringType,
				},
				"disable_on_attributes": schema.SetAttribute{
					Description: "Set of attributes for which typo tolerance is disabled.",
					Optional:    true,
					ElementType: types.StringType,
				},
			},
		},
		"pagination": schema.SingleNestedAttribute{
			Description: "Controls pagination settings.",
			Optional:    true,
			Attributes: map[string]schema.Attribute{
				"max_total_hits": schema.Int64Attribute{
					Description: "Maximum number of search results that can be returned.",
					Optional:    true,
				},
			},
		},
		"faceting": schema.SingleNestedAttribute{
			Description: "Controls faceting settings.",
			Optional:    true,
			Attributes: map[string]schema.Attribute{
				"max_values_per_facet": schema.Int64Attribute{
					Description: "Maximum number of facet values returned for each facet.",
					Optional:    true,
				},
				"sort_facet_values_by": schema.MapAttribute{
					Description: "Map of facet name (or \"*\") to sort order, \"alpha\" or \"count\".",
					Optional:    true,
					ElementType: types.StringType,
					Validators: []validator.Map{
						mapvalidator.ValueStringsAre(stringvalidator.OneOf("alpha", "count")),
					},
				},
			},
		},
		"localized_attributes": schema.ListNestedAttribute{
			Description: "List of localized attribute rules, associating attribute patterns with locales.",
			Optional:    true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"locales": schema.SetAttribute{
						Description: "Set of locales to apply to the matched attributes.",
						Optional:    true,
						ElementType: types.StringType,
					},
					"attribute_patterns": schema.SetAttribute{
						Description: "Set of attribute name patterns this rule applies to.",
						Optional:    true,
						ElementType: types.StringType,
					},
				},
			},
		},
		"embedders": schema.MapNestedAttribute{
			Description: "Map of embedder name to embedder configuration, used for AI-powered search. Which fields are accepted depends on `source`.",
			Optional:    true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"source": schema.StringAttribute{
						Description: "Embedder source: \"openAi\", \"huggingFace\", \"userProvided\", \"rest\", or \"ollama\".",
						Optional:    true,
						Validators: []validator.String{
							stringvalidator.OneOf(embedderSources...),
						},
					},
					"model": schema.StringAttribute{
						Description: "Model name, for \"openAi\", \"huggingFace\" or \"ollama\" sources.",
						Optional:    true,
					},
					"api_key": schema.StringAttribute{
						Description: "API key, for \"openAi\", \"rest\" or \"ollama\" sources. Meilisearch redacts this on read, so it is never refreshed from the server.",
						Optional:    true,
						Sensitive:   true,
					},
					"document_template": schema.StringAttribute{
						Description: "Template describing the data Meilisearch sends the embedder. Not accepted by the \"userProvided\" source.",
						Optional:    true,
					},
					"dimensions": schema.Int64Attribute{
						Description: "Number of dimensions in the embedding output.",
						Optional:    true,
					},
					"url": schema.StringAttribute{
						Description: "URL for \"openAi\", \"rest\" or \"ollama\" sources.",
						Optional:    true,
					},
					"revision": schema.StringAttribute{
						Description: "Model revision, for the \"huggingFace\" source.",
						Optional:    true,
					},
					"request": schema.StringAttribute{
						Description: "JSON-encoded request body template, for the \"rest\" source.",
						Optional:    true,
					},
					"response": schema.StringAttribute{
						Description: "JSON-encoded response body template, for the \"rest\" source.",
						Optional:    true,
					},
					"headers": schema.MapAttribute{
						Description: "Map of extra HTTP headers, for the \"rest\" source.",
						Optional:    true,
						ElementType: types.StringType,
					},
				},
			},
		},
	}
}

// buildSettingsFromPlan constructs a Meilisearch Settings object from the
// Terraform plan. Only attributes that are set to a non-empty value are
// included: meilisearch.Settings tags every field `omitempty`, so an empty
// collection would silently vanish from the request body rather than clearing
// the setting. Emptying and removing are handled by indexSettingsOps instead.
func (r *indexResource) buildSettingsFromPlan(ctx context.Context, plan *indexResourceModel, diags *diag.Diagnostics) *meilisearch.Settings {
	var hasSettings bool
	settings := &meilisearch.Settings{}

	set := func(populated bool) {
		if populated {
			hasSettings = true
		}
	}

	if isSet(plan.RankingRules) {
		var v []string
		diags.Append(plan.RankingRules.ElementsAs(ctx, &v, false)...)
		settings.RankingRules = v
		set(len(v) > 0)
	}

	if isSet(plan.DistinctAttribute) {
		v := plan.DistinctAttribute.ValueString()
		settings.DistinctAttribute = &v
		set(true)
	}

	if isSet(plan.SearchableAttributes) {
		var v []string
		diags.Append(plan.SearchableAttributes.ElementsAs(ctx, &v, false)...)
		settings.SearchableAttributes = v
		set(len(v) > 0)
	}

	if isSet(plan.DisplayedAttributes) {
		var v []string
		diags.Append(plan.DisplayedAttributes.ElementsAs(ctx, &v, false)...)
		settings.DisplayedAttributes = v
		set(len(v) > 0)
	}

	if isSet(plan.FilterableAttributes) {
		var v []string
		diags.Append(plan.FilterableAttributes.ElementsAs(ctx, &v, false)...)
		settings.FilterableAttributes = v
		set(len(v) > 0)
	}

	if isSet(plan.SortableAttributes) {
		var v []string
		diags.Append(plan.SortableAttributes.ElementsAs(ctx, &v, false)...)
		settings.SortableAttributes = v
		set(len(v) > 0)
	}

	if isSet(plan.StopWords) {
		var v []string
		diags.Append(plan.StopWords.ElementsAs(ctx, &v, false)...)
		settings.StopWords = v
		set(len(v) > 0)
	}

	if isSet(plan.Synonyms) {
		var v map[string][]string
		diags.Append(plan.Synonyms.ElementsAs(ctx, &v, false)...)
		settings.Synonyms = v
		set(len(v) > 0)
	}

	if isSet(plan.Dictionary) {
		var v []string
		diags.Append(plan.Dictionary.ElementsAs(ctx, &v, false)...)
		settings.Dictionary = v
		set(len(v) > 0)
	}

	if isSet(plan.SearchCutoffMs) {
		settings.SearchCutoffMs = plan.SearchCutoffMs.ValueInt64()
		set(true)
	}

	if isSet(plan.ProximityPrecision) {
		settings.ProximityPrecision = meilisearch.ProximityPrecisionType(plan.ProximityPrecision.ValueString())
		set(true)
	}

	if isSet(plan.SeparatorTokens) {
		var v []string
		diags.Append(plan.SeparatorTokens.ElementsAs(ctx, &v, false)...)
		settings.SeparatorTokens = v
		set(len(v) > 0)
	}

	if isSet(plan.NonSeparatorTokens) {
		var v []string
		diags.Append(plan.NonSeparatorTokens.ElementsAs(ctx, &v, false)...)
		settings.NonSeparatorTokens = v
		set(len(v) > 0)
	}

	if isSet(plan.TypoTolerance) {
		settings.TypoTolerance = objectToTypoTolerance(ctx, plan.TypoTolerance, diags)
		set(true)
	}

	if isSet(plan.Pagination) {
		settings.Pagination = objectToPagination(plan.Pagination, diags)
		set(true)
	}

	if isSet(plan.Faceting) {
		settings.Faceting = objectToFaceting(ctx, plan.Faceting, diags)
		set(true)
	}

	if isSet(plan.LocalizedAttributes) {
		la := listToLocalizedAttributes(ctx, plan.LocalizedAttributes, diags)
		settings.LocalizedAttributes = la
		set(len(la) > 0)
	}

	if isSet(plan.Embedders) {
		emb := mapToEmbedders(ctx, plan.Embedders, diags)
		settings.Embedders = emb
		set(len(emb) > 0)
	}

	if diags.HasError() {
		return nil
	}

	if hasSettings {
		return settings
	}

	return nil
}

// indexSettingsOps returns the per-setting API calls needed to express changes
// the bulk settings endpoint cannot: attributes removed from the configuration
// (reset to the Meilisearch default) and attributes explicitly set to an empty
// collection (which `omitempty` would otherwise drop from the request body).
//
// state may be nil, for Create, where nothing can have been removed yet.
func (r *indexResource) indexSettingsOps(plan *indexResourceModel, state *indexResourceModel, indexUID string) []settingOp {
	index := r.client.Index(indexUID)

	var ops []settingOp

	// add records a reset when an attribute was dropped from the configuration,
	// or an explicit empty write when it is present but empty.
	add := func(attribute string, planValue attr.Value, planLen int, stateValue attr.Value,
		reset func() (*meilisearch.TaskInfo, error), empty func() (*meilisearch.TaskInfo, error)) {
		removed := planValue.IsNull() && state != nil && !stateValue.IsNull()
		emptied := !planValue.IsNull() && !planValue.IsUnknown() && planLen == 0

		switch {
		case removed:
			ops = append(ops, settingOp{attribute: attribute, run: reset})
		case emptied && empty != nil:
			ops = append(ops, settingOp{attribute: attribute, run: empty})
		}
	}

	stateOf := func(get func(*indexResourceModel) attr.Value) attr.Value {
		if state == nil {
			return get(plan)
		}
		return get(state)
	}

	emptyStrings := func(update func(*[]string) (*meilisearch.TaskInfo, error)) func() (*meilisearch.TaskInfo, error) {
		return func() (*meilisearch.TaskInfo, error) {
			empty := []string{}
			return update(&empty)
		}
	}

	add("ranking_rules", plan.RankingRules, len(plan.RankingRules.Elements()),
		stateOf(func(m *indexResourceModel) attr.Value { return m.RankingRules }),
		index.ResetRankingRules, emptyStrings(index.UpdateRankingRules))

	add("searchable_attributes", plan.SearchableAttributes, len(plan.SearchableAttributes.Elements()),
		stateOf(func(m *indexResourceModel) attr.Value { return m.SearchableAttributes }),
		index.ResetSearchableAttributes, emptyStrings(index.UpdateSearchableAttributes))

	add("displayed_attributes", plan.DisplayedAttributes, len(plan.DisplayedAttributes.Elements()),
		stateOf(func(m *indexResourceModel) attr.Value { return m.DisplayedAttributes }),
		index.ResetDisplayedAttributes, emptyStrings(index.UpdateDisplayedAttributes))

	add("filterable_attributes", plan.FilterableAttributes, len(plan.FilterableAttributes.Elements()),
		stateOf(func(m *indexResourceModel) attr.Value { return m.FilterableAttributes }),
		index.ResetFilterableAttributes, func() (*meilisearch.TaskInfo, error) {
			empty := []interface{}{}
			return index.UpdateFilterableAttributes(&empty)
		})

	add("sortable_attributes", plan.SortableAttributes, len(plan.SortableAttributes.Elements()),
		stateOf(func(m *indexResourceModel) attr.Value { return m.SortableAttributes }),
		index.ResetSortableAttributes, emptyStrings(index.UpdateSortableAttributes))

	add("stop_words", plan.StopWords, len(plan.StopWords.Elements()),
		stateOf(func(m *indexResourceModel) attr.Value { return m.StopWords }),
		index.ResetStopWords, emptyStrings(index.UpdateStopWords))

	add("dictionary", plan.Dictionary, len(plan.Dictionary.Elements()),
		stateOf(func(m *indexResourceModel) attr.Value { return m.Dictionary }),
		index.ResetDictionary, func() (*meilisearch.TaskInfo, error) {
			return index.UpdateDictionary([]string{})
		})

	add("separator_tokens", plan.SeparatorTokens, len(plan.SeparatorTokens.Elements()),
		stateOf(func(m *indexResourceModel) attr.Value { return m.SeparatorTokens }),
		index.ResetSeparatorTokens, func() (*meilisearch.TaskInfo, error) {
			return index.UpdateSeparatorTokens([]string{})
		})

	add("non_separator_tokens", plan.NonSeparatorTokens, len(plan.NonSeparatorTokens.Elements()),
		stateOf(func(m *indexResourceModel) attr.Value { return m.NonSeparatorTokens }),
		index.ResetNonSeparatorTokens, func() (*meilisearch.TaskInfo, error) {
			return index.UpdateNonSeparatorTokens([]string{})
		})

	add("synonyms", plan.Synonyms, len(plan.Synonyms.Elements()),
		stateOf(func(m *indexResourceModel) attr.Value { return m.Synonyms }),
		index.ResetSynonyms, func() (*meilisearch.TaskInfo, error) {
			empty := map[string][]string{}
			return index.UpdateSynonyms(&empty)
		})

	add("localized_attributes", plan.LocalizedAttributes, len(plan.LocalizedAttributes.Elements()),
		stateOf(func(m *indexResourceModel) attr.Value { return m.LocalizedAttributes }),
		index.ResetLocalizedAttributes, func() (*meilisearch.TaskInfo, error) {
			return index.UpdateLocalizedAttributes([]*meilisearch.LocalizedAttributes{})
		})

	add("embedders", plan.Embedders, len(plan.Embedders.Elements()),
		stateOf(func(m *indexResourceModel) attr.Value { return m.Embedders }),
		index.ResetEmbedders, func() (*meilisearch.TaskInfo, error) {
			return index.UpdateEmbedders(map[string]meilisearch.Embedder{})
		})

	// Scalars and nested objects can only be removed, never emptied.
	add("distinct_attribute", plan.DistinctAttribute, -1,
		stateOf(func(m *indexResourceModel) attr.Value { return m.DistinctAttribute }),
		index.ResetDistinctAttribute, nil)

	add("search_cutoff_ms", plan.SearchCutoffMs, -1,
		stateOf(func(m *indexResourceModel) attr.Value { return m.SearchCutoffMs }),
		index.ResetSearchCutoffMs, nil)

	add("proximity_precision", plan.ProximityPrecision, -1,
		stateOf(func(m *indexResourceModel) attr.Value { return m.ProximityPrecision }),
		index.ResetProximityPrecision, nil)

	add("typo_tolerance", plan.TypoTolerance, -1,
		stateOf(func(m *indexResourceModel) attr.Value { return m.TypoTolerance }),
		index.ResetTypoTolerance, nil)

	add("pagination", plan.Pagination, -1,
		stateOf(func(m *indexResourceModel) attr.Value { return m.Pagination }),
		index.ResetPagination, nil)

	add("faceting", plan.Faceting, -1,
		stateOf(func(m *indexResourceModel) attr.Value { return m.Faceting }),
		index.ResetFaceting, nil)

	return ops
}

// readSettings refreshes the settings attributes the configuration manages.
//
// Attributes that are null in the incoming model are left null: they are not
// managed by this resource, and populating them from the server would turn
// every unmanaged Meilisearch default into a permanent diff. The same rule is
// applied to nested attributes, so a `typo_tolerance` block that only sets
// `enabled` does not acquire the server's values for its siblings.
func (r *indexResource) readSettings(ctx context.Context, indexUID string, model *indexResourceModel, diags *diag.Diagnostics) {
	settings, err := r.client.Index(indexUID).GetSettingsWithContext(ctx)
	if err != nil {
		diags.AddError(
			"Error reading index settings",
			"Could not read settings for index "+indexUID+": "+err.Error(),
		)
		return
	}

	model.RankingRules = refreshList(ctx, model.RankingRules, settings.RankingRules, diags)
	model.SearchableAttributes = refreshSet(ctx, model.SearchableAttributes, settings.SearchableAttributes, diags)
	model.DisplayedAttributes = refreshSet(ctx, model.DisplayedAttributes, settings.DisplayedAttributes, diags)
	model.FilterableAttributes = refreshSet(ctx, model.FilterableAttributes, settings.FilterableAttributes, diags)
	model.SortableAttributes = refreshSet(ctx, model.SortableAttributes, settings.SortableAttributes, diags)
	model.StopWords = refreshSet(ctx, model.StopWords, settings.StopWords, diags)
	model.Dictionary = refreshSet(ctx, model.Dictionary, settings.Dictionary, diags)
	model.SeparatorTokens = refreshSet(ctx, model.SeparatorTokens, settings.SeparatorTokens, diags)
	model.NonSeparatorTokens = refreshSet(ctx, model.NonSeparatorTokens, settings.NonSeparatorTokens, diags)

	if !model.DistinctAttribute.IsNull() && settings.DistinctAttribute != nil {
		model.DistinctAttribute = types.StringValue(*settings.DistinctAttribute)
	}

	if !model.SearchCutoffMs.IsNull() {
		model.SearchCutoffMs = types.Int64Value(settings.SearchCutoffMs)
	}

	if !model.ProximityPrecision.IsNull() && settings.ProximityPrecision != "" {
		model.ProximityPrecision = types.StringValue(string(settings.ProximityPrecision))
	}

	if !model.Synonyms.IsNull() {
		synonyms := settings.Synonyms
		if synonyms == nil {
			synonyms = map[string][]string{}
		}
		value, d := types.MapValueFrom(ctx, types.ListType{ElemType: types.StringType}, synonyms)
		diags.Append(d...)
		model.Synonyms = value
	}

	model.TypoTolerance = refreshTypoTolerance(ctx, model.TypoTolerance, settings.TypoTolerance, diags)
	model.Pagination = refreshPagination(model.Pagination, settings.Pagination, diags)
	model.Faceting = refreshFaceting(ctx, model.Faceting, settings.Faceting, diags)
	model.LocalizedAttributes = refreshLocalizedAttributes(ctx, model.LocalizedAttributes, settings.LocalizedAttributes, diags)
	model.Embedders = refreshEmbedders(ctx, model.Embedders, settings.Embedders, diags)
}

// isSet reports whether an attribute holds a usable value, i.e. the practitioner
// configured it and Terraform has resolved it.
func isSet(value attr.Value) bool {
	return !value.IsNull() && !value.IsUnknown()
}

func refreshSet(ctx context.Context, current types.Set, values []string, diags *diag.Diagnostics) types.Set {
	if current.IsNull() {
		return current
	}
	if values == nil {
		values = []string{}
	}

	value, d := types.SetValueFrom(ctx, types.StringType, values)
	diags.Append(d...)

	return value
}

func refreshList(ctx context.Context, current types.List, values []string, diags *diag.Diagnostics) types.List {
	if current.IsNull() {
		return current
	}
	if values == nil {
		values = []string{}
	}

	value, d := types.ListValueFrom(ctx, types.StringType, values)
	diags.Append(d...)

	return value
}

func objectToTypoTolerance(ctx context.Context, obj types.Object, diags *diag.Diagnostics) *meilisearch.TypoTolerance {
	attrs := obj.Attributes()

	tt := &meilisearch.TypoTolerance{}

	if v, ok := attrs["enabled"].(types.Bool); ok && isSet(v) {
		tt.Enabled = v.ValueBool()
	}

	if v, ok := attrs["min_word_size_for_typos"].(types.Object); ok && isSet(v) {
		sub := v.Attributes()
		if oneTypo, ok := sub["one_typo"].(types.Int64); ok && isSet(oneTypo) {
			tt.MinWordSizeForTypos.OneTypo = oneTypo.ValueInt64()
		}
		if twoTypos, ok := sub["two_typos"].(types.Int64); ok && isSet(twoTypos) {
			tt.MinWordSizeForTypos.TwoTypos = twoTypos.ValueInt64()
		}
	}

	if v, ok := attrs["disable_on_words"].(types.Set); ok && isSet(v) {
		diags.Append(v.ElementsAs(ctx, &tt.DisableOnWords, false)...)
	}

	if v, ok := attrs["disable_on_attributes"].(types.Set); ok && isSet(v) {
		diags.Append(v.ElementsAs(ctx, &tt.DisableOnAttributes, false)...)
	}

	return tt
}

func refreshTypoTolerance(ctx context.Context, current types.Object, tt *meilisearch.TypoTolerance, diags *diag.Diagnostics) types.Object {
	if current.IsNull() || tt == nil {
		return current
	}

	attrs := current.Attributes()
	values := map[string]attr.Value{
		"enabled":                 attrs["enabled"],
		"min_word_size_for_typos": attrs["min_word_size_for_typos"],
		"disable_on_words":        attrs["disable_on_words"],
		"disable_on_attributes":   attrs["disable_on_attributes"],
	}

	if v, ok := attrs["enabled"].(types.Bool); ok && !v.IsNull() {
		values["enabled"] = types.BoolValue(tt.Enabled)
	}

	if v, ok := attrs["min_word_size_for_typos"].(types.Object); ok && !v.IsNull() {
		sub := v.Attributes()
		subValues := map[string]attr.Value{
			"one_typo":  sub["one_typo"],
			"two_typos": sub["two_typos"],
		}
		if oneTypo, ok := sub["one_typo"].(types.Int64); ok && !oneTypo.IsNull() {
			subValues["one_typo"] = types.Int64Value(tt.MinWordSizeForTypos.OneTypo)
		}
		if twoTypos, ok := sub["two_typos"].(types.Int64); ok && !twoTypos.IsNull() {
			subValues["two_typos"] = types.Int64Value(tt.MinWordSizeForTypos.TwoTypos)
		}

		minWordSize, d := types.ObjectValue(minWordSizeForTyposAttrTypes, subValues)
		diags.Append(d...)
		values["min_word_size_for_typos"] = minWordSize
	}

	if v, ok := attrs["disable_on_words"].(types.Set); ok && !v.IsNull() {
		values["disable_on_words"] = refreshSet(ctx, v, tt.DisableOnWords, diags)
	}

	if v, ok := attrs["disable_on_attributes"].(types.Set); ok && !v.IsNull() {
		values["disable_on_attributes"] = refreshSet(ctx, v, tt.DisableOnAttributes, diags)
	}

	obj, d := types.ObjectValue(typoToleranceAttrTypes, values)
	diags.Append(d...)

	return obj
}

func objectToPagination(obj types.Object, _ *diag.Diagnostics) *meilisearch.Pagination {
	attrs := obj.Attributes()

	p := &meilisearch.Pagination{}
	if v, ok := attrs["max_total_hits"].(types.Int64); ok && isSet(v) {
		p.MaxTotalHits = v.ValueInt64()
	}

	return p
}

func refreshPagination(current types.Object, p *meilisearch.Pagination, diags *diag.Diagnostics) types.Object {
	if current.IsNull() || p == nil {
		return current
	}

	attrs := current.Attributes()
	values := map[string]attr.Value{"max_total_hits": attrs["max_total_hits"]}

	if v, ok := attrs["max_total_hits"].(types.Int64); ok && !v.IsNull() {
		values["max_total_hits"] = types.Int64Value(p.MaxTotalHits)
	}

	obj, d := types.ObjectValue(paginationAttrTypes, values)
	diags.Append(d...)

	return obj
}

func objectToFaceting(ctx context.Context, obj types.Object, diags *diag.Diagnostics) *meilisearch.Faceting {
	attrs := obj.Attributes()

	f := &meilisearch.Faceting{}

	if v, ok := attrs["max_values_per_facet"].(types.Int64); ok && isSet(v) {
		f.MaxValuesPerFacet = v.ValueInt64()
	}

	if v, ok := attrs["sort_facet_values_by"].(types.Map); ok && isSet(v) {
		var m map[string]string
		diags.Append(v.ElementsAs(ctx, &m, false)...)
		if len(m) > 0 {
			f.SortFacetValuesBy = make(map[string]meilisearch.SortFacetType, len(m))
			for k, val := range m {
				f.SortFacetValuesBy[k] = meilisearch.SortFacetType(val)
			}
		}
	}

	return f
}

func refreshFaceting(ctx context.Context, current types.Object, f *meilisearch.Faceting, diags *diag.Diagnostics) types.Object {
	if current.IsNull() || f == nil {
		return current
	}

	attrs := current.Attributes()
	values := map[string]attr.Value{
		"max_values_per_facet": attrs["max_values_per_facet"],
		"sort_facet_values_by": attrs["sort_facet_values_by"],
	}

	if v, ok := attrs["max_values_per_facet"].(types.Int64); ok && !v.IsNull() {
		values["max_values_per_facet"] = types.Int64Value(f.MaxValuesPerFacet)
	}

	if v, ok := attrs["sort_facet_values_by"].(types.Map); ok && !v.IsNull() {
		sortBy := make(map[string]string, len(f.SortFacetValuesBy))
		for k, sortType := range f.SortFacetValuesBy {
			sortBy[k] = string(sortType)
		}
		value, d := types.MapValueFrom(ctx, types.StringType, sortBy)
		diags.Append(d...)
		values["sort_facet_values_by"] = value
	}

	obj, d := types.ObjectValue(facetingAttrTypes, values)
	diags.Append(d...)

	return obj
}

func listToLocalizedAttributes(ctx context.Context, list types.List, diags *diag.Diagnostics) []*meilisearch.LocalizedAttributes {
	elements := list.Elements()
	if len(elements) == 0 {
		return nil
	}

	result := make([]*meilisearch.LocalizedAttributes, 0, len(elements))
	for i, elem := range elements {
		obj, ok := elem.(types.Object)
		if !ok {
			diags.AddError(
				"Unexpected localized_attributes element",
				fmt.Sprintf("Element %d is %T, expected an object. This is a bug in the provider.", i, elem),
			)
			continue
		}
		attrs := obj.Attributes()

		la := &meilisearch.LocalizedAttributes{}
		if v, ok := attrs["locales"].(types.Set); ok && isSet(v) {
			diags.Append(v.ElementsAs(ctx, &la.Locales, false)...)
		}
		if v, ok := attrs["attribute_patterns"].(types.Set); ok && isSet(v) {
			diags.Append(v.ElementsAs(ctx, &la.AttributePatterns, false)...)
		}
		result = append(result, la)
	}

	return result
}

func refreshLocalizedAttributes(ctx context.Context, current types.List, list []*meilisearch.LocalizedAttributes, diags *diag.Diagnostics) types.List {
	objType := types.ObjectType{AttrTypes: localizedAttributeAttrTypes}

	if current.IsNull() {
		return current
	}

	values := make([]attr.Value, 0, len(list))
	for _, la := range list {
		if la == nil {
			continue
		}
		locales, d := types.SetValueFrom(ctx, types.StringType, la.Locales)
		diags.Append(d...)
		patterns, d := types.SetValueFrom(ctx, types.StringType, la.AttributePatterns)
		diags.Append(d...)

		obj, d := types.ObjectValue(localizedAttributeAttrTypes, map[string]attr.Value{
			"locales":            locales,
			"attribute_patterns": patterns,
		})
		diags.Append(d...)

		values = append(values, obj)
	}

	result, d := types.ListValue(objType, values)
	diags.Append(d...)

	return result
}

func mapToEmbedders(ctx context.Context, m types.Map, diags *diag.Diagnostics) map[string]meilisearch.Embedder {
	elements := m.Elements()
	if len(elements) == 0 {
		return nil
	}

	result := make(map[string]meilisearch.Embedder, len(elements))
	for name, elem := range elements {
		obj, ok := elem.(types.Object)
		if !ok {
			diags.AddError(
				"Unexpected embedders element",
				fmt.Sprintf("Embedder %q is %T, expected an object. This is a bug in the provider.", name, elem),
			)
			continue
		}
		attrs := obj.Attributes()

		emb := meilisearch.Embedder{}

		if v, ok := attrs["source"].(types.String); ok && isSet(v) {
			emb.Source = meilisearch.EmbedderSource(v.ValueString())
		}
		if v, ok := attrs["model"].(types.String); ok && isSet(v) {
			emb.Model = v.ValueString()
		}
		if v, ok := attrs["api_key"].(types.String); ok && isSet(v) {
			emb.APIKey = v.ValueString()
		}
		if v, ok := attrs["document_template"].(types.String); ok && isSet(v) {
			emb.DocumentTemplate = v.ValueString()
		}
		if v, ok := attrs["dimensions"].(types.Int64); ok && isSet(v) {
			emb.Dimensions = int(v.ValueInt64())
		}
		if v, ok := attrs["url"].(types.String); ok && isSet(v) {
			emb.URL = v.ValueString()
		}
		if v, ok := attrs["revision"].(types.String); ok && isSet(v) {
			emb.Revision = v.ValueString()
		}
		if v, ok := attrs["request"].(types.String); ok && isSet(v) && v.ValueString() != "" {
			var req map[string]interface{}
			if err := json.Unmarshal([]byte(v.ValueString()), &req); err != nil {
				diags.AddError("Invalid embedder request JSON", fmt.Sprintf("Embedder %q: %s", name, err))
			} else {
				emb.Request = req
			}
		}
		if v, ok := attrs["response"].(types.String); ok && isSet(v) && v.ValueString() != "" {
			var resp map[string]interface{}
			if err := json.Unmarshal([]byte(v.ValueString()), &resp); err != nil {
				diags.AddError("Invalid embedder response JSON", fmt.Sprintf("Embedder %q: %s", name, err))
			} else {
				emb.Response = resp
			}
		}
		if v, ok := attrs["headers"].(types.Map); ok && isSet(v) {
			diags.Append(v.ElementsAs(ctx, &emb.Headers, false)...)
		}

		result[name] = emb
	}

	return result
}

// refreshEmbedders refreshes only the embedders the configuration declares, and
// within each one only the fields it sets. `api_key` is never refreshed because
// Meilisearch redacts it on read, which would otherwise overwrite the real key
// in state with a mask.
func refreshEmbedders(ctx context.Context, current types.Map, embedders map[string]meilisearch.Embedder, diags *diag.Diagnostics) types.Map {
	objType := types.ObjectType{AttrTypes: embedderAttrTypes}

	if current.IsNull() {
		return current
	}

	values := make(map[string]attr.Value, len(current.Elements()))
	for name, elem := range current.Elements() {
		obj, ok := elem.(types.Object)
		if !ok {
			values[name] = elem
			continue
		}

		emb, found := embedders[name]
		if !found {
			// Removed outside Terraform: drop it so the next plan recreates it.
			continue
		}

		attrs := obj.Attributes()
		next := make(map[string]attr.Value, len(embedderAttrTypes))
		for key, value := range attrs {
			next[key] = value
		}

		if v, ok := attrs["source"].(types.String); ok && !v.IsNull() {
			next["source"] = types.StringValue(string(emb.Source))
		}
		if v, ok := attrs["model"].(types.String); ok && !v.IsNull() {
			next["model"] = types.StringValue(emb.Model)
		}
		if v, ok := attrs["document_template"].(types.String); ok && !v.IsNull() {
			next["document_template"] = types.StringValue(emb.DocumentTemplate)
		}
		if v, ok := attrs["dimensions"].(types.Int64); ok && !v.IsNull() {
			next["dimensions"] = types.Int64Value(int64(emb.Dimensions))
		}
		if v, ok := attrs["url"].(types.String); ok && !v.IsNull() {
			next["url"] = types.StringValue(emb.URL)
		}
		if v, ok := attrs["revision"].(types.String); ok && !v.IsNull() {
			next["revision"] = types.StringValue(emb.Revision)
		}
		if v, ok := attrs["headers"].(types.Map); ok && !v.IsNull() {
			headers, d := types.MapValueFrom(ctx, types.StringType, emb.Headers)
			diags.Append(d...)
			next["headers"] = headers
		}

		value, d := types.ObjectValue(embedderAttrTypes, next)
		diags.Append(d...)
		values[name] = value
	}

	result, d := types.MapValue(objType, values)
	diags.Append(d...)

	return result
}
