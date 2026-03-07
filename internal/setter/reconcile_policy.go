package setter

import (
	"slices"
	"strings"
)

// resolveScalarValue resolves a scalar value from stale sources.
// The returned bool is true when stale values are non-empty and disagree.
func resolveScalarValue[T comparable](configured T, staleValues []T) (T, bool) {
	if len(staleValues) == 0 {
		return configured, false
	}

	candidate := staleValues[0]
	for _, value := range staleValues[1:] {
		if value != candidate {
			return configured, true
		}
	}
	return candidate, false
}

func canonicalTagKey(tag string) string {
	name, value, hasValue := strings.Cut(tag, ":")
	if !hasValue {
		return strings.ToLower(tag)
	}
	return strings.ToLower(name) + ":" + value
}

func normalizeTagsDeterministic(tags []string) map[string]string {
	valuesByKey := make(map[string]string, len(tags))
	for _, tag := range tags {
		key := canonicalTagKey(tag)
		if existing, exists := valuesByKey[key]; !exists || tag < existing {
			valuesByKey[key] = tag
		}
	}
	return valuesByKey
}

type tagSetSummary struct {
	representative map[string]string
	occurrence     map[string]int
	setCount       int
}

func summarizeTagSets(tagSets [][]string) tagSetSummary {
	summary := tagSetSummary{
		representative: make(map[string]string),
		occurrence:     make(map[string]int),
		setCount:       len(tagSets),
	}
	for _, tags := range tagSets {
		normalized := normalizeTagsDeterministic(tags)
		for key, value := range normalized {
			summary.occurrence[key]++
			if existing, ok := summary.representative[key]; !ok || value < existing {
				summary.representative[key] = value
			}
		}
	}
	return summary
}

// commonTags computes the greatest common subset of tags across stale sets.
func commonTags(staleTagSets [][]string) []string {
	if len(staleTagSets) == 0 {
		return nil
	}

	summary := summarizeTagSets(staleTagSets)

	inheritedKeys := make([]string, 0, len(summary.representative))
	for key := range summary.representative {
		if summary.occurrence[key] == summary.setCount {
			inheritedKeys = append(inheritedKeys, key)
		}
	}
	if len(inheritedKeys) == 0 {
		return nil
	}
	slices.Sort(inheritedKeys)
	resolved := make([]string, 0, len(inheritedKeys))
	for _, key := range inheritedKeys {
		resolved = append(resolved, summary.representative[key])
	}
	return resolved
}

func sameTagsByPolicy(left, right []string) bool {
	leftNorm := normalizeTagsDeterministic(left)
	rightNorm := normalizeTagsDeterministic(right)
	if len(leftNorm) != len(rightNorm) {
		return false
	}
	for key := range leftNorm {
		if _, ok := rightNorm[key]; !ok {
			return false
		}
	}
	return true
}
