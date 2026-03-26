package heartbeat_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
)

func TestMergeMessages(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		messages []heartbeat.Message
		want     heartbeat.Message
	}{
		"empty": {
			messages: nil,
			want:     heartbeat.NewMessage(),
		},
		"all-success": {
			messages: []heartbeat.Message{
				heartbeat.NewMessagef(true, "hello"),
				heartbeat.NewMessagef(true, "world"),
			},
			want: heartbeat.Message{
				OK:    true,
				Lines: []string{"hello", "world"},
			},
		},
		"failure-wins": {
			messages: []heartbeat.Message{
				heartbeat.NewMessagef(true, "ignored"),
				heartbeat.NewMessagef(false, "first failure"),
				heartbeat.NewMessagef(false, "second failure"),
			},
			want: heartbeat.Message{
				OK:    false,
				Lines: []string{"first failure", "second failure"},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, heartbeat.MergeMessages(tc.messages...))
		})
	}
}
