# Feedback learning + full text Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `skimtriage examples` / `report` (učení ze zpětné vazby) a `skimdigest fulltext` (plný text u kandidátů s krátkým obsahem), napojené do skillů.

**Architecture:** `internal/store` dostane čtení feedbacku, záznamy návrhů a úložiště plného textu; `internal/feed` extrakci textu ze stránky. Nové podpříkazy v existujících nástrojích; skilly je volají.

**Tech Stack:** Go (stdlib), Claude skills.

**Spec:** `docs/superpowers/specs/2026-11-16-feedback-learning-design.md`, `docs/superpowers/specs/2026-11-16-fulltext-design.md`

## Global Constraints

- Jen stdlib; `gofmt -l .` prázdné, `go vet ./...`, `go test ./...` po každém tasku.
- Archiv (`data/articles`, `data/triage`) se nemění; `data/fulltext/*.jsonl`, `data/proposals.jsonl` jen append.
- Hodnoty: příklady `-n 30`; `due` = nových ≥ 20, nebo ≥ 5 a ≥ 7 dní; fulltext `-min 3`, `-short 600`, výsledek < 300 → `empty`, max. 20 000 znaků, souběh 4, timeout 20 s, max. 5 MB, jen `text/html`.
- User-Agent `skim-fetch/0.1 (+personal RSS reader)`.
- Git: změny na větvi `feedback-fulltext`, commit na konci (s CI z předchozího kroku); push až po souhlasu.

## Review Focus

1. **Hlas k článku bez triage nebo mimo archiv** → přeskočit v `examples` i `report`, žádná chyba. Test v Task 2 (`TestExamplesSkipsUnknown`).
2. **Stejný článek hlasovaný 2× (up, pak down)** → platí down v `examples` i `report`. Test v Task 2 a 3 (`TestExamplesLastVoteWins`, `TestReportLastVoteWins`).
3. **Stránka je cookie wall / JS shell (text < 300)** → `status: empty`, `candidates` nechá původní excerpt. Test v Task 5 (`TestFulltextEmptyKeepsExcerpt`).
4. **robots.txt vrací 404 nebo chybu** → stahování povoleno; `Disallow: /` pro `*` → `status: robots`. Test v Task 5 (`TestFulltextRobots`).
5. **Přesměrování na jinou doménu** → sleduje se, uloží se konečná URL. Test v Task 5 (`TestFulltextRedirect`).

---

### Task 1: Store – feedback, návrhy, plný text

**Files:** Create `internal/store/fulltext.go`, `internal/store/proposals.go`, `internal/store/feedback_test.go`; Modify `internal/store/feedback.go`.

**Interfaces – Produces:**
```go
func ReadFeedback(dataDir string) ([]Feedback, error)          // data/feedback.jsonl, skips malformed; missing = nil
type Proposal struct { At string `json:"at"`; Votes int `json:"votes"` }
func AppendProposal(dataDir string, p Proposal) error          // data/proposals.jsonl
func LastProposal(dataDir string) (*Proposal, error)           // nil when none
type Fulltext struct { ID, URL, Status, Content, FetchedAt string } // json: id,url,status,content,fetched_at
func ReadFulltext(dataDir, day string) ([]Fulltext, error)
func AppendFulltext(dataDir, day string, recs []Fulltext) error // data/fulltext/<day>.jsonl
```
- [ ] Testy `TestFeedbackRoundTrip`, `TestProposals` (bez souboru → nil; dva append → poslední), `TestFulltextRoundTrip` → FAIL → implementuj přes `readJSONL`/`appendJSONL` → PASS.

### Task 2: `skimtriage examples`

**Files:** Create `cmd/skimtriage/feedback.go` (sdílené načtení hlasů), `cmd/skimtriage/examples.go`, `cmd/skimtriage/examples_test.go`; Modify `cmd/skimtriage/main.go`.

**Interfaces – Produces:**
```go
type vote struct { ID, Vote, Note, Day, At string }
func latestVotes(dataDir string) ([]vote, error)   // one per article id, last `at` wins, newest first
type Example struct { ID, Topic, FeedTitle, Title string; Score int; Reason, Vote, Note, Day string; Mismatch bool }
// json: id, topic, feed_title, title, score, reason, vote, note(omitempty), day, mismatch
func runExamples(dataDir string, n int, out io.Writer) error
```
Triage záznamy pro všechna ID: projdi `store.Days(dataDir,"triage")` + `ReadTriage`. Články: `loadArchive`. Pořadí: mismatch nejdřív, pak ostatní; uvnitř skupin od nejnovějšího `at`. Summary `{"summary":true,"examples":N,"mismatches":M}`.
- [ ] Testy: `TestExamplesBasic` (up u 5, down u 4 → down/4 je mismatch a jde první), `TestExamplesLastVoteWins`, `TestExamplesSkipsUnknown`, `TestExamplesLimit` (n=2 ze 3), `TestExamplesNoFeedback` (jen summary 0/0) → FAIL → implementuj → PASS.

### Task 3: `skimtriage report`

**Files:** Create `cmd/skimtriage/report.go`, `cmd/skimtriage/report_test.go`; Modify `cmd/skimtriage/main.go`.

**Interfaces – Produces:**
```go
type FeedStat struct { FeedTitle, Topic string; Up, Down int; AvgScore float64 } // json: feed_title, topic, up, down, avg_score
type Report struct {
    Due bool; NewVotes, TotalVotes int; DaysSinceLast *int
    ByFeed []FeedStat; ByScore map[string]map[string]int; Notes []string
} // json: due, new_votes, total_votes, days_since_last, by_feed, by_score ({"4":{"up":1,"down":2}}), notes
func runReport(dataDir string, mark bool, now time.Time, out io.Writer) error
```
„Nové“ = hlasy s `at` > `LastProposal.At`; bez návrhu jsou nové všechny a `days_since_last` se počítá od nejstaršího hlasu (bez hlasů `null`). `by_feed` seřazené podle `down` sestupně. `AvgScore` na 1 desetinné místo. `mark` → `AppendProposal({At: now RFC3339, Votes: TotalVotes})` po výpisu.
- [ ] Testy: `TestReportAggregates`, `TestReportLastVoteWins`, `TestReportDue` (tabulka: 20 nových → true; 5 nových + 8 dní → true; 5 nových + 2 dny → false; 4 nové + 30 dní → false), `TestReportMark` (po `-mark` jsou nové 0, notes prázdné), `TestReportEmpty` → FAIL → implementuj → PASS.

### Task 4: Extrakce textu ze stránky

**Files:** Create `internal/feed/extract.go`, `internal/feed/extract_test.go`.

**Interfaces – Produces:** `func ExtractText(html string, max int) string`

Postup: odstraň bloky `script|style|nav|header|footer|aside|form|noscript|svg` (regex, case-insensitive, nehladový); region = obsah prvního `<article…>…</article>`, jinak `<main…>…</main>`, jinak `<body…>…</body>`, jinak celý vstup; z regionu vezmi elementy `p|h1|h2|h3|h4|li|pre|blockquote` (obsah mezi otevíracím a zavíracím tagem), každý přes `HTMLToText(_, 0)`, prázdné vynech, spoj `\n\n`, ořízni na `max` run.
- [ ] Testy: `TestExtractArticle` (nav + article + footer → jen text článku), `TestExtractMainFallback`, `TestExtractBodyFallback`, `TestExtractDropsScripts`, `TestExtractMax` → FAIL → implementuj → PASS.

### Task 5: `skimdigest fulltext` + napojení na `candidates`

**Files:** Create `cmd/skimdigest/fulltext.go`, `cmd/skimdigest/robots.go`, `cmd/skimdigest/fulltext_test.go`; Modify `cmd/skimdigest/main.go`, `cmd/skimdigest/day.go`, `cmd/skimdigest/candidates.go`.

**Interfaces:**
- Consumes: `store.ReadFulltext/AppendFulltext/Fulltext` (Task 1), `feed.ExtractText` (Task 4), `loadDay`.
- Produces:
```go
type FulltextOptions struct { DataDir, Day string; Min, Short int; Client *http.Client; Now time.Time }
func runFulltext(o FulltextOptions, out io.Writer) error
type robotsCache struct { … }                       // per host, Disallow rules for "*" and "skim-fetch"
func (c *robotsCache) allowed(u *url.URL) bool       // non-200 robots.txt → allowed
// entry gains: Fulltext *store.Fulltext
```
Reddit: host končí na `reddit.com` a cesta obsahuje `/comments/` → GET `strings.TrimSuffix(url,"/")+".json"`, JSON `[ {data:{children:[{data:{url, selftext, is_self}}]}} ]`; `is_self` → obsah = `selftext` (bez dalšího stahování, status ok/empty podle délky), jinak cíl = `url`. Výsledek pro stránku: `ExtractText(body, 20000)`; < 300 run → `empty`. URL v záznamu = konečná po přesměrování. Výstup řádek na článek `{id, url, status, chars}` + summary `{summary, tried, ok, failed}` (failed = vše mimo ok). Zápis přes `AppendFulltext` jednou na konci.
`candidates`: excerpt bere z `Fulltext.Content`, když `status == ok` a je delší než `Content` článku.
- [ ] Testy (httptest): `TestFulltextBasic` (krátký kandidát → ok, uloženo, `candidates` vrací plný text), `TestFulltextSelectsOnlyShortHighScored` (skóre 2 / dlouhý obsah / už stažený → nezkoušet), `TestFulltextEmptyKeepsExcerpt`, `TestFulltextRobots`, `TestFulltextRedirect`, `TestFulltextReddit` (link post → cílová stránka; self post → selftext), `TestFulltextHTTPError` (500 → status error, druhý běh nezkouší) → FAIL → implementuj → PASS.

### Task 6: Skilly, dokumentace, ostrý běh, commit

**Files:** Modify `skills/skim-triage/SKILL.md`, `skills/skim/SKILL.md`, `skills/skim-digest/SKILL.md`, `CLAUDE.md`, `README.md`.

- [ ] skim-triage: krok „před první dávkou spusť `go run ./cmd/skimtriage examples`; příklady = vodítko pro jemné rozdíly, při rozporu s `topics.md` platí `topics.md`; když žádné nejsou, pokračuj“.
- [ ] skim: nový krok 5 „Návrh úprav“: `go run ./cmd/skimtriage report`; když `due`, navrhni 1–3 úpravy **Zajímá mě / Nezajímá mě** s doložením (feed, počty, titulky z `examples`), uprav jen po souhlasu; pak `report -mark`.
- [ ] skim-digest krok 1: nejdřív `go run ./cmd/skimdigest fulltext [-day D]`, pak `candidates`.
- [ ] CLAUDE.md + README: nové příkazy, `data/fulltext/`, `data/proposals.jsonl`, učení ze zpětné vazby.
- [ ] Ověř: testy/vet/gofmt; `go run ./cmd/skimdigest fulltext -day 2026-11-16` ostře (počty ok/empty/robots/error); `go run ./cmd/skimtriage report` (bez hlasů → due false).
- [ ] Commit na větvi `feedback-fulltext` (včetně CI workflow a badge).
