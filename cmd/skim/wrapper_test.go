package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func wrapperPath(t *testing.T) string {
	p, err := filepath.Abs("../../skim")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func runWrapper(t *testing.T, script, pathEnv string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{script}, args...)...)
	cmd.Dir = t.TempDir() // run from an unrelated directory
	cmd.Env = append(os.Environ(), "PATH="+pathEnv)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func goDir(t *testing.T) string {
	g, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go not on PATH")
	}
	return filepath.Dir(g)
}

// Called through a symlink in another directory, the wrapper still runs
// the repo's code.
func TestWrapperViaSymlink(t *testing.T) {
	gd := goDir(t)
	link := filepath.Join(t.TempDir(), "skim")
	if err := os.Symlink(wrapperPath(t), link); err != nil {
		t.Fatal(err)
	}
	out, err := runWrapper(t, link, gd+":/usr/bin:/bin", "version")
	if err != nil || !strings.Contains(out, "skim dev") {
		t.Fatalf("err=%v out=%q", err, out)
	}
}

// With Go available, the repo source wins over a (possibly stale) skim
// binary on PATH.
func TestWrapperPrefersRepoSourceWithGo(t *testing.T) {
	gd := goDir(t)
	fake := t.TempDir()
	os.WriteFile(filepath.Join(fake, "skim"), []byte("#!/bin/sh\necho stale-binary\n"), 0o755)
	out, err := runWrapper(t, wrapperPath(t), fake+":"+gd+":/usr/bin:/bin", "version")
	if err != nil || !strings.Contains(out, "skim dev") || strings.Contains(out, "stale-binary") {
		t.Fatalf("err=%v out=%q", err, out)
	}
}

// toolsWithoutGo returns a directory with only the commands the wrapper
// needs, so the test really runs without Go (CI images have /usr/bin/go).
func toolsWithoutGo(t *testing.T) string {
	dir := t.TempDir()
	for _, name := range []string{"bash", "sh", "dirname", "basename", "readlink"} {
		p, err := exec.LookPath(name)
		if err != nil {
			t.Skipf("%s not found", name)
		}
		if err := os.Symlink(p, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// Without Go, an installed binary is used.
func TestWrapperUsesBinaryWithoutGo(t *testing.T) {
	fake := t.TempDir()
	os.WriteFile(filepath.Join(fake, "skim"), []byte("#!/bin/sh\necho installed-binary \"$@\"\n"), 0o755)
	out, err := runWrapper(t, wrapperPath(t), fake+":"+toolsWithoutGo(t), "version")
	if err != nil || !strings.Contains(out, "installed-binary version") {
		t.Fatalf("err=%v out=%q", err, out)
	}
}

func TestGitattributesKeepsWrapperLF(t *testing.T) {
	b, err := os.ReadFile("../../.gitattributes")
	if err != nil || !strings.Contains(string(b), "skim text eol=lf") {
		t.Fatalf("missing .gitattributes rule: %v %q", err, b)
	}
}
