package feed

import (
	"strings"
	"testing"
)

func TestHTMLToText(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"<p>Ahoj&nbsp;<b>světe</b></p><p>dál</p>", 100, "Ahoj světe\n\ndál"},
		{"<script>x()</script><STYLE>a{}</STYLE>text", 100, "text"},
		{"<script type=\"a\">\nmulti\nline\n</script>ok", 100, "ok"},
		{"a<br>b<br/>c", 100, "a\nb\nc"},
		{"&lt;tag&gt; &amp; &#233;", 100, "<tag> & é"},
		{"  lots   of\t\tspace  ", 100, "lots of space"},
		{"<div>a</div>\n\n\n\n<div>b</div>", 100, "a\n\nb"},
		{strings.Repeat("é", 30), 10, strings.Repeat("é", 10)},
	}
	for _, c := range cases {
		if got := HTMLToText(c.in, c.max); got != c.want {
			t.Errorf("HTMLToText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
