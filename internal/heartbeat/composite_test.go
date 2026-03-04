package heartbeat_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
)

func TestComposedDescribe(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)

	ms1 := make([]heartbeat.BasicHeartbeat, 0, 3)
	for range 3 {
		m := mocks.NewMockHeartbeat(mockCtrl)
		m.EXPECT().Describe(gomock.Any()).DoAndReturn(
			func(yield func(string, string) bool) {
				yield("name", "params")
			},
		)
		ms1 = append(ms1, m)
	}
	ms2 := make([]heartbeat.BasicHeartbeat, 0, 2)
	for range 2 {
		m := mocks.NewMockHeartbeat(mockCtrl)
		ms2 = append(ms2, m)
	}
	c := heartbeat.NewComposed(heartbeat.NewComposed(ms1...), heartbeat.NewComposed(ms2...))

	count := 0
	for range c.Describe {
		count++
		if count >= 3 {
			break
		}
	}
	require.Equal(t, 3, count)
}

func TestNewComposed(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)

	first := mocks.NewMockBasicHeartbeat(mockCtrl)
	second := mocks.NewMockBasicHeartbeat(mockCtrl)
	third := mocks.NewMockBasicHeartbeat(mockCtrl)

	var missing heartbeat.BasicHeartbeat

	composed := heartbeat.NewComposed(
		missing,
		first,
		heartbeat.NewComposed(second, nil, third),
		nil,
	)

	require.Equal(t, heartbeat.Composed{first, second, third}, composed)
}

func TestComposedPing(t *testing.T) {
	t.Parallel()

	for name1, tc1 := range map[string]struct {
		lines []string
	}{
		"nil":   {nil},
		"empty": {[]string{}},
		"one":   {[]string{"hi"}},
		"two":   {[]string{"hi", "hey"}},
	} {
		for name2, tc2 := range map[string]struct {
			ok bool
		}{
			"ok":     {true},
			"not-ok": {false},
		} {
			t.Run(fmt.Sprintf("%s/%s", name1, name2), func(t *testing.T) {
				t.Parallel()

				ms := make([]heartbeat.BasicHeartbeat, 0, 5)
				mockCtrl := gomock.NewController(t)
				mockPP := mocks.NewMockPP(mockCtrl)

				msg := heartbeat.Message{
					OK:    tc2.ok,
					Lines: tc1.lines,
				}

				for range 5 {
					m := mocks.NewMockHeartbeat(mockCtrl)
					m.EXPECT().Ping(context.Background(), mockPP, msg).Return(true)
					ms = append(ms, m)
				}

				ok := heartbeat.NewComposed(ms...).Ping(context.Background(), mockPP, msg)
				require.True(t, ok)
			})
		}
	}
}

func TestComposedStart(t *testing.T) {
	t.Parallel()

	ms := make([]heartbeat.BasicHeartbeat, 0, 5)

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	message := "你好"

	for range 5 {
		m := mocks.NewMockHeartbeat(mockCtrl)
		m.EXPECT().Start(context.Background(), mockPP, message).Return(true)
		ms = append(ms, m)
	}

	ok := heartbeat.NewComposed(ms...).Start(context.Background(), mockPP, message)
	require.True(t, ok)
}

func TestComposedExit(t *testing.T) {
	t.Parallel()

	ms := make([]heartbeat.BasicHeartbeat, 0, 5)

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	message := "bye!"

	for range 5 {
		m := mocks.NewMockHeartbeat(mockCtrl)
		m.EXPECT().Exit(context.Background(), mockPP, message).Return(true)
		ms = append(ms, m)
	}

	ok := heartbeat.NewComposed(ms...).Exit(context.Background(), mockPP, message)
	require.True(t, ok)
}

func TestComposedLog(t *testing.T) {
	t.Parallel()

	for name1, tc1 := range map[string]struct {
		lines []string
	}{
		"nil":   {nil},
		"empty": {[]string{}},
		"one":   {[]string{"hi"}},
		"two":   {[]string{"hi", "hey"}},
	} {
		for name2, tc2 := range map[string]struct {
			ok bool
		}{
			"ok":     {true},
			"not-ok": {false},
		} {
			t.Run(fmt.Sprintf("%s/%s", name1, name2), func(t *testing.T) {
				t.Parallel()

				ms := make([]heartbeat.BasicHeartbeat, 0, 5)
				mockCtrl := gomock.NewController(t)
				mockPP := mocks.NewMockPP(mockCtrl)

				msg := heartbeat.Message{
					OK:    tc2.ok,
					Lines: tc1.lines,
				}

				for range 3 {
					m := mocks.NewMockHeartbeat(mockCtrl)
					m.EXPECT().Log(context.Background(), mockPP, msg).Return(true)
					ms = append(ms, m)
				}
				for range 2 {
					m := mocks.NewMockBasicHeartbeat(mockCtrl)
					if !tc2.ok {
						m.EXPECT().Ping(context.Background(), mockPP, msg).Return(true)
					}
					ms = append(ms, m)
				}

				ok := heartbeat.NewComposed(ms...).Log(context.Background(), mockPP, msg)
				require.True(t, ok)
			})
		}
	}
}
