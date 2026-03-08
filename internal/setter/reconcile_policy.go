package setter

import (
	"maps"
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

type canonicalTagSet struct {
	keys                  []string
	representative        map[string]string
	hasDuplicateCanonical bool
}

func canonicalizeTagSet(tags []string) canonicalTagSet {
	valuesByKey := make(map[string]string, len(tags))
	duplicateCanonical := false
	for _, tag := range tags {
		key := canonicalTagKey(tag)
		if existing, exists := valuesByKey[key]; exists {
			duplicateCanonical = true
			if tag < existing {
				valuesByKey[key] = tag
			}
		} else {
			valuesByKey[key] = tag
		}
	}
	keys := slices.Collect(maps.Keys(valuesByKey))
	slices.Sort(keys)
	return canonicalTagSet{
		keys:                  keys,
		representative:        valuesByKey,
		hasDuplicateCanonical: duplicateCanonical,
	}
}

type tagSetSummary struct {
	representative        map[string]string
	occurrence            map[string]int
	setCount              int
	hasAmbiguousCanonical bool
	hasDuplicateCanonical bool
}

func summarizeTagSets(tagSets [][]string) tagSetSummary {
	summary := tagSetSummary{
		representative:        make(map[string]string),
		occurrence:            make(map[string]int),
		setCount:              len(tagSets),
		hasAmbiguousCanonical: false,
		hasDuplicateCanonical: false,
	}
	var firstCanonicalKeys []string
	for i, tags := range tagSets {
		canonical := canonicalizeTagSet(tags)
		if i == 0 {
			firstCanonicalKeys = canonical.keys
		} else if !slices.Equal(firstCanonicalKeys, canonical.keys) {
			summary.hasAmbiguousCanonical = true
		}
		if canonical.hasDuplicateCanonical {
			summary.hasDuplicateCanonical = true
		}
		for _, key := range canonical.keys {
			value := canonical.representative[key]
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
	leftCanonical := canonicalizeTagSet(left)
	rightCanonical := canonicalizeTagSet(right)
	return slices.Equal(leftCanonical.keys, rightCanonical.keys)
}
