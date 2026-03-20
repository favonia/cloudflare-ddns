// Package tags provides helpers for Cloudflare DNS record tags.
//
// The helpers here implement Cloudflare-shaped tag semantics shared by
// API-side verification and setter-side reconciliation:
//   - tag names are canonicalized case-insensitively
//   - tag values remain case-sensitive
//   - representation-only differences such as order or duplicate canonical forms
//     do not change semantic equality
package tags

import (
	"maps"
	"slices"
	"strings"
)

func canonicalKey(tag string) string {
	name, value, hasValue := strings.Cut(tag, ":")
	if !hasValue {
		return strings.ToLower(tag)
	}
	return strings.ToLower(name) + ":" + value
}

// Undocumented returns tags that are not in Cloudflare's documented stored
// name:value form. Empty values are allowed, but missing separators and empty
// names are treated as undocumented.
func Undocumented(tags []string) []string {
	var undocumented []string
	for _, tag := range tags {
		name, _, hasValue := strings.Cut(tag, ":")
		if !hasValue || name == "" {
			undocumented = append(undocumented, tag)
		}
	}
	return undocumented
}

type canonicalSet struct {
	keys                  []string
	representative        map[string]string
	hasDuplicateCanonical bool
}

func canonicalize(tags []string) canonicalSet {
	valuesByKey := make(map[string]string, len(tags))
	duplicateCanonical := false
	for _, tag := range tags {
		key := canonicalKey(tag)
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
	return canonicalSet{
		keys:                  keys,
		representative:        valuesByKey,
		hasDuplicateCanonical: duplicateCanonical,
	}
}

// Summary describes the canonical relationship across multiple tag sets.
type Summary struct {
	Representative        map[string]string
	Occurrence            map[string]int
	SetCount              int
	HasAmbiguousCanonical bool
	HasDuplicateCanonical bool
}

// SummarizeSets analyzes tag sets by Cloudflare tag semantics.
func SummarizeSets(tagSets [][]string) Summary {
	summary := Summary{
		Representative:        make(map[string]string),
		Occurrence:            make(map[string]int),
		SetCount:              len(tagSets),
		HasAmbiguousCanonical: false,
		HasDuplicateCanonical: false,
	}
	var firstCanonicalKeys []string
	for i, tags := range tagSets {
		canonical := canonicalize(tags)
		if i == 0 {
			firstCanonicalKeys = canonical.keys
		} else if !slices.Equal(firstCanonicalKeys, canonical.keys) {
			summary.HasAmbiguousCanonical = true
		}
		if canonical.hasDuplicateCanonical {
			summary.HasDuplicateCanonical = true
		}
		for _, key := range canonical.keys {
			value := canonical.representative[key]
			summary.Occurrence[key]++
			if existing, ok := summary.Representative[key]; !ok || value < existing {
				summary.Representative[key] = value
			}
		}
	}
	return summary
}

// CommonSubset computes the greatest canonical subset of tags across tag sets.
//
// This is the tag reconciliation result used when there is no configured
// fallback tag set to merge in: only tags present in every input set survive.
func CommonSubset(tagSets [][]string) []string {
	if len(tagSets) == 0 {
		return nil
	}

	summary := SummarizeSets(tagSets)

	inheritedKeys := make([]string, 0, len(summary.Representative))
	for key := range summary.Representative {
		if summary.Occurrence[key] == summary.SetCount {
			inheritedKeys = append(inheritedKeys, key)
		}
	}
	if len(inheritedKeys) == 0 {
		return nil
	}
	slices.Sort(inheritedKeys)
	resolved := make([]string, 0, len(inheritedKeys))
	for _, key := range inheritedKeys {
		resolved = append(resolved, summary.Representative[key])
	}
	return resolved
}

// Equal reports whether two tag sets have the same canonical tag keys.
func Equal(left, right []string) bool {
	leftCanonical := canonicalize(left)
	rightCanonical := canonicalize(right)
	return slices.Equal(leftCanonical.keys, rightCanonical.keys)
}
