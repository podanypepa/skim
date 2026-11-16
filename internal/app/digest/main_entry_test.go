package digest

import (
	"bytes"
	"strings"
	"testing"
)

func TestMainErrorExitCode(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Main([]string{"render", "-day", "2026-11-16", "-data", t.TempDir()}, strings.NewReader(""), &out, &errb); code != 1 || !strings.Contains(errb.String(), "skimdigest:") {
		t.Fatalf("code=%d err=%q", code, errb.String())
	}
	if code := Main(nil, strings.NewReader(""), &out, &errb); code != 2 {
		t.Fatalf("no subcommand code=%d", code)
	}
	if code := Main([]string{"candidates", "-day", "bad"}, strings.NewReader(""), &out, &errb); code != 1 {
		t.Fatalf("bad day code=%d", code)
	}
}
