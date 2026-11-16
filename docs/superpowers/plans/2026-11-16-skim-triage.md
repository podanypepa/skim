# skim-triage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Go nástroj `cmd/skimtriage` (`pending` / `save`) + skill `skim-triage`, kterým Claude ohodnotí každý stažený článek skóre 1–5 s českým důvodem do `data/triage/YYYY-MM-DD.jsonl`.

**Architecture:** `internal/store` dostane čtení archivu a typ/zápis hodnocení. `cmd/skimtriage pending` vypisuje dávky neohodnocených článků, `save` validuje dávku (vše, nebo nic) a připisuje. Úsudek dělá skill.

**Tech Stack:** Go (module `skim`), jen standardní knihovna.

**Spec:** `docs/superpowers/specs/2026-11-16-skim-triage-design.md`

## Global Constraints

- Jen standardní knihovna; `gofmt -l .` prázdné, `go vet ./...`, `go test ./...` projdou po každém tasku.
- `data/articles/` se nikdy nemění; `data/triage/*.jsonl` jen append.
- Výchozí hodnoty: `-limit 25`, `-days 7`, `-excerpt 1500` (run), `reason` max. 300 run, `score` celé číslo 1–5.
- Soubor hodnocení má stejný den jako soubor článku (`data/triage/<den článku>.jsonl`).
- Projekt není git repo: místo commitů `gofmt -l . && go vet ./... && go test ./...`.
- Texty pro uživatele česky.

## Review Focus

1. **Poškozený (useknutý) řádek v archivu nebo v triage souboru** → přeskočit, nástroj nespadne. Test v Task 1 (`TestReadSkipsMalformedLines`).
2. **`reason` s diakritikou** → limit 300 se počítá v runách, ne bajtech (300× „ř“ projde, 301 ne). Test v Task 3 (`TestSaveReasonLimitRunes`).
3. **`score` jako `4.5`, `"4"`, `0`, `6` nebo chybějící** → chyba; `4.0` → také chyba (musí to být celé JSON číslo bez desetinné části). Test v Task 3 (`TestSaveScoreValidation`).
4. **Stejné ID ve dvou denních souborech** (duplicita po pádu skimfetch) → `pending` ho ukáže jednou (první den), `save` ho zapíše jednou do prvního dne. Test v Task 2 (`TestPendingDedupAcrossDays`) a Task 3 (`TestSaveUsesFirstDay`).
5. **Prázdný vstup do `save`** → chyba „žádné záznamy“, nic se nezapíše (chrání před tichým selháním skillu). Test v Task 3 (`TestSaveEmptyInput`).

---

### Task 1: Čtení archivu a hodnocení v `internal/store`

**Files:**
- Create: `internal/store/read.go`, `internal/store/triage.go`, `internal/store/read_test.go`

**Interfaces:**
- Produces:
  ```go
  // Days returns dates (YYYY-MM-DD) of files in dataDir/<sub>/*.jsonl, ascending.
  func Days(dataDir, sub string) ([]string, error)            // sub: "articles" | "triage"; missing dir = nil, nil
  // ReadArticles reads dataDir/articles/<day>.jsonl, skipping malformed lines.
  func ReadArticles(dataDir, day string) ([]Article, error)

  type Triage struct {
      ID        string `json:"id"`
      Topic     string `json:"topic"`
      Score     int    `json:"score"`
      Reason    string `json:"reason"`
      TriagedAt string `json:"triaged_at"` // RFC3339
  }
  func ReadTriage(dataDir, day string) ([]Triage, error)       // skips malformed lines
  func TriagedIDs(dataDir string) (map[string]bool, error)     // across all triage files
  func AppendTriage(dataDir, day string, recs []Triage) error  // same semantics as AppendArticles
  ```

- [ ] **Step 1: Failing testy** (`read_test.go`):
  - `TestDays`: soubory `articles/2026-11-14.jsonl`, `2026-11-16.jsonl`, `notes.txt`, `bad.jsonl` → `["2026-11-14","2026-11-16"]`; neexistující adresář → prázdné, bez chyby.
  - `TestReadSkipsMalformedLines`: soubor se 2 platnými řádky, prázdným řádkem a useknutým `{"id":"x","ti` mezi nimi → 2 články; totéž pro `ReadTriage`.
  - `TestTriageRoundTrip`: `AppendTriage` 2× pro dva dny → `ReadTriage` vrátí záznamy v pořadí; `TriagedIDs` obsahuje všechna ID; HTML znaky neescapované.
- [ ] **Step 2:** `go test ./internal/store/` → FAIL (nedefinované funkce).
- [ ] **Step 3: Implementuj.** `Days`: `os.ReadDir`, jen jména odpovídající `time.Parse("2006-01-02", name bez .jsonl)`. Čtení přes `bufio.Scanner` s bufferem 4 MB; nevalidní JSON nebo prázdné `id` → přeskočit. `AppendTriage` sdílí s `AppendArticles` jednu interní funkci `appendJSONL(path string, v []any)` (nebo generickou `appendJSONL[T any]`).
- [ ] **Step 4:** `go test ./internal/store/` → PASS.

### Task 2: `skimtriage pending`

**Files:**
- Create: `cmd/skimtriage/main.go`, `cmd/skimtriage/pending.go`, `cmd/skimtriage/pending_test.go`

**Interfaces:**
- Consumes: `store.Days`, `store.ReadArticles`, `store.TriagedIDs` (Task 1).
- Produces:
  ```go
  type PendingOptions struct {
      DataDir, Topic         string
      Limit, Days, Excerpt   int
      Now                    time.Time
  }
  type PendingItem struct {
      ID        string `json:"id"`
      Day       string `json:"day"`
      Topic     string `json:"topic"`
      FeedTitle string `json:"feed_title"`
      Title     string `json:"title"`
      URL       string `json:"url"`
      Published string `json:"published,omitempty"`
      Excerpt   string `json:"excerpt"`
  }
  type PendingSummary struct { Summary bool `json:"summary"`; Pending int `json:"pending"`; Shown int `json:"shown"` }
  func runPending(opts PendingOptions, out io.Writer) error
  ```
  `main.go`: `os.Args[1]` = `pending` | `save`, každý s vlastním `flag.FlagSet`; jinak usage na stderr a exit 2. Chyba z `run*` → stderr, exit 1. `-data data` u obou.

- [ ] **Step 1: Failing testy** (`Now = 2026-11-16 12:00 UTC`, temp data dir, články zapsané přes `store.AppendArticles`):
  - `TestPendingBasic`: 3 články ve dnech 11-15 a 11-16, 1 z nich v triage → 2 řádky, nejstarší den první, v rámci dne pořadí v souboru; summary `{pending:2, shown:2}`.
  - `TestPendingLimit`: 30 článků, `Limit 25` → 25 řádků, summary `{pending:30, shown:25}`.
  - `TestPendingDaysWindow`: `Days 7` → článek ze dne 11-10 (7. den zpět včetně dneška = 11-10..11-16) se ukáže, z 11-09 ne.
  - `TestPendingTopicFilter`: jen články daného tématu, `pending` počítá jen je.
  - `TestPendingExcerptRunes`: obsah 2000× „ř“, `Excerpt 1500` → excerpt přesně 1500 run.
  - `TestPendingDedupAcrossDays`: stejné ID v 11-15 i 11-16 → jednou, `day == "2026-11-15"`.
  - `TestPendingEmpty`: nic k hodnocení → jen summary `{pending:0, shown:0}`.
- [ ] **Step 2:** `go test ./cmd/skimtriage/` → FAIL.
- [ ] **Step 3: Implementuj** `runPending`. Okno: dny `d` s `d >= Now.Local() - (Days-1) dní` (porovnání řetězců `YYYY-MM-DD`). JSON encoder s `SetEscapeHTML(false)`.
- [ ] **Step 4:** `go test ./cmd/skimtriage/` → PASS.

### Task 3: `skimtriage save`

**Files:**
- Create: `cmd/skimtriage/save.go`, `cmd/skimtriage/save_test.go`

**Interfaces:**
- Consumes: `store.Days`, `store.ReadArticles`, `store.TriagedIDs`, `store.AppendTriage`, `store.Triage` (Task 1).
- Produces: `func runSave(dataDir string, in io.Reader, now time.Time, out io.Writer) error`

**Chování:** čti stdin po řádcích (prázdné přeskoč); každý řádek dekóduj `json.Decoder` s `UseNumber()` do `{id, score json.Number, reason}` a neznámá pole odmítni (`DisallowUnknownFields`). Pravidla na řádek: ID existuje v archivu (všechny dny), není v `TriagedIDs`, není v dávce dřív; `score` je celé číslo bez `.`/exponentu v 1–5; `reason` po `TrimSpace` neprázdný a ≤ 300 run. Sbírej **všechny** chyby jako `řádek N: <popis>`; jakákoli chyba nebo 0 záznamů → vrať error s chybami spojenými `\n`, nic nezapisuj. Jinak: `Topic` a den z článku (první den, kde se ID vyskytuje), `TriagedAt = now.Format(RFC3339)`, seskup podle dne, `AppendTriage` po dnech, vypiš `{"saved":N}`.

- [ ] **Step 1: Failing testy:**
  - `TestSaveOK`: 2 platné záznamy pro články ze dvou dnů → `{"saved":2}`, každý v souboru svého dne, `topic` a `triaged_at` doplněné.
  - `TestSaveAllOrNothing`: 1 platný + 1 s neznámým ID → error obsahuje `řádek 2`, žádný triage soubor nevznikl.
  - `TestSaveScoreValidation`: tabulkový, každé zvlášť → error: `4.5`, `"4"`, `0`, `6`, `4.0`, chybějící score.
  - `TestSaveReasonLimitRunes`: 300× „ř“ → OK; 301× „ř“ → error; `"   "` → error.
  - `TestSaveDuplicates`: stejné ID 2× v dávce → error; ID už uložené dřívějším `save` → error.
  - `TestSaveUnknownField`: `{"id":..,"score":4,"reason":"x","foo":1}` → error.
  - `TestSaveUsesFirstDay`: ID ve dvou dnech → zapsáno jen do souboru prvního dne.
  - `TestSaveEmptyInput`: prázdný vstup / jen prázdné řádky → error, nic nezapsáno.
  - `TestSaveCollectsAllErrors`: 3 chybné řádky → error obsahuje `řádek 1`, `řádek 2`, `řádek 3`.
- [ ] **Step 2:** `go test ./cmd/skimtriage/ -run Save` → FAIL.
- [ ] **Step 3: Implementuj** `runSave` podle chování výše.
- [ ] **Step 4:** `go test ./...` → PASS; `gofmt`, `go vet` čisté.

### Task 4: Skill, dokumentace, ostrý běh

**Files:**
- Create: `skills/skim-triage/SKILL.md`
- Modify: `CLAUDE.md` (Struktura: `cmd/skimtriage/`, `data/triage/`; Skilly: `skim-triage` mezi hotové; Konvence: příkazy)

- [ ] **Step 1: Napiš `SKILL.md`** (frontmatter `name: skim-triage`; `description`: „Ohodnotí nově stažené články skóre 1–5 podle topics.md a zapíše důvod. Použij po skim-fetch, když chci roztřídit články nebo vědět, co z nového stojí za přečtení.“). Obsah podle specu: škála 1–5 s významy (doslova ze specu), postup (přečti `topics.md` → smyčka `go run ./cmd/skimtriage pending` → ohodnoť → `go run ./cmd/skimtriage save <<'EOF' … EOF` → dokud `pending` > 0), pravidla hodnocení (hodnoť vůči tématu článku; „Nezajímá mě“ → 1–2; krátký/prázdný obsah → podle titulku a feedu; důvod 1 věta česky, konkrétní – co článek přináší nebo proč je šum), co dělat při chybě `save` (oprav jen chybné řádky a pošli celou dávku znovu), závěrečné shrnutí (počty podle skóre; tabulka 5 a 4: titulek, feed, důvod), návrh úprav `topics.md` při opakovaném vzoru jen po souhlasu. Pravidla: neměň `data/articles/` ani existující triage řádky.
- [ ] **Step 2: Uprav `CLAUDE.md`.**
- [ ] **Step 3: Ověř:** `go test ./... && go vet ./... && gofmt -l .` čisté; `go run ./cmd/skimtriage pending -limit 3` vypíše 3 řádky + summary s `pending: 80`.
- [ ] **Step 4: Ostrý běh skillu** nad dnešními 80 články (postup ze SKILL.md); po skončení `pending` vrací 0 a `data/triage/2026-11-16.jsonl` má 80 řádků.
