package monitor

import (
	"context"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type Monitor interface {
	DescribeService() string
	DescribeBaseURL() string
	Success(context.Context, pp.PP) bool
	Start(context.Context, pp.PP) bool
	Failure(context.Context, pp.PP) bool
	ExitStatus(context.Context, pp.PP, int) bool
}
