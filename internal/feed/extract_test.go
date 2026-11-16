package feed

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestExtractArticle(t *testing.T) {
	html := `<html><body><nav><p>Menu item</p></nav><header><h1>Site</h1></header>
<article><h1>Title</h1><p>First <b>para</b>.</p><ul><li>point</li></ul><div class="ad">ad text</div></article>
<footer><p>Copyright</p></footer></body></html>`
	got := ExtractText(html, 0)
	if got != "Title\n\nFirst para.\n\npoint" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractMainFallback(t *testing.T) {
	html := `<body><nav><p>x</p></nav><main><p>Main text.</p></main><aside><p>side</p></aside></body>`
	if got := ExtractText(html, 0); got != "Main text." {
		t.Fatalf("got %q", got)
	}
}

func TestExtractBodyFallback(t *testing.T) {
	html := `<html><head><title>T</title></head><body><div><p>One.</p><p>Two.</p></div><footer><p>f</p></footer></body></html>`
	if got := ExtractText(html, 0); got != "One.\n\nTwo." {
		t.Fatalf("got %q", got)
	}
}

func TestExtractDropsScripts(t *testing.T) {
	html := `<article><script>var p = "<p>no</p>";</script><style>p{}</style><p>Yes.</p><form><p>login</p></form></article>`
	if got := ExtractText(html, 0); got != "Yes." {
		t.Fatalf("got %q", got)
	}
}

func TestExtractMax(t *testing.T) {
	html := "<p>" + strings.Repeat("ř", 50) + "</p>"
	if got := ExtractText(html, 10); utf8.RuneCountInString(got) != 10 {
		t.Fatalf("got %q", got)
	}
}

func TestExtractNoDoubleUnescape(t *testing.T) {
	html := `<article><p>if a &lt; b and c &gt; d then &amp;lt; stays</p></article>`
	if got := ExtractText(html, 0); got != "if a < b and c > d then &lt; stays" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractSkipsTinyArticleCard(t *testing.T) {
	body := strings.Repeat("Real content sentence. ", 20)
	html := `<body><article><a href="/x"><h3>Related post</h3></a></article><main><p>` + body + `</p></main></body>`
	if got := ExtractText(html, 0); !strings.HasPrefix(got, "Real content sentence.") {
		t.Fatalf("got %q", got)
	}
}
