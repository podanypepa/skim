package triage

import (
	"bytes"
	"strings"
	"testing"
)

func TestMainErrorExitCode(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Main([]string{"pending", "-days", "0", "-data", t.TempDir()}, strings.NewReader(""), &out, &errb); code != 1 || !strings.Contains(errb.String(), "skimtriage:") {
		t.Fatalf("code=%d err=%q", code, errb.String())
	}
	if code := Main([]string{"bogus"}, strings.NewReader(""), &out, &errb); code != 2 {
		t.Fatalf("unknown subcommand code=%d", code)
	}
	out.Reset()
	if code := Main([]string{"pending", "-data", t.TempDir()}, strings.NewReader(""), &out, &errb); code != 0 || !strings.Contains(out.String(), `"summary":true`) {
		t.Fatalf("ok path code=%d out=%q", code, out.String())
	}
}
