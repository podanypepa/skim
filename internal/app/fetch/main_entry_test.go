package fetch

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainErrorExitCode(t *testing.T) {
	var out, errb bytes.Buffer
	code := Main([]string{"-feeds", filepath.Join(t.TempDir(), "missing.yaml"), "-data", t.TempDir()}, strings.NewReader(""), &out, &errb)
	if code != 1 || !strings.Contains(errb.String(), "skimfetch:") {
		t.Fatalf("code=%d err=%q", code, errb.String())
	}
	if code := Main([]string{"-nonsense"}, strings.NewReader(""), &out, &errb); code != 2 {
		t.Fatalf("bad flag code=%d", code)
	}
}
