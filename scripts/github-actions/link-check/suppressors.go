package main

import (
	"net/url"
	"strings"
)

type externalWarningSuppressor func(probeResult) bool

var externalWarningSuppressors = []externalWarningSuppressor{
	suppressTrailingSlashRedirectWarning,
}

func shouldSuppressExternalWarning(result probeResult) bool {
	for _, suppressor := range externalWarningSuppressors {
		if suppressor(result) {
			return true
		}
	}
	return false
}

func suppressTrailingSlashRedirectWarning(result probeResult) bool {
	if len(result.Hops) != 2 {
		return false
	}
	first := result.Hops[0]
	second := result.Hops[1]
	if !redirectStatusCodes[first.Status] || second.Status == 0 {
		return false
	}

	from, err := url.Parse(first.URL)
	if err != nil {
		return false
	}
	to, err := url.Parse(second.URL)
	if err != nil {
		return false
	}
	if from.Scheme != to.Scheme || from.Host != to.Host {
		return false
	}
	if from.RawQuery != to.RawQuery || from.Fragment != to.Fragment {
		return false
	}
	if !pathsDifferOnlyByTrailingSlash(from.Path, to.Path) {
		return false
	}
	return true
}

func pathsDifferOnlyByTrailingSlash(fromPath, toPath string) bool {
	return toPath == fromPath+"/" || (strings.HasSuffix(fromPath, "/") && strings.TrimSuffix(fromPath, "/") == toPath)
}
