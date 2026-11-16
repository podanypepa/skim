package feedsfile

import (
	"strings"
	"testing"
)

func TestParseRealFile(t *testing.T) {
	fs, err := Load("../../feeds.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) < 2 {
		t.Fatalf("want at least 2 feeds, got %d", len(fs))
	}
	if fs[0].Title != "OpenAI News" || fs[0].URL != "https://openai.com/news/rss.xml" || fs[0].Topic != "ai-novinky" {
		t.Fatalf("bad first feed: %+v", fs[0])
	}
	if !strings.Contains(fs[1].Why, "Olshansk/rss-feeds.") || !strings.Contains(fs[1].Why, ";") {
		t.Fatalf("bad why: %q", fs[1].Why)
	}
}

func TestParseEmptyList(t *testing.T) {
	fs, err := Parse(strings.NewReader("# c\nfeeds: []\n"))
	if err != nil || len(fs) != 0 {
		t.Fatalf("got %v, %v", fs, err)
	}
}

func TestParseUnquotedAndComments(t *testing.T) {
	in := `# header
feeds:
  # first
  - title: Plain title: with colon
    url: https://a.com/feed   # trailing comment
    topic: 'single quoted'

  - url: "https://b.com/rss"
    why: "say \"hi\" \\ ok # not a comment"
`
	fs, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 2 {
		t.Fatalf("want 2, got %d", len(fs))
	}
	if fs[0].Title != "Plain title: with colon" || fs[0].URL != "https://a.com/feed" || fs[0].Topic != "single quoted" {
		t.Fatalf("bad 0: %+v", fs[0])
	}
	if fs[1].URL != "https://b.com/rss" || fs[1].Why != `say "hi" \ ok # not a comment` {
		t.Fatalf("bad 1: %+v", fs[1])
	}
}

func TestParseRejects(t *testing.T) {
	cases := map[string]string{
		"unknown key":  "feeds:\n  - url: https://a\n    foo: x\n",
		"nested":       "feeds:\n  - url: https://a\n    tags:\n      - x\n",
		"missing url":  "feeds:\n  - title: x\n",
		"no feeds key": "other:\n  - url: https://a\n",
		"top-level kv": "feeds:\n  - url: https://a\nextra: 1\n",
	}
	for name, in := range cases {
		_, err := Parse(strings.NewReader(in))
		if err == nil {
			t.Errorf("%s: expected error", name)
			continue
		}
		if !strings.Contains(err.Error(), "feeds.yaml:") {
			t.Errorf("%s: error without line: %v", name, err)
		}
	}
}
