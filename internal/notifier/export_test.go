package notifier

import "github.com/favonia/cloudflare-ddns/internal/pp"

//nolint:gochecknoglobals // Test-only shim that re-exports the unexported helper under its old name.
var DescribeShoutrrrService func(pp.PP, string) string = describeShoutrrrService
