package main

import (
	"fmt"
	"io"
	"os"

	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/external"
	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/local"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		writeRootUsage(stderr)
		return 1
	}
	switch args[0] {
	case "-h", "--help":
		writeRootUsage(stdout)
		return 0
	case "local":
		return local.Run(root, args[1:], stdout, stderr)
	case "external":
		return external.Run(root, args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown subcommand %q\n\n", args[0])
		writeRootUsage(stderr)
		return 1
	}
}

func writeRootUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage: link-check <local|external>")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Commands:")
	_, _ = fmt.Fprintln(w, "  local     Check repository-local links and repo-path references")
	_, _ = fmt.Fprintln(w, "  external  Probe external URLs referenced by tracked files")
}
