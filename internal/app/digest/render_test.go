package digest

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func render(dir, input string, force bool) (string, error) {
	var out bytes.Buffer
	err := runRender(dir, day, force, strings.NewReader(input), &out)
	return out.String(), err
}

func digestPath(dir, ext string) string { return filepath.Join(dir, "digest", day+ext) }

func noDigest(t *testing.T, dir string) {
	t.Helper()
	for _, ext := range []string{".md", ".json"} {
		if _, err := os.Stat(digestPath(dir, ext)); !os.IsNotExist(err) {
			t.Fatalf("%s written", ext)
		}
	}
}

func goldenDay(t *testing.T) string {
	dir := t.TempDir()
	addArticles(t, dir, day, art("a"), art("b"), art("c"), art("d"), art("u"))
	addTriage(t, dir, day, tri("a", 5), tri("b", 4), tri("c", 3), tri("d", 4))
	return dir
}

const goldenInput = `{"stories":[
 {"topic":"ai","title":"Příběh jedna","summary":"Shrnutí prvního.","ids":["b","a"]},
 {"topic":"ai","title":"Příběh dva","summary":"Shrnutí druhého.","ids":["d"]}],
 "more":["c"]}`

func TestRenderGolden(t *testing.T) {
	dir := goldenDay(t)
	out, err := render(dir, goldenInput, false)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := os.ReadFile("testdata/digest.golden.md")
	got, _ := os.ReadFile(digestPath(dir, ".md"))
	if string(got) != string(want) {
		t.Fatalf("markdown mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	var d Digest
	b, _ := os.ReadFile(digestPath(dir, ".json"))
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	if d.Day != day || len(d.Stories) != 2 || d.Stories[0].N != 1 || d.Stories[1].N != 2 ||
		d.Stories[0].Score != 5 || d.Stories[0].Articles[0].URL != "https://e.com/b" || len(d.More) != 1 {
		t.Fatalf("digest json = %+v", d)
	}
	var res map[string]any
	if err := json.Unmarshal([]byte(out), &res); err != nil || res["stories"] != 2.0 || res["articles"] != 4.0 || res["path"] != digestPath(dir, ".md") {
		t.Fatalf("out = %q (%v)", out, err)
	}
}

func TestRenderValidation(t *testing.T) {
	long := func(n int) string { return strings.Repeat("ř", n) }
	story := func(title, summary, ids string) string {
		return `{"stories":[{"topic":"ai","title":"` + title + `","summary":"` + summary + `","ids":[` + ids + `]}]}`
	}
	cases := map[string]string{
		"unknown id":       story("T", "S", `"zzz"`),
		"untriaged id":     story("T", "S", `"u"`),
		"dup in stories":   `{"stories":[{"topic":"ai","title":"T","summary":"S","ids":["a"]},{"topic":"ai","title":"T2","summary":"S","ids":["a"]}]}`,
		"dup story-more":   `{"stories":[{"topic":"ai","title":"T","summary":"S","ids":["a"]}],"more":["a"]}`,
		"no stories":       `{"stories":[]}`,
		"empty title":      story("", "S", `"a"`),
		"empty summary":    story("T", "", `"a"`),
		"empty topic":      `{"stories":[{"topic":"","title":"T","summary":"S","ids":["a"]}]}`,
		"empty ids":        story("T", "S", ``),
		"newline in title": story(`A\nB`, "S", `"a"`),
		"long title":       story(long(121), "S", `"a"`),
		"long summary":     story("T", long(601), `"a"`),
		"id instead ids":   `{"stories":[{"topic":"ai","title":"T","summary":"S","id":["a"]}]}`,
		"two objects":      story("T", "S", `"a"`) + story("T", "S", `"b"`),
	}
	for name, in := range cases {
		dir := goldenDay(t)
		if _, err := render(dir, in, false); err == nil || !strings.Contains(err.Error(), "nic neuloženo") {
			t.Errorf("%s: err = %v", name, err)
		}
		noDigest(t, dir)
	}
	// Boundary values pass.
	dir := goldenDay(t)
	if _, err := render(dir, story(long(120), long(600)+`\n\n`+"druhý odstavec", `"a"`), false); err == nil {
		// 600 runes + paragraph exceeds the limit; ensure exactly 600 passes below.
		t.Fatal("summary over 600 runes should fail")
	}
	if _, err := render(dir, story(long(120), long(600), `"a"`), false); err != nil {
		t.Fatalf("boundary should pass: %v", err)
	}
}

func TestRenderCollectsAllErrors(t *testing.T) {
	dir := goldenDay(t)
	_, err := render(dir, `{"stories":[{"topic":"ai","title":"","summary":"S","ids":["a"]},{"topic":"ai","title":"T","summary":"S","ids":["zzz"]}]}`, false)
	if err == nil || !strings.Contains(err.Error(), "příběh 1") || !strings.Contains(err.Error(), "příběh 2") {
		t.Fatalf("err = %v", err)
	}
}

func TestRenderForce(t *testing.T) {
	dir := goldenDay(t)
	if _, err := render(dir, goldenInput, false); err != nil {
		t.Fatal(err)
	}
	md1, _ := os.ReadFile(digestPath(dir, ".md"))
	js1, _ := os.ReadFile(digestPath(dir, ".json"))
	other := `{"stories":[{"topic":"ai","title":"Jiný","summary":"Jiné.","ids":["a"]}]}`
	if _, err := render(dir, other, false); err == nil {
		t.Fatal("expected error without force")
	}
	md2, _ := os.ReadFile(digestPath(dir, ".md"))
	js2, _ := os.ReadFile(digestPath(dir, ".json"))
	if string(md1) != string(md2) || string(js1) != string(js2) {
		t.Fatal("files changed without force")
	}
	if _, err := render(dir, other, true); err != nil {
		t.Fatal(err)
	}
	md3, _ := os.ReadFile(digestPath(dir, ".md"))
	js3, _ := os.ReadFile(digestPath(dir, ".json"))
	if !strings.Contains(string(md3), "Jiný") || !strings.Contains(string(js3), "Jiný") {
		t.Fatal("force did not overwrite")
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "digest"))
	if len(entries) != 2 {
		t.Fatalf("leftover temp files: %v", entries)
	}
}

func TestRenderEscapesLinks(t *testing.T) {
	dir := t.TempDir()
	a := art("a")
	a.FeedTitle, a.Title, a.URL = "Feed", "A [beta] test", "https://e.com/a_(b) c"
	addArticles(t, dir, day, a, art("m"))
	addTriage(t, dir, day, tri("a", 4), tri("m", 3))
	if _, err := render(dir, `{"stories":[{"topic":"ai","title":"T","summary":"S","ids":["a"]}],"more":["a2"]}`, false); err == nil {
		t.Fatal("unknown more id should fail")
	}
	if _, err := render(dir, `{"stories":[{"topic":"ai","title":"T","summary":"S","ids":["a"]}]}`, false); err != nil {
		t.Fatal(err)
	}
	md, _ := os.ReadFile(digestPath(dir, ".md"))
	want := `[Feed – A \[beta\] test](<https://e.com/a_(b) c>)`
	if !strings.Contains(string(md), want) {
		t.Fatalf("missing %q in:\n%s", want, md)
	}
}

func TestCzechPlural(t *testing.T) {
	want := map[int]string{0: "příběhů", 1: "příběh", 2: "příběhy", 4: "příběhy", 5: "příběhů", 11: "příběhů", 12: "příběhů", 22: "příběhy"}
	for n, w := range want {
		if got := plural(n, "příběh", "příběhy", "příběhů"); got != w {
			t.Errorf("plural(%d) = %q, want %q", n, got, w)
		}
	}
}

func TestRenderForceKeepsPairConsistent(t *testing.T) {
	dir := goldenDay(t)
	if _, err := render(dir, goldenInput, false); err != nil {
		t.Fatal(err)
	}
	md1, _ := os.ReadFile(digestPath(dir, ".md"))
	// Make the JSON target impossible to replace.
	os.Remove(digestPath(dir, ".json"))
	if err := os.MkdirAll(filepath.Join(digestPath(dir, ".json"), "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := render(dir, `{"stories":[{"topic":"ai","title":"Jiný","summary":"Jiné.","ids":["a"]}]}`, true)
	if err == nil || !strings.Contains(err.Error(), "nic neuloženo") {
		t.Fatalf("err = %v", err)
	}
	md2, _ := os.ReadFile(digestPath(dir, ".md"))
	if string(md1) != string(md2) {
		t.Fatal("markdown replaced although json was not")
	}
}

func TestRenderRejectsMarkdownInjection(t *testing.T) {
	story := func(title, summary string) string {
		return `{"stories":[{"topic":"ai","title":"` + title + `","summary":"` + summary + `","ids":["a"]}]}`
	}
	for name, in := range map[string]string{
		"link in summary":    story("T", "Viz [zdroj](https://evil.example)."),
		"url in summary":     story("T", "Viz https://evil.example."),
		"autolink":           story("T", "Viz <http://x.y>."),
		"heading in summary": story("T", `Text.\n\n### 99. Falešný`),
		"quote line":         story("T", `Text.\n> citace`),
		"list line":          story("T", `Text.\n- bod`),
		"numbered line":      story("T", `Text.\n12. bod`),
		"backtick title":     story("C# a `x", "S"),
		"url in title":       story("Viz https://x.y", "S"),
		"newline in topic":   `{"stories":[{"topic":"ai\n## x","title":"T","summary":"S","ids":["a"]}]}`,
	} {
		dir := goldenDay(t)
		if _, err := render(dir, in, false); err == nil {
			t.Errorf("%s: expected error", name)
		}
		noDigest(t, dir)
	}
	// Ordinary Czech text with dashes, numbers and quotes passes.
	dir := goldenDay(t)
	if _, err := render(dir, story("GPT-6 a „Fable 5.2“", `Cena klesla o 20 % – na 8 USD.\n\nDruhý odstavec: 3 modely.`), false); err != nil {
		t.Fatalf("plain text rejected: %v", err)
	}
}

func TestRenderEscapesArchiveTitles(t *testing.T) {
	dir := t.TempDir()
	a, b := art("a"), art("b")
	a.Title, b.Title = "Don`t panic", "Vec<T> is *fast* _now_"
	addArticles(t, dir, day, a, b)
	addTriage(t, dir, day, tri("a", 4), tri("b", 4))
	if _, err := render(dir, `{"stories":[{"topic":"ai","title":"T","summary":"S","ids":["a","b"]}]}`, false); err != nil {
		t.Fatal(err)
	}
	md, _ := os.ReadFile(digestPath(dir, ".md"))
	want := "[Feed a – Don\\`t panic](https://e.com/a) · [Feed b – Vec\\<T\\> is \\*fast\\* \\_now\\_](https://e.com/b)"
	if !strings.Contains(string(md), want) {
		t.Fatalf("missing %q in:\n%s", want, md)
	}
}
