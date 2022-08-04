package monitor

import (
	"context"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//go:generate mockgen -destination=../mocks/mock_monitor.go -package=mocks . Monitor

type Monitor interface {
	DescribeService() string
	Success(context.Context, pp.PP) bool
	Start(context.Context, pp.PP) bool
	Failure(context.Context, pp.PP) bool
	ExitStatus(context.Context, pp.PP, int) bool
}
