package digest

import (
	"net/url"
	"strings"
	"testing"
)

func TestRobotsRFC9309(t *testing.T) {
	cases := []struct {
		robots, path string
		allowed      bool
	}{
		{"User-agent: *\nDisallow: /\nAllow: /blog/\n", "/blog/post", true},
		{"User-agent: *\nDisallow: /\nAllow: /blog/\n", "/shop", false},
		{"User-agent: *\nDisallow: /*?utm\n", "/a?utm=x", false},
		{"User-agent: *\nDisallow: /*?utm\n", "/a", true},
		{"User-agent: *\nDisallow: /*/print$\n", "/x/print", false},
		{"User-agent: *\nDisallow: /*/print$\n", "/x/printer", true},
		{"User-agent: *\nDisallow: /search?q=\n", "/search?q=go", false},
		{"User-agent: *\nDisallow: /\n\nUser-agent: skim-fetch\nAllow: /\n", "/anything", true},
		{"User-agent: *\nDisallow: /private\n", "/private/a", false},
		{"User-agent: *\nDisallow:\n", "/x", true},
		{"User-agent: other\nDisallow: /\n", "/x", true},
		{"User-agent: *\nDisallow: /page\nAllow: /page\n", "/page", true},
	}
	for _, c := range cases {
		g := parseRobots(strings.NewReader(c.robots))
		u, _ := url.Parse("https://e.com" + c.path)
		if got := g.allows(u); got != c.allowed {
			t.Errorf("robots %q path %q: allowed=%v, want %v", c.robots, c.path, got, c.allowed)
		}
	}
}
