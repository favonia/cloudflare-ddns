// Package tags provides helpers for Cloudflare DNS record tags.
//
// The helpers here implement Cloudflare-shaped tag semantics shared by
// API-side verification and setter-side reconciliation. The expected snapshot
// for these semantics was adopted on 2026-03-22; update that date only when
// scripts/github-actions/cloudflare-doc-watch/config/dns-record-attributes.json changes:
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

// tagSetsSummary describes the canonical relationship across multiple tag sets.
type tagSetsSummary struct {
	representative        map[string]string
	occurrence            map[string]int
	setCount              int
	hasAmbiguousCanonical bool
	hasDuplicateCanonical bool
}

// summarizeSets analyzes tag sets by Cloudflare tag semantics.
func summarizeSets(tagSets [][]string) tagSetsSummary {
	summary := tagSetsSummary{
		representative:        make(map[string]string),
		occurrence:            make(map[string]int),
		setCount:              len(tagSets),
		hasAmbiguousCanonical: false,
		hasDuplicateCanonical: false,
	}
	var firstCanonicalKeys []string
	for i, tags := range tagSets {
		canonical := canonicalize(tags)
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
			if existing, exists := summary.representative[key]; !exists || value < existing {
				summary.representative[key] = value
			}
		}
	}
	return summary
}

// Resolved describes the canonical reconciliation of tags across multiple tag
// sets.
type Resolved struct {
	Inherited             []string
	Dropped               []string
	HasAmbiguousCanonical bool
	HasDuplicateCanonical bool
}

// Resolve computes the canonical reconciliation result across tag sets.
// When no configured fallback tag set is merged in, only tags present in every
// input set survive in Inherited and the remaining canonical tags are reported
// in Dropped. This is the DNS tag-specific instantiation of the managed-record
// reconciliation rule from docs/designs/features/managed-record-ownership.markdown.
func Resolve(tags [][]string) Resolved {
	summary := summarizeSets(tags)
	resolved := Resolved{
		Inherited:             nil,
		Dropped:               nil,
		HasAmbiguousCanonical: summary.hasAmbiguousCanonical,
		HasDuplicateCanonical: summary.hasDuplicateCanonical,
	}

	inheritedKeys := make([]string, 0, len(summary.representative))
	droppedKeys := make([]string, 0, len(summary.representative))
	for key := range summary.representative {
		if summary.occurrence[key] == summary.setCount {
			inheritedKeys = append(inheritedKeys, key)
		} else {
			droppedKeys = append(droppedKeys, key)
		}
	}
	if len(inheritedKeys) > 0 {
		slices.Sort(inheritedKeys)
		resolved.Inherited = make([]string, 0, len(inheritedKeys))
		for _, key := range inheritedKeys {
			resolved.Inherited = append(resolved.Inherited, summary.representative[key])
		}
	}
	if len(droppedKeys) > 0 {
		slices.Sort(droppedKeys)
		resolved.Dropped = make([]string, 0, len(droppedKeys))
		for _, key := range droppedKeys {
			resolved.Dropped = append(resolved.Dropped, summary.representative[key])
		}
	}
	return resolved
}

// Equal reports whether two tag sets have the same canonical tag keys.
func Equal(left, right []string) bool {
	leftCanonical := canonicalize(left)
	rightCanonical := canonicalize(right)
	return slices.Equal(leftCanonical.keys, rightCanonical.keys)
}
