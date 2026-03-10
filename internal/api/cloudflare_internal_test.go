package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDescribeFreeFormString(t *testing.T) {
	t.Parallel()

	require.Equal(t, "empty", DescribeFreeFormString(""))
	require.Equal(t, `"hello"`, DescribeFreeFormString("hello"))
}
