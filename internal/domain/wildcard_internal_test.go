package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateNormalizedWildcardSuffixLeavesTargetPolicyToCaller(t *testing.T) {
	t.Parallel()

	wildcard, err := validateNormalizedWildcardSuffix("")
	require.NoError(t, err)
	require.Equal(t, Wildcard(""), wildcard)
}
