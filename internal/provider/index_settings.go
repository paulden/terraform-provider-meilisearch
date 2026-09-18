package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/meilisearch/meilisearch-go"
)

// applySettings sends the given settings to Meilisearch and waits for the
// resulting task to complete.
func (r *indexResource) applySettings(ctx context.Context, indexUID string, settings *meilisearch.Settings) error {
	task, err := r.client.Index(indexUID).UpdateSettings(settings)
	if err != nil {
		return fmt.Errorf("could not update settings: %w", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	waitTask, err := r.client.WaitForTaskWithContext(waitCtx, task.TaskUID, 100*time.Millisecond)
	if err != nil {
		return fmt.Errorf("error waiting for settings update task: %w", err)
	}

	if waitTask.Status != "succeeded" {
		return fmt.Errorf("settings update task failed with status: %s", waitTask.Status)
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

// indexSettingsSchemaAttributes returns the schema attributes covering the full
// Meilisearch index settings API, to be merged into the index resource schema.
func indexSettingsSchemaAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"ranking_rules": schema.ListAttribute{
			Description: "Ordered list of ranking rules applied to search results. Default is [\"words\", \"typo\", \"proximity\", \"attribute\", \"sort\", \"exactness\"].",
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
		"dictionary": schema.SetAttribute{
			Description: "Set of words considered as a single term by the tokenizer.",
			Optional:    true,
			Computed:    true,
			ElementType: types.StringType,
		},
		"search_cutoff_ms": schema.Int64Attribute{
			Description: "Maximum duration, in milliseconds, of a search query.",
			Optional:    true,
			Computed:    true,
		},
		"proximity_precision": schema.StringAttribute{
			Description: "Precision level when calculating the proximity ranking rule. One of \"byWord\" or \"byAttribute\".",
			Optional:    true,
			Computed:    true,
		},
		"separator_tokens": schema.SetAttribute{
			Description: "Set of characters or words treated as word separators by the tokenizer.",
			Optional:    true,
			Computed:    true,
			ElementType: types.StringType,
		},
		"non_separator_tokens": schema.SetAttribute{
			Description: "Set of characters or words normally treated as separators that should instead be treated as normal characters.",
			Optional:    true,
			Computed:    true,
			ElementType: types.StringType,
		},
		"typo_tolerance": schema.SingleNestedAttribute{
			Description: "Controls the typo tolerance feature.",
			Optional:    true,
			Computed:    true,
			Attributes: map[string]schema.Attribute{
				"enabled": schema.BoolAttribute{
					Description: "Whether typo tolerance is enabled.",
					Optional:    true,
					Computed:    true,
				},
				"min_word_size_for_typos": schema.SingleNestedAttribute{
					Description: "Minimum word size for accepting typos.",
					Optional:    true,
					Computed:    true,
					Attributes: map[string]schema.Attribute{
						"one_typo": schema.Int64Attribute{
							Description: "Minimum word size to accept one typo.",
							Optional:    true,
							Computed:    true,
						},
						"two_typos": schema.Int64Attribute{
							Description: "Minimum word size to accept two typos.",
							Optional:    true,
							Computed:    true,
						},
					},
				},
				"disable_on_words": schema.SetAttribute{
					Description: "Set of words for which typo tolerance is disabled.",
					Optional:    true,
					Computed:    true,
					ElementType: types.StringType,
				},
				"disable_on_attributes": schema.SetAttribute{
					Description: "Set of attributes for which typo tolerance is disabled.",
					Optional:    true,
					Computed:    true,
					ElementType: types.StringType,
				},
			},
		},
		"pagination": schema.SingleNestedAttribute{
			Description: "Controls pagination settings.",
			Optional:    true,
			Computed:    true,
			Attributes: map[string]schema.Attribute{
				"max_total_hits": schema.Int64Attribute{
					Description: "Maximum number of search results that can be returned.",
					Optional:    true,
					Computed:    true,
				},
			},
		},
		"faceting": schema.SingleNestedAttribute{
			Description: "Controls faceting settings.",
			Optional:    true,
			Computed:    true,
			Attributes: map[string]schema.Attribute{
				"max_values_per_facet": schema.Int64Attribute{
					Description: "Maximum number of facet values returned for each facet.",
					Optional:    true,
					Computed:    true,
				},
				"sort_facet_values_by": schema.MapAttribute{
					Description: "Map of facet name (or \"*\") to sort order, \"alpha\" or \"count\".",
					Optional:    true,
					Computed:    true,
					ElementType: types.StringType,
				},
			},
		},
		"localized_attributes": schema.ListNestedAttribute{
			Description: "List of localized attribute rules, associating attribute patterns with locales.",
			Optional:    true,
			Computed:    true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"locales": schema.SetAttribute{
						Description: "Set of locales to apply to the matched attributes.",
						Optional:    true,
						Computed:    true,
						ElementType: types.StringType,
					},
					"attribute_patterns": schema.SetAttribute{
						Description: "Set of attribute name patterns this rule applies to.",
						Optional:    true,
						Computed:    true,
						ElementType: types.StringType,
					},
				},
			},
		},
		"embedders": schema.MapNestedAttribute{
			Description: "Map of embedder name to embedder configuration, used for AI-powered search.",
			Optional:    true,
			Computed:    true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"source": schema.StringAttribute{
						Description: "Embedder source: \"openAi\", \"huggingFace\", \"userProvided\", \"rest\", or \"ollama\".",
						Optional:    true,
						Computed:    true,
					},
					"model": schema.StringAttribute{
						Description: "Model name, for \"openAi\", \"huggingFace\" or \"ollama\" sources.",
						Optional:    true,
						Computed:    true,
					},
					"api_key": schema.StringAttribute{
						Description: "API key, for \"openAi\", \"rest\" or \"ollama\" sources.",
						Optional:    true,
						Computed:    true,
						Sensitive:   true,
					},
					"document_template": schema.StringAttribute{
						Description: "Template describing the data Meilisearch sends the embedder.",
						Optional:    true,
						Computed:    true,
					},
					"dimensions": schema.Int64Attribute{
						Description: "Number of dimensions in the embedding output.",
						Optional:    true,
						Computed:    true,
					},
					"url": schema.StringAttribute{
						Description: "URL for \"openAi\", \"rest\" or \"ollama\" sources.",
						Optional:    true,
						Computed:    true,
					},
					"revision": schema.StringAttribute{
						Description: "Model revision, for the \"huggingFace\" source.",
						Optional:    true,
						Computed:    true,
					},
					"request": schema.StringAttribute{
						Description: "JSON-encoded request body template, for the \"rest\" source.",
						Optional:    true,
						Computed:    true,
					},
					"response": schema.StringAttribute{
						Description: "JSON-encoded response body template, for the \"rest\" source.",
						Optional:    true,
						Computed:    true,
					},
					"headers": schema.MapAttribute{
						Description: "Map of extra HTTP headers, for the \"rest\" source.",
						Optional:    true,
						Computed:    true,
						ElementType: types.StringType,
					},
				},
			},
		},
	}
}

// buildSettingsFromPlan constructs a Meilisearch Settings object from the Terraform plan.
// Only fields explicitly set (non-null, non-unknown) in the plan are sent, so
// attributes left out of the config keep their previously computed value.
func (r *indexResource) buildSettingsFromPlan(ctx context.Context, plan *indexResourceModel, diags *diag.Diagnostics) *meilisearch.Settings {
	var hasSettings bool
	settings := &meilisearch.Settings{}

	if !plan.RankingRules.IsNull() && !plan.RankingRules.IsUnknown() {
		var v []string
		diags.Append(plan.RankingRules.ElementsAs(ctx, &v, false)...)
		settings.RankingRules = v
		hasSettings = true
	}

	if !plan.DistinctAttribute.IsNull() && !plan.DistinctAttribute.IsUnknown() {
		v := plan.DistinctAttribute.ValueString()
		settings.DistinctAttribute = &v
		hasSettings = true
	}

	if !plan.SearchableAttributes.IsNull() && !plan.SearchableAttributes.IsUnknown() {
		var v []string
		diags.Append(plan.SearchableAttributes.ElementsAs(ctx, &v, false)...)
		settings.SearchableAttributes = v
		hasSettings = true
	}

	if !plan.DisplayedAttributes.IsNull() && !plan.DisplayedAttributes.IsUnknown() {
		var v []string
		diags.Append(plan.DisplayedAttributes.ElementsAs(ctx, &v, false)...)
		settings.DisplayedAttributes = v
		hasSettings = true
	}

	if !plan.FilterableAttributes.IsNull() && !plan.FilterableAttributes.IsUnknown() {
		var v []string
		diags.Append(plan.FilterableAttributes.ElementsAs(ctx, &v, false)...)
		settings.FilterableAttributes = v
		hasSettings = true
	}

	if !plan.SortableAttributes.IsNull() && !plan.SortableAttributes.IsUnknown() {
		var v []string
		diags.Append(plan.SortableAttributes.ElementsAs(ctx, &v, false)...)
		settings.SortableAttributes = v
		hasSettings = true
	}

	if !plan.StopWords.IsNull() && !plan.StopWords.IsUnknown() {
		var v []string
		diags.Append(plan.StopWords.ElementsAs(ctx, &v, false)...)
		settings.StopWords = v
		hasSettings = true
	}

	if !plan.Synonyms.IsNull() && !plan.Synonyms.IsUnknown() {
		var v map[string][]string
		diags.Append(plan.Synonyms.ElementsAs(ctx, &v, false)...)
		settings.Synonyms = v
		hasSettings = true
	}

	if !plan.Dictionary.IsNull() && !plan.Dictionary.IsUnknown() {
		var v []string
		diags.Append(plan.Dictionary.ElementsAs(ctx, &v, false)...)
		settings.Dictionary = v
		hasSettings = true
	}

	if !plan.SearchCutoffMs.IsNull() && !plan.SearchCutoffMs.IsUnknown() {
		settings.SearchCutoffMs = plan.SearchCutoffMs.ValueInt64()
		hasSettings = true
	}

	if !plan.ProximityPrecision.IsNull() && !plan.ProximityPrecision.IsUnknown() {
		settings.ProximityPrecision = meilisearch.ProximityPrecisionType(plan.ProximityPrecision.ValueString())
		hasSettings = true
	}

	if !plan.SeparatorTokens.IsNull() && !plan.SeparatorTokens.IsUnknown() {
		var v []string
		diags.Append(plan.SeparatorTokens.ElementsAs(ctx, &v, false)...)
		settings.SeparatorTokens = v
		hasSettings = true
	}

	if !plan.NonSeparatorTokens.IsNull() && !plan.NonSeparatorTokens.IsUnknown() {
		var v []string
		diags.Append(plan.NonSeparatorTokens.ElementsAs(ctx, &v, false)...)
		settings.NonSeparatorTokens = v
		hasSettings = true
	}

	if !plan.TypoTolerance.IsNull() && !plan.TypoTolerance.IsUnknown() {
		tt := objectToTypoTolerance(ctx, plan.TypoTolerance, diags)
		settings.TypoTolerance = tt
		hasSettings = true
	}

	if !plan.Pagination.IsNull() && !plan.Pagination.IsUnknown() {
		p := objectToPagination(ctx, plan.Pagination, diags)
		settings.Pagination = p
		hasSettings = true
	}

	if !plan.Faceting.IsNull() && !plan.Faceting.IsUnknown() {
		f := objectToFaceting(ctx, plan.Faceting, diags)
		settings.Faceting = f
		hasSettings = true
	}

	if !plan.LocalizedAttributes.IsNull() && !plan.LocalizedAttributes.IsUnknown() {
		la := listToLocalizedAttributes(ctx, plan.LocalizedAttributes, diags)
		settings.LocalizedAttributes = la
		hasSettings = true
	}

	if !plan.Embedders.IsNull() && !plan.Embedders.IsUnknown() {
		emb := mapToEmbedders(ctx, plan.Embedders, diags)
		settings.Embedders = emb
		hasSettings = true
	}

	if diags.HasError() {
		return nil
	}

	if hasSettings {
		return settings
	}
	return nil
}

// readSettings retrieves settings from Meilisearch and populates the model.
func (r *indexResource) readSettings(ctx context.Context, indexUID string, model *indexResourceModel, diags *diag.Diagnostics) {
	settings, err := r.client.Index(indexUID).GetSettings()
	if err != nil {
		diags.AddError(
			"Error reading index settings",
			"Could not read settings for index "+indexUID+": "+err.Error(),
		)
		return
	}

	var d diag.Diagnostics

	model.RankingRules, d = types.ListValueFrom(ctx, types.StringType, settings.RankingRules)
	diags.Append(d...)

	if settings.DistinctAttribute != nil {
		model.DistinctAttribute = types.StringValue(*settings.DistinctAttribute)
	} else {
		model.DistinctAttribute = types.StringNull()
	}

	model.SearchableAttributes, d = types.SetValueFrom(ctx, types.StringType, settings.SearchableAttributes)
	diags.Append(d...)

	model.DisplayedAttributes, d = types.SetValueFrom(ctx, types.StringType, settings.DisplayedAttributes)
	diags.Append(d...)

	model.FilterableAttributes, d = types.SetValueFrom(ctx, types.StringType, settings.FilterableAttributes)
	diags.Append(d...)

	model.SortableAttributes, d = types.SetValueFrom(ctx, types.StringType, settings.SortableAttributes)
	diags.Append(d...)

	model.StopWords, d = types.SetValueFrom(ctx, types.StringType, settings.StopWords)
	diags.Append(d...)

	model.Synonyms, d = types.MapValueFrom(ctx, types.ListType{ElemType: types.StringType}, settings.Synonyms)
	diags.Append(d...)

	model.Dictionary, d = types.SetValueFrom(ctx, types.StringType, settings.Dictionary)
	diags.Append(d...)

	model.SearchCutoffMs = types.Int64Value(settings.SearchCutoffMs)

	model.ProximityPrecision = types.StringValue(string(settings.ProximityPrecision))

	model.SeparatorTokens, d = types.SetValueFrom(ctx, types.StringType, settings.SeparatorTokens)
	diags.Append(d...)

	model.NonSeparatorTokens, d = types.SetValueFrom(ctx, types.StringType, settings.NonSeparatorTokens)
	diags.Append(d...)

	model.TypoTolerance = typoToleranceToObject(ctx, settings.TypoTolerance, diags)
	model.Pagination = paginationToObject(ctx, settings.Pagination, diags)
	model.Faceting = facetingToObject(ctx, settings.Faceting, diags)
	model.LocalizedAttributes = localizedAttributesToList(ctx, settings.LocalizedAttributes, diags)
	model.Embedders = embeddersToMap(ctx, settings.Embedders, diags)
}

func objectToTypoTolerance(ctx context.Context, obj types.Object, diags *diag.Diagnostics) *meilisearch.TypoTolerance {
	attrs := obj.Attributes()

	tt := &meilisearch.TypoTolerance{}

	if v, ok := attrs["enabled"].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
		tt.Enabled = v.ValueBool()
	}

	if v, ok := attrs["min_word_size_for_typos"].(types.Object); ok && !v.IsNull() && !v.IsUnknown() {
		sub := v.Attributes()
		if oneTypo, ok := sub["one_typo"].(types.Int64); ok && !oneTypo.IsNull() && !oneTypo.IsUnknown() {
			tt.MinWordSizeForTypos.OneTypo = oneTypo.ValueInt64()
		}
		if twoTypos, ok := sub["two_typos"].(types.Int64); ok && !twoTypos.IsNull() && !twoTypos.IsUnknown() {
			tt.MinWordSizeForTypos.TwoTypos = twoTypos.ValueInt64()
		}
	}

	if v, ok := attrs["disable_on_words"].(types.Set); ok && !v.IsNull() && !v.IsUnknown() {
		diags.Append(v.ElementsAs(ctx, &tt.DisableOnWords, false)...)
	}

	if v, ok := attrs["disable_on_attributes"].(types.Set); ok && !v.IsNull() && !v.IsUnknown() {
		diags.Append(v.ElementsAs(ctx, &tt.DisableOnAttributes, false)...)
	}

	return tt
}

func typoToleranceToObject(ctx context.Context, tt *meilisearch.TypoTolerance, diags *diag.Diagnostics) types.Object {
	if tt == nil {
		return types.ObjectNull(typoToleranceAttrTypes)
	}

	disableOnWords, d := types.SetValueFrom(ctx, types.StringType, tt.DisableOnWords)
	diags.Append(d...)
	disableOnAttributes, d := types.SetValueFrom(ctx, types.StringType, tt.DisableOnAttributes)
	diags.Append(d...)

	minWordSize, d := types.ObjectValue(minWordSizeForTyposAttrTypes, map[string]attr.Value{
		"one_typo":  types.Int64Value(tt.MinWordSizeForTypos.OneTypo),
		"two_typos": types.Int64Value(tt.MinWordSizeForTypos.TwoTypos),
	})
	diags.Append(d...)

	obj, d := types.ObjectValue(typoToleranceAttrTypes, map[string]attr.Value{
		"enabled":                 types.BoolValue(tt.Enabled),
		"min_word_size_for_typos": minWordSize,
		"disable_on_words":        disableOnWords,
		"disable_on_attributes":   disableOnAttributes,
	})
	diags.Append(d...)

	return obj
}

func objectToPagination(_ context.Context, obj types.Object, _ *diag.Diagnostics) *meilisearch.Pagination {
	attrs := obj.Attributes()
	p := &meilisearch.Pagination{}
	if v, ok := attrs["max_total_hits"].(types.Int64); ok && !v.IsNull() && !v.IsUnknown() {
		p.MaxTotalHits = v.ValueInt64()
	}
	return p
}

func paginationToObject(_ context.Context, p *meilisearch.Pagination, diags *diag.Diagnostics) types.Object {
	if p == nil {
		return types.ObjectNull(paginationAttrTypes)
	}
	obj, d := types.ObjectValue(paginationAttrTypes, map[string]attr.Value{
		"max_total_hits": types.Int64Value(p.MaxTotalHits),
	})
	diags.Append(d...)
	return obj
}

func objectToFaceting(ctx context.Context, obj types.Object, diags *diag.Diagnostics) *meilisearch.Faceting {
	attrs := obj.Attributes()
	f := &meilisearch.Faceting{}

	if v, ok := attrs["max_values_per_facet"].(types.Int64); ok && !v.IsNull() && !v.IsUnknown() {
		f.MaxValuesPerFacet = v.ValueInt64()
	}

	if v, ok := attrs["sort_facet_values_by"].(types.Map); ok && !v.IsNull() && !v.IsUnknown() {
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

func facetingToObject(ctx context.Context, f *meilisearch.Faceting, diags *diag.Diagnostics) types.Object {
	if f == nil {
		return types.ObjectNull(facetingAttrTypes)
	}

	sortBy := make(map[string]string, len(f.SortFacetValuesBy))
	for k, v := range f.SortFacetValuesBy {
		sortBy[k] = string(v)
	}

	sortByValue, d := types.MapValueFrom(ctx, types.StringType, sortBy)
	diags.Append(d...)

	obj, d := types.ObjectValue(facetingAttrTypes, map[string]attr.Value{
		"max_values_per_facet": types.Int64Value(f.MaxValuesPerFacet),
		"sort_facet_values_by": sortByValue,
	})
	diags.Append(d...)

	return obj
}

func listToLocalizedAttributes(ctx context.Context, list types.List, diags *diag.Diagnostics) []*meilisearch.LocalizedAttributes {
	elements := list.Elements()
	if len(elements) == 0 {
		return nil
	}

	result := make([]*meilisearch.LocalizedAttributes, 0, len(elements))
	for _, elem := range elements {
		obj, ok := elem.(types.Object)
		if !ok {
			continue
		}
		attrs := obj.Attributes()

		la := &meilisearch.LocalizedAttributes{}
		if v, ok := attrs["locales"].(types.Set); ok && !v.IsNull() && !v.IsUnknown() {
			diags.Append(v.ElementsAs(ctx, &la.Locales, false)...)
		}
		if v, ok := attrs["attribute_patterns"].(types.Set); ok && !v.IsNull() && !v.IsUnknown() {
			diags.Append(v.ElementsAs(ctx, &la.AttributePatterns, false)...)
		}
		result = append(result, la)
	}

	return result
}

func localizedAttributesToList(ctx context.Context, list []*meilisearch.LocalizedAttributes, diags *diag.Diagnostics) types.List {
	objType := types.ObjectType{AttrTypes: localizedAttributeAttrTypes}

	if len(list) == 0 {
		return types.ListNull(objType)
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
			continue
		}
		attrs := obj.Attributes()

		emb := meilisearch.Embedder{}

		if v, ok := attrs["source"].(types.String); ok && !v.IsNull() && !v.IsUnknown() {
			emb.Source = v.ValueString()
		}
		if v, ok := attrs["model"].(types.String); ok && !v.IsNull() && !v.IsUnknown() {
			emb.Model = v.ValueString()
		}
		if v, ok := attrs["api_key"].(types.String); ok && !v.IsNull() && !v.IsUnknown() {
			emb.APIKey = v.ValueString()
		}
		if v, ok := attrs["document_template"].(types.String); ok && !v.IsNull() && !v.IsUnknown() {
			emb.DocumentTemplate = v.ValueString()
		}
		if v, ok := attrs["dimensions"].(types.Int64); ok && !v.IsNull() && !v.IsUnknown() {
			emb.Dimensions = int(v.ValueInt64())
		}
		if v, ok := attrs["url"].(types.String); ok && !v.IsNull() && !v.IsUnknown() {
			emb.URL = v.ValueString()
		}
		if v, ok := attrs["revision"].(types.String); ok && !v.IsNull() && !v.IsUnknown() {
			emb.Revision = v.ValueString()
		}
		if v, ok := attrs["request"].(types.String); ok && !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
			var req map[string]interface{}
			if err := json.Unmarshal([]byte(v.ValueString()), &req); err != nil {
				diags.AddError("Invalid embedder request JSON", err.Error())
			} else {
				emb.Request = req
			}
		}
		if v, ok := attrs["response"].(types.String); ok && !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
			var resp map[string]interface{}
			if err := json.Unmarshal([]byte(v.ValueString()), &resp); err != nil {
				diags.AddError("Invalid embedder response JSON", err.Error())
			} else {
				emb.Response = resp
			}
		}
		if v, ok := attrs["headers"].(types.Map); ok && !v.IsNull() && !v.IsUnknown() {
			diags.Append(v.ElementsAs(ctx, &emb.Headers, false)...)
		}

		result[name] = emb
	}

	return result
}

func embeddersToMap(ctx context.Context, embedders map[string]meilisearch.Embedder, diags *diag.Diagnostics) types.Map {
	objType := types.ObjectType{AttrTypes: embedderAttrTypes}

	if len(embedders) == 0 {
		return types.MapNull(objType)
	}

	values := make(map[string]attr.Value, len(embedders))
	for name, emb := range embedders {
		var requestJSON, responseJSON types.String
		if emb.Request != nil {
			b, err := json.Marshal(emb.Request)
			if err != nil {
				diags.AddError("Error encoding embedder request JSON", err.Error())
			}
			requestJSON = types.StringValue(string(b))
		} else {
			requestJSON = types.StringNull()
		}
		if emb.Response != nil {
			b, err := json.Marshal(emb.Response)
			if err != nil {
				diags.AddError("Error encoding embedder response JSON", err.Error())
			}
			responseJSON = types.StringValue(string(b))
		} else {
			responseJSON = types.StringNull()
		}

		headers, d := types.MapValueFrom(ctx, types.StringType, emb.Headers)
		diags.Append(d...)

		model := types.StringValue(emb.Model)
		if emb.Model == "" {
			model = types.StringNull()
		}
		apiKey := types.StringValue(emb.APIKey)
		if emb.APIKey == "" {
			apiKey = types.StringNull()
		}
		documentTemplate := types.StringValue(emb.DocumentTemplate)
		if emb.DocumentTemplate == "" {
			documentTemplate = types.StringNull()
		}
		url := types.StringValue(emb.URL)
		if emb.URL == "" {
			url = types.StringNull()
		}
		revision := types.StringValue(emb.Revision)
		if emb.Revision == "" {
			revision = types.StringNull()
		}

		obj, d := types.ObjectValue(embedderAttrTypes, map[string]attr.Value{
			"source":            types.StringValue(emb.Source),
			"model":             model,
			"api_key":           apiKey,
			"document_template": documentTemplate,
			"dimensions":        types.Int64Value(int64(emb.Dimensions)),
			"url":               url,
			"revision":          revision,
			"request":           requestJSON,
			"response":          responseJSON,
			"headers":           headers,
		})
		diags.Append(d...)

		values[name] = obj
	}

	result, d := types.MapValue(objType, values)
	diags.Append(d...)

	return result
}
