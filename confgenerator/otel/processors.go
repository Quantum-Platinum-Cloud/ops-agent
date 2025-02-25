// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otel

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// Helper functions to easily build up processor configs.

// MetricsFilter returns a Component that filters metrics.
// polarity should be "include" or "exclude".
// matchType should be "strict" or "regexp".
func MetricsFilter(polarity, matchType string, metricNames ...string) Component {
	return Component{
		Type: "filter",
		Config: map[string]interface{}{
			"metrics": map[string]interface{}{
				polarity: map[string]interface{}{
					"match_type":   matchType,
					"metric_names": metricNames,
				},
			},
		},
	}
}

// MetricsTransform returns a Component that performs the transformations specified as arguments.
func MetricsTransform(metrics ...map[string]interface{}) Component {
	return Component{
		Type: "metricstransform",
		Config: map[string]interface{}{
			"transforms": metrics,
		},
	}
}

// NormalizeSums returns a Component that performs counter normalization.
func NormalizeSums() Component {
	return Component{
		Type:   "normalizesums",
		Config: map[string]interface{}{},
	}
}

// CastToSum returns a Component that performs a cast of each metric to a sum.
func CastToSum(metrics ...string) Component {
	return Component{
		Type: "casttosum",
		Config: map[string]interface{}{
			"metrics": metrics,
		},
	}
}

// AddPrefix returns a config snippet that adds a domain prefix to all metrics.
func AddPrefix(prefix string, operations ...map[string]interface{}) map[string]interface{} {
	return RegexpRename(
		`^(.*)$`,
		path.Join(prefix, `${1}`),
		operations...,
	)
}

// ChangePrefix returns a config snippet that updates a prefix on all metrics.
func ChangePrefix(oldPrefix, newPrefix string) map[string]interface{} {
	return RegexpRename(
		fmt.Sprintf(`^%s(.*)$`, oldPrefix),
		fmt.Sprintf("%s%s", newPrefix, `${1}`),
	)
}

// RegexpRename returns a config snippet that renames metrics matching the given regexp.
// The `rename` argument supports capture groups as `${1}`, `${2}`, and so on.
func RegexpRename(regexp string, rename string, operations ...map[string]interface{}) map[string]interface{} {
	// $ needs to be escaped because reasons.
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/metricstransformprocessor#rename-multiple-metrics-using-substitution
	out := map[string]interface{}{
		"include":    strings.ReplaceAll(regexp, "$", "$$"),
		"match_type": "regexp",
		"action":     "update",
		"new_name":   strings.ReplaceAll(rename, "$", "$$"),
	}

	if len(operations) > 0 {
		out["operations"] = operations
	}

	return out
}

// TransformationMetrics returns a transform processor object that contains all the queries passed into it.
func TransformationMetrics(queries ...TransformQuery) Component {
	queryStrings := []string{}
	for _, q := range queries {
		queryStrings = append(queryStrings, string(q))
	}
	return Component{
		Type: "transform",
		Config: map[string]map[string]interface{}{
			"metrics": {
				"queries": queryStrings,
			},
		},
	}
}

// TransformQuery is a type wrapper for query expressions supported by the transform
// processor found here: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/transformprocessor
type TransformQuery string

// FlattenResourceAttribute returns an expression that brings down a resource attribute to a
// metric attribute.
func FlattenResourceAttribute(resourceAttribute, metricAttribute string) TransformQuery {
	return TransformQuery(fmt.Sprintf(`set(attributes["%s"], resource.attributes["%s"])`, metricAttribute, resourceAttribute))
}

// ConvertGaugeToSum returns an expression where a gauge metric can be converted into a sum
func ConvertGaugeToSum(metricName string) TransformQuery {
	return TransformQuery(fmt.Sprintf(`convert_gauge_to_sum("cumulative", true) where metric.name == "%s"`, metricName))
}

// SetDescription returns a metrics transform expression where the metrics description will be set to what is provided
func SetDescription(metricName, metricDescription string) TransformQuery {
	return TransformQuery(fmt.Sprintf(`set(metric.description, "%s") where metric.name == "%s"`, metricDescription, metricName))
}

// SetUnit returns a metrics transform expression where the metric unit is set to provided value
func SetUnit(metricName, unit string) TransformQuery {
	return TransformQuery(fmt.Sprintf(`set(metric.unit, "%s") where metric.name == "%s"`, unit, metricName))
}

// SetName returns a metrics transform expression where the metric name is set to provided value
func SetName(oldName, newName string) TransformQuery {
	return TransformQuery(fmt.Sprintf(`set(metric.name, "%s") where metric.name == "%s"`, newName, oldName))
}

func SetAttribute(metricName, attributeKey, attributeValue string) TransformQuery {
	return TransformQuery(fmt.Sprintf(`set(attributes["%s"], "%s") where metric.name == "%s"`, attributeKey, attributeValue, metricName))
}

// SummarySumValToSum creates a new Sum metric out of a summary metric's sum value. The new metric has a name of "<Old Name>_sum".
func SummarySumValToSum(metricName, aggregation string, isMonotonic bool) TransformQuery {
	return TransformQuery(fmt.Sprintf(`convert_summary_sum_val_to_sum("%s",  %t) where metric.name == "%s"`, aggregation, isMonotonic, metricName))
}

// SummaryCountValToSum creates a new Sum metric out of a summary metric's count value. The new metric has a name of "<Old Name>_count".
func SummaryCountValToSum(metricName, aggregation string, isMonotonic bool) TransformQuery {
	return TransformQuery(fmt.Sprintf(`convert_summary_count_val_to_sum("%s",  %t) where metric.name == "%s"`, aggregation, isMonotonic, metricName))
}

// RetainResource retains the resource attributes provided, and drops all other attributes.
func RetainResource(resourceAttributeKeys ...string) TransformQuery {
	keyList := ""
	for _, key := range resourceAttributeKeys {
		keyList += fmt.Sprintf(`, "%s"`, key)
	}
	return TransformQuery(fmt.Sprintf(`keep_keys(resource.attributes%s)`, keyList))
}

// RenameMetric returns a config snippet that renames old to new, applying zero or more transformations.
func RenameMetric(old, new string, operations ...map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{
		"include":  old,
		"action":   "update",
		"new_name": new,
	}
	if len(operations) > 0 {
		out["operations"] = operations
	}
	return out
}

// UpdateMetric returns a config snippet applies transformations to the given metric name
func UpdateMetric(metric string, operations ...map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{
		"include": metric,
		"action":  "update",
	}
	if len(operations) > 0 {
		out["operations"] = operations
	}
	return out
}

// UpdateMetricRegexp returns a config snippet that applies transformations to metrics matching
// the input regex
func UpdateMetricRegexp(metricRegex string, operations ...map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{
		"include":    metricRegex,
		"match_type": "regexp",
		"action":     "update",
	}
	if len(operations) > 0 {
		out["operations"] = operations
	}
	return out
}

// DuplicateMetric returns a config snippet that copies old to new, applying zero or more transformations.
func DuplicateMetric(old, new string, operations ...map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{
		"include":  old,
		"action":   "insert",
		"new_name": new,
	}
	if len(operations) > 0 {
		out["operations"] = operations
	}
	return out
}

// CombineMetrics returns a config snippet that renames metrics matching the regex old to new, applying zero or more transformations.
func CombineMetrics(old, new string, operations ...map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{
		"include":       old,
		"match_type":    "regexp",
		"action":        "combine",
		"new_name":      new,
		"submatch_case": "lower",
	}
	if len(operations) > 0 {
		out["operations"] = operations
	}
	return out
}

// ToggleScalarDataType transforms int -> double and double -> int.
var ToggleScalarDataType = map[string]interface{}{"action": "toggle_scalar_data_type"}

// AddLabel adds a label with a fixed value.
func AddLabel(key, value string) map[string]interface{} {
	return map[string]interface{}{
		"action":    "add_label",
		"new_label": key,
		"new_value": value,
	}
}

// RenameLabel renames old to new
func RenameLabel(old, new string) map[string]interface{} {
	return map[string]interface{}{
		"action":    "update_label",
		"label":     old,
		"new_label": new,
	}
}

// RenameLabelValues renames label values
func RenameLabelValues(label string, transforms map[string]string) map[string]interface{} {
	var actions []map[string]string
	for old, new := range transforms {
		actions = append(actions, map[string]string{
			"value":     old,
			"new_value": new,
		})
	}
	sort.Slice(actions, func(i, j int) bool {
		return actions[i]["value"] < actions[j]["value"]
	})
	return map[string]interface{}{
		"action":        "update_label",
		"label":         label,
		"value_actions": actions,
	}
}

// DeleteLabelValue removes streams with the given label value
func DeleteLabelValue(label, value string) map[string]interface{} {
	return map[string]interface{}{
		"action":      "delete_label_value",
		"label":       label,
		"label_value": value,
	}
}

// ScaleValue multiplies the value by factor
func ScaleValue(factor float64) map[string]interface{} {
	return map[string]interface{}{
		"action":             "experimental_scale_value",
		"experimental_scale": factor,
	}
}

// AggregateLabels removes all labels except those in the passed list, aggregating values using aggregationType.
func AggregateLabels(aggregationType string, labels ...string) map[string]interface{} {
	return map[string]interface{}{
		"action":           "aggregate_labels",
		"label_set":        labels,
		"aggregation_type": aggregationType,
	}
}

// AggregateLabelValues combines the given values into a single value
func AggregateLabelValues(aggregationType string, label string, new string, old ...string) map[string]interface{} {
	return map[string]interface{}{
		"action":            "aggregate_label_values",
		"aggregation_type":  aggregationType,
		"label":             label,
		"new_value":         new,
		"aggregated_values": old,
	}
}

// CondenseResourceMetrics merges multiple resource metrics on
// a slice of metrics to a single resource metrics payload, if they have the same
// resource.
func CondenseResourceMetrics() Component {
	return Component{
		Type:   "groupbyattrs",
		Config: map[string]any{},
	}
}
