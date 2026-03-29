// Package extract classifies tracked-file content into link and comment data
// for the link-check script.
package extract

import (
	"go/parser"
	"go/token"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
)

var (
	markdownInlineLinkRE = regexp.MustCompile(`(?s)\[[^\]]+\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	markdownRefLinkRE    = regexp.MustCompile(`(?s)\[([^\]]+)\]\[([^\]]*)\]`)
	markdownRefDefRE     = regexp.MustCompile(`(?m)^\[([^\]]+)\]:\s*(\S+)`)
	htmlTargetRE         = regexp.MustCompile(`(?is)<(?:a|img|script|source)\b[^>]*\s(?:href|src)=["']([^"']+)["']`)
	htmlIDRE             = regexp.MustCompile(`(?is)\bid=["']([^"']+)["']`)
	urlRE                = regexp.MustCompile("https?://[^\\s<>)\"'`\\]\\\\]+")
	uriSchemeRE          = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)
)

type lineIndex struct {
	newlineOffsets []int
}

func newLineIndex(text string) lineIndex {
	offsets := make([]int, 0)
	for offset, r := range text {
		if r == '\n' {
			offsets = append(offsets, offset)
		}
	}
	return lineIndex{newlineOffsets: offsets}
}

func (index lineIndex) lineNumberAtOffset(offset int) int {
	return sort.Search(len(index.newlineOffsets), func(i int) bool {
		return index.newlineOffsets[i] >= offset
	}) + 1
}

func collectTextHTTPURLs(text string) []HTTPLinkTarget {
	lineNumbers := newLineIndex(text)
	targets := make([]HTTPLinkTarget, 0, len(urlRE.FindAllStringIndex(text, -1)))
	for _, match := range urlRE.FindAllStringIndex(text, -1) {
		targets = append(targets, HTTPLinkTarget{
			URL:  strings.TrimRight(text[match[0]:match[1]], ".,:;"),
			Line: lineNumbers.lineNumberAtOffset(match[0]),
		})
	}
	return targets
}

func collectMarkdownLinks(text string) FileLinks {
	lineNumbers := newLineIndex(text)
	targets := make([]rawTarget, 0)
	for _, match := range markdownInlineLinkRE.FindAllStringSubmatchIndex(text, -1) {
		if match[0] > 0 && text[match[0]-1] == '!' {
			continue
		}
		targets = append(targets, rawTarget{
			Target: text[match[2]:match[3]],
			Line:   lineNumbers.lineNumberAtOffset(match[2]),
		})
	}
	refTargets := map[string]rawTarget{}
	for _, match := range markdownRefDefRE.FindAllStringSubmatchIndex(text, -1) {
		label := strings.ToLower(strings.TrimSpace(text[match[2]:match[3]]))
		refTargets[label] = rawTarget{
			Target: text[match[4]:match[5]],
			Line:   lineNumbers.lineNumberAtOffset(match[4]),
		}
	}
	for _, match := range markdownRefLinkRE.FindAllStringSubmatchIndex(text, -1) {
		if match[0] > 0 && text[match[0]-1] == '!' {
			continue
		}
		label := strings.ToLower(strings.TrimSpace(text[match[2]:match[3]]))
		ref := strings.ToLower(strings.TrimSpace(text[match[4]:match[5]]))
		if ref == "" {
			ref = label
		}
		target, ok := refTargets[ref]
		if ok {
			targets = append(targets, target)
		}
	}
	for _, match := range htmlTargetRE.FindAllStringSubmatchIndex(text, -1) {
		targets = append(targets, rawTarget{
			Target: text[match[2]:match[3]],
			Line:   lineNumbers.lineNumberAtOffset(match[2]),
		})
	}

	links := FileLinks{
		LocalReferences: make([]LocalReference, 0),
		HTTPLinks:       make([]HTTPLinkTarget, 0),
	}
	for _, target := range targets {
		switch {
		case isHTTPURL(target.Target):
			links.HTTPLinks = append(links.HTTPLinks, HTTPLinkTarget{
				URL:  target.Target,
				Line: target.Line,
			})
		case hasURIScheme(target.Target):
			continue
		default:
			links.LocalReferences = append(links.LocalReferences, LocalReference(target))
		}
	}
	return links
}

func extractCommentBlocks(file, text string) []string {
	switch path.Ext(file) {
	case ".go":
		return extractGoCommentBlocks(file, text)
	case ".yaml", ".yml":
		blocks := make([]string, 0)
		for line := range strings.SplitSeq(text, "\n") {
			trimmed := strings.TrimLeft(line, " \t")
			if strings.HasPrefix(trimmed, "#") {
				blocks = append(blocks, trimmed)
			}
		}
		return blocks
	default:
		return nil
	}
}

// extractGoCommentBlocks uses the Go parser so repository-path checks can see
// only actual Go comments rather than string literals or code tokens.
func extractGoCommentBlocks(file, text string) []string {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, file, text, parser.ParseComments|parser.AllErrors)
	if parsed != nil {
		blocks := make([]string, 0, len(parsed.Comments))
		for _, group := range parsed.Comments {
			blocks = append(blocks, group.Text())
		}
		return blocks
	}
	if err != nil {
		return nil
	}
	return nil
}

func isMarkdownFile(file string) bool {
	switch path.Ext(file) {
	case ".md", ".markdown":
		return true
	default:
		return false
	}
}

// isHTTPURL reports whether a target is an HTTP or HTTPS URL that this tool
// can extract as a probeable external link.
func isHTTPURL(target string) bool {
	parsed, err := url.Parse(target)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func hasURIScheme(target string) bool {
	return uriSchemeRE.MatchString(target)
}

// Extract returns the pre-classified link and comment data extracted from one
// tracked file. File-type detection is intentionally private to this package so
// callers only depend on the extracted results, not on extraction strategy.
func Extract(file, text string) FileLinks {
	links := FileLinks{
		HTTPLinks:       collectTextHTTPURLs(text),
		CommentBlocks:   extractCommentBlocks(file, text),
		LocalReferences: nil,
	}
	if !isMarkdownFile(file) {
		return links
	}
	markdownLinks := collectMarkdownLinks(text)
	links.LocalReferences = markdownLinks.LocalReferences
	links.HTTPLinks = append(links.HTTPLinks, markdownLinks.HTTPLinks...)
	return links
}

// LocalTargets returns the fragment destinations exposed by one tracked file.
// Unsupported file types simply report that fragment validation is unavailable.
func LocalTargets(file, text string) LocalTargetInfo {
	if !isMarkdownFile(file) {
		return LocalTargetInfo{}
	}
	return LocalTargetInfo{
		SupportsFragments: true,
		Fragments:         markdownExplicitIDs(text),
	}
}

// markdownExplicitIDs returns explicit HTML id attributes from one Markdown
// file. Renderer-generated heading ids are intentionally excluded because they
// are renderer-dependent.
func markdownExplicitIDs(text string) map[string]bool {
	ids := map[string]bool{}
	for _, match := range htmlIDRE.FindAllStringSubmatch(text, -1) {
		ids[match[1]] = true
	}
	return ids
}
