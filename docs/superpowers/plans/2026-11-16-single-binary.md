# Single binary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Jeden program `skim` (podpříkazy check/fetch/triage/digest/version), staré `cmd/*` jako obálky, wrapper `./skim`, release workflow pro binárky.

**Architecture:** Kód čtyř `package main` se přesune do `internal/app/{check,fetch,triage,digest}` s jednotným vstupem `Main`; `cmd/skim` a staré `cmd/*` jen volají `Main`. Skilly volají `./skim`, který použije nainstalovanou binárku, jinak `go run ./cmd/skim`.

**Tech Stack:** Go stdlib, bash, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-11-16-single-binary-design.md`

## Global Constraints

- Chování, flagy a výstupy nástrojů beze změny (JSONL, chybové hlášky, návratové kódy 0/1/2).
- `go run ./cmd/feedcheck|skimfetch|skimtriage|skimdigest …` funguje dál.
- Jen stdlib; `gofmt`, `go vet`, `go test ./...` po každém tasku; CI zelené.
- Release targety: darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64; `CGO_ENABLED=0 -trimpath`.
- Žádný tag/release bez souhlasu uživatele. Větev `single-binary`.

## Review Focus

1. **Staré příkazy (`go run ./cmd/skimtriage pending`)** → stejný výstup jako `skim triage pending`. Test v Task 2 (`TestLegacyAndNewSame` – volá obě `Main` se stejnými argumenty nad temp data a porovná výstup).
2. **Neznámý / chybějící podpříkaz (`skim`, `skim foo`, `skim triage`)** → nápověda na stderr, kód 2, nic na stdout. Test v Task 2 (`TestDispatchUsage`).
3. **Chyba nástroje** → kód 1 a hláška s prefixem nástroje (`skimtriage: …`) na stderr. Test v Task 1 (`TestMainErrorExitCode` pro každý balíček).
4. **`./skim` bez Go i bez binárky** → srozumitelná hláška, kód 127. Ověřit ručně v Task 3 (`PATH=/usr/bin:/bin ./skim version`).
5. **Windows binárka** → `skim.exe` v zipu; ověřit lokálním `GOOS=windows go build` v Task 4.

---

### Task 1: Přesun logiky do `internal/app/*`

**Files:**
- Move: `cmd/feedcheck/*.go` → `internal/app/check/`, `cmd/skimfetch/*.go` → `internal/app/fetch/`, `cmd/skimtriage/*.go` → `internal/app/triage/`, `cmd/skimdigest/*.go` (+ `testdata/`) → `internal/app/digest/` (`git mv`).
- Create: nové tenké `cmd/<name>/main.go`.

**Interfaces – Produces (každý balíček):**
```go
// Main runs the tool with args (without the program name) and returns the exit code.
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int
```
Úpravy v přesunutých souborech: `package main` → `package check|fetch|triage|digest` (i testy); `main()` → `Main(...)`; `flag.*` globály → `flag.NewFlagSet(name, flag.ContinueOnError)` s `SetOutput(stderr)` (chyba parsování → kód 2); `os.Args` → `args`; `os.Stdout/Stdin/Stderr` → parametry; `os.Exit(n)`/`usage()` → `return n`. Prefix chyb zůstává (`feedcheck:`/`skimfetch:`/`skimtriage:`/`skimdigest:`). Obálka: `func main() { os.Exit(check.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }`.
- [ ] Testy `TestMainErrorExitCode` v každém balíčku (feedcheck bez URL → 2; fetch s neexistujícím `-feeds` → 1 a stderr obsahuje `skimfetch:`; triage `pending -days 0` → 1; digest `render` s prázdným stdin nad prázdnými daty → 1) → FAIL (Main neexistuje) → přesun + úpravy → `go test ./...` PASS; `go run ./cmd/skimtriage pending -limit 1` vypíše summary.

### Task 2: `cmd/skim`

**Files:** Create `cmd/skim/main.go`, `cmd/skim/dispatch.go`, `cmd/skim/dispatch_test.go`.

**Interfaces – Produces:** `var version = "dev"`; `func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int`.
Směrování: `check` → `check.Main(rest)`, `fetch` → `fetch.Main(rest)`, `triage` → `triage.Main(rest)` (rest začíná podpříkazem `pending|save|examples|report`), `digest` → `digest.Main(rest)`; `version` → `skim <version>`; `help`/`-h`/`--help` → nápověda na stdout, 0; nic/neznámé → nápověda na stderr, 2. Nápověda vypíše příkazy s jednořádkovým popisem.
- [ ] Testy `TestDispatchUsage` (bez args / `foo` → 2, stdout prázdný), `TestDispatchHelpVersion`, `TestLegacyAndNewSame` (temp data s 1 článkem: `run(["triage","pending","-data",d])` == `triage.Main(["pending","-data",d])` výstup) → FAIL → implementuj → PASS.

### Task 3: Wrapper `./skim`, skilly, dokumentace

**Files:** Create `skim` (bash, `chmod +x`); Modify `skills/*/SKILL.md`, `CLAUDE.md`, `README.md`, `.gitignore` (`/dist/`).
- [ ] `./skim`: `#!/usr/bin/env bash`, `set -euo pipefail`, `cd "$(dirname "$0")"`; když `command -v skim` a nejde o tento skript (porovnej `realpath`), `exec skim "$@"`; jinak když `command -v go`, `exec go run ./cmd/skim "$@"`; jinak hláška „skim: install the skim binary (see README) or Go 1.24+“ na stderr, `exit 127`.
- [ ] Skilly a CLAUDE.md: nahraď `go run ./cmd/feedcheck` → `./skim check`, `go run ./cmd/skimfetch` → `./skim fetch`, `go run ./cmd/skimtriage X` → `./skim triage X`, `go run ./cmd/skimdigest X` → `./skim digest X` (grep musí být prázdný: `grep -rn "go run ./cmd" skills CLAUDE.md`).
- [ ] README: sekce „Install“ (3 cesty: Releases binárka do PATH / `go install github.com/podanypepa/skim/cmd/skim@latest` / nic, `./skim` použije Go), příkazy v sekci tools přepsat na `skim …`, poznámka, že staré `go run ./cmd/...` fungují.
- [ ] **Ruling:** `go install` vyžaduje module path `github.com/podanypepa/skim`; dnes `go.mod` říká `module skim` → změň module path a importy (`sed` přes `*.go`), jinak `go install` nefunguje.
- [ ] Ověř: `./skim version`, `./skim triage pending -limit 1`, `PATH=/usr/bin:/bin ./skim version` → kód 127 s hláškou.

### Task 4: Release workflow

**Files:** Create `.github/workflows/release.yml`.
- [ ] Trigger `push: tags: ['v*']`; `permissions: contents: write`; job: checkout, setup-go (`go-version-file`), `go test ./...`, smyčka přes 5 targetů: `GOOS/GOARCH CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${GITHUB_REF_NAME}" -o dist/<name>/skim[.exe] ./cmd/skim`, zabal `skim_<ver>_<os>_<arch>.tar.gz` (Windows `.zip`) s README a LICENSE, `sha256sum > dist/SHA256SUMS`, `gh release create "$GITHUB_REF_NAME" dist/*.tar.gz dist/*.zip dist/SHA256SUMS --generate-notes` (`GH_TOKEN: ${{ github.token }}`).
- [ ] Ověř lokálně: build pro všech 5 targetů projde (do scratch adresáře), `windows` dá `skim.exe`.
- [ ] Commit na větvi; merge + push a tag `v0.1.0` až po souhlasu.
