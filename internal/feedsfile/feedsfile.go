// Package feedsfile reads feeds.yaml. It supports only the file's own
// schema (a top-level "feeds:" list of flat string maps), so it needs no
// YAML dependency. Anything else is rejected with a line-numbered error.
package feedsfile

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Feed is one entry of feeds.yaml.
type Feed struct {
	Title, URL, Site, Topic, Kind, Added, Why string
}

// Load reads and parses the file at path.
func Load(path string) ([]Feed, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}

// Parse parses feeds.yaml content.
func Parse(r io.Reader) ([]Feed, error) {
	var (
		feeds     []Feed
		cur       *Feed
		curLine   int
		seenFeeds bool
		emptyList bool
		n         int
	)
	errf := func(line int, format string, a ...any) error {
		return fmt.Errorf("feeds.yaml:%d: "+format, append([]any{line}, a...)...)
	}
	finish := func() error {
		if cur == nil {
			return nil
		}
		if cur.URL == "" {
			return errf(curLine, "feed without url")
		}
		feeds = append(feeds, *cur)
		cur = nil
		return nil
	}

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		n++
		raw := strings.TrimRight(sc.Text(), " \t\r")
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " "))

		if !seenFeeds {
			if indent == 0 && strings.Join(strings.Fields(trimmed), " ") == "feeds: []" {
				seenFeeds, emptyList = true, true
				continue
			}
			key, val, err := splitKV(trimmed)
			if err != nil || indent != 0 || key != "feeds" {
				return nil, errf(n, "expected top-level \"feeds:\"")
			}
			if val != "" {
				return nil, errf(n, "unsupported value for feeds: %q", val)
			}
			seenFeeds = true
			continue
		}
		if emptyList {
			return nil, errf(n, "unexpected content after \"feeds: []\"")
		}

		body := trimmed
		switch {
		case indent == 2 && strings.HasPrefix(trimmed, "- "):
			if err := finish(); err != nil {
				return nil, err
			}
			cur, curLine = &Feed{}, n
			body = strings.TrimSpace(trimmed[2:])
		case indent == 4 && cur != nil:
		default:
			return nil, errf(n, "unsupported structure (expected \"  - key: value\" or \"    key: value\")")
		}

		key, val, err := splitKV(body)
		if err != nil {
			return nil, errf(n, "%v", err)
		}
		if val == "" {
			return nil, errf(n, "empty or nested value for %q is not supported", key)
		}
		if err := set(cur, key, val); err != nil {
			return nil, errf(n, "%v", err)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if !seenFeeds {
		return nil, errf(n, "missing top-level \"feeds:\"")
	}
	if err := finish(); err != nil {
		return nil, err
	}
	return feeds, nil
}

// splitKV splits "key: value", decoding a quoted value and stripping a
// trailing comment from an unquoted one.
func splitKV(s string) (string, string, error) {
	i := strings.Index(s, ":")
	if i <= 0 {
		return "", "", fmt.Errorf("expected \"key: value\", got %q", s)
	}
	key := strings.TrimSpace(s[:i])
	rest := s[i+1:]
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
		return "", "", fmt.Errorf("expected space after \"%s:\"", key)
	}
	val, err := parseValue(strings.TrimSpace(rest))
	return key, val, err
}

func parseValue(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	switch s[0] {
	case '"':
		var b strings.Builder
		for i := 1; i < len(s); i++ {
			c := s[i]
			switch {
			case c == '\\' && i+1 < len(s):
				i++
				switch s[i] {
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				default:
					b.WriteByte(s[i])
				}
			case c == '"':
				return b.String(), checkTail(s[i+1:])
			default:
				b.WriteByte(c)
			}
		}
		return "", fmt.Errorf("unterminated double-quoted value")
	case '\'':
		var b strings.Builder
		for i := 1; i < len(s); i++ {
			if s[i] == '\'' {
				if i+1 < len(s) && s[i+1] == '\'' {
					b.WriteByte('\'')
					i++
					continue
				}
				return b.String(), checkTail(s[i+1:])
			}
			b.WriteByte(s[i])
		}
		return "", fmt.Errorf("unterminated single-quoted value")
	case '[', '{', '|', '>', '&', '*':
		return "", fmt.Errorf("unsupported YAML value %q", s)
	}
	if i := strings.Index(s, " #"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s), nil
}

func checkTail(t string) error {
	t = strings.TrimSpace(t)
	if t == "" || strings.HasPrefix(t, "#") {
		return nil
	}
	return fmt.Errorf("unexpected text after quoted value: %q", t)
}

func set(f *Feed, key, val string) error {
	switch key {
	case "title":
		f.Title = val
	case "url":
		f.URL = val
	case "site":
		f.Site = val
	case "topic":
		f.Topic = val
	case "kind":
		f.Kind = val
	case "added":
		f.Added = val
	case "why":
		f.Why = val
	default:
		return fmt.Errorf("unknown key %q", key)
	}
	return nil
}
