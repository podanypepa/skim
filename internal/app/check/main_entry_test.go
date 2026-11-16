package check

import (
	"bytes"
	"strings"
	"testing"
)

func TestMainErrorExitCode(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Main(nil, strings.NewReader(""), &out, &errb); code != 2 || out.Len() != 0 || !strings.Contains(errb.String(), "usage") {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), errb.String())
	}
}

func TestMainHelpExitsZero(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Main([]string{"-h"}, strings.NewReader(""), &out, &errb); code != 0 {
		t.Fatalf("-h code=%d", code)
	}
}
