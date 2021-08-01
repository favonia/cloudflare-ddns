package api_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/favonia/cloudflare-ddns-go/internal/api"
)

func TestTTLDescribe(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		seconds     int
		description string
	}{
		"automatic": {1, "1 (automatic)"},
		"2":         {2, "2"},
		"30":        {30, "30"},
		"293":       {293, "293"},
		"842":       {842, "842"},
		"37284789":  {37284789, "37284789"},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.description, api.TTL(tc.seconds).Describe())
		})
	}
}

func TestTTLString(t *testing.T) {
	t.Parallel()
	for _, tc := range [...]struct {
		seconds int
		str     string
	}{
		{1, "1"},
		{2, "2"},
		{30, "30"},
		{293, "293"},
		{842, "842"},
		{37284789, "37284789"},
	} {
		tc := tc
		t.Run(tc.str, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.str, api.TTL(tc.seconds).String())
		})
	}
}

func TestTTLInt(t *testing.T) {
	t.Parallel()
	for _, i := range [...]int{
		1, 2, 30, 293, 842, 37284789,
	} {
		i := i
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, i, api.TTL(i).Int())
		})
	}
}
