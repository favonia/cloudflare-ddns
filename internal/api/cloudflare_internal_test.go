package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDescribeFreeFormString(t *testing.T) {
	t.Parallel()

	require.Equal(t, "empty", describeFreeFormString(""))
	require.Equal(t, `"hello"`, describeFreeFormString("hello"))
}
