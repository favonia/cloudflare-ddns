package setter

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
