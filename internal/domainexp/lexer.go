package domainexp

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

var (
	// ErrSingleAnd is triggered by single & (which should have been &&).
	ErrSingleAnd = errors.New(`use "&&" instead of "&"`)

	// ErrSingleOr is triggered by single | (which should have been ||).
	ErrSingleOr = errors.New(`use "||" instead of "|"`)

	// ErrUTF8 is triggered by invalid UTF-8 strings.
	ErrUTF8 = errors.New(`invalid UTF-8 string`)
)

func splitter(data []byte, atEOF bool) (int, []byte, error) {
	reader := bytes.NewReader(data)
	startIndex := 0

	const (
		StateInit  = iota
		StateAnd0  // &&
		StateOr0   // ||
		StateOther // others
	)
	state := StateInit

	returnToken := func() (int, []byte, error) {
		endIndex := len(data) - reader.Len()
		return endIndex, data[startIndex:endIndex], nil
	}

	for reader.Len() > 0 {
		ch, size, err := reader.ReadRune()
		if err != nil {
			return startIndex, nil, fmt.Errorf("reader.ReadRune: %w", err)
		}
		if ch == utf8.RuneError && size == 1 {
			if reader.Len() == 0 && !atEOF {
				// special case: the UTF-8 decoding failed,
				// but reading more bytes might help
				break
			}
			return startIndex, nil, fmt.Errorf("cannot parse %s: %w", data, ErrUTF8)
		}

		switch state {
		case StateInit:
			switch {
			case unicode.IsSpace(ch):
				startIndex += size
			case strings.ContainsRune("(),!", ch):
				return returnToken()
			case ch == '&':
				state = StateAnd0
			case ch == '|':
				state = StateOr0
			default:
				state = StateOther
			}
		case StateAnd0:
			if ch != '&' {
				return 0, nil, ErrSingleAnd
			}
			return returnToken()
		case StateOr0:
			if ch != '|' {
				return 0, nil, ErrSingleOr
			}
			return returnToken()
		case StateOther:
			if unicode.IsSpace(ch) || strings.ContainsRune("(),!&|", ch) {
				if err = reader.UnreadRune(); err != nil {
					return startIndex, nil, fmt.Errorf("reader.UnreadRune: %w", err)
				}

				return returnToken()
			}
		}
	}

	if !atEOF {
		return startIndex, nil, nil
	}

	switch state {
	case StateInit:
		return startIndex, nil, nil
	case StateAnd0:
		return startIndex, nil, ErrSingleAnd
	case StateOr0:
		return startIndex, nil, ErrSingleOr
	default:
		return returnToken()
	}
}

func tokenize(ppfmt pp.PP, key string, input string) ([]string, bool) {
	scanner := bufio.NewScanner(strings.NewReader(input))
	scanner.Split(splitter)

	tokens := []string{}

	for scanner.Scan() {
		tokens = append(tokens, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		ppfmt.Noticef(pp.EmojiUserError, "%s (%q) is ill-formed: %v", key, input, err)
		return nil, false
	}
	return tokens, true
}
