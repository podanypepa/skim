# skim-digest Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Go nástroj `cmd/skimdigest` (`candidates` / `render` / `feedback`) + skill `skim-digest`, který z ohodnocených článků dne udělá markdown digest `data/digest/D.md` s příběhy, shrnutími a správnými odkazy, a sbírá zpětnou vazbu.

**Architecture:** `candidates` spojí triage a články dne. Claude seskupí příběhy a napíše shrnutí jako JSON; `render` ho zvaliduje (vše, nebo nic) a vykreslí markdown podle pevné šablony s odkazy z archivu, plus uloží JSON. `feedback` mapuje čísla příběhů na ID článků a připisuje do `data/feedback.jsonl`.

**Tech Stack:** Go (module `skim`), jen standardní knihovna.

**Spec:** `docs/superpowers/specs/2026-11-16-skim-digest-design.md`

## Global Constraints

- Jen standardní knihovna; `gofmt -l .` prázdné, `go vet ./...`, `go test ./...` projdou po každém tasku.
- `data/articles/` a `data/triage/` se nemění; `data/feedback.jsonl` jen append.
- Výchozí hodnoty: `-day` = dnešek (lokální), `-min 3`, `-excerpt 2000`; `title` ≤ 120 run, `summary` ≤ 600 run, `note` ≤ 300 run; `more` max. 10 není vynucováno nástrojem (je to pravidlo skillu).
- Odkazy, názvy feedů a titulky článků v markdownu vždy z archivu, nikdy ze vstupu.
- Projekt není git repo: místo commitů `gofmt -l . && go vet ./... && go test ./...`.
- Texty pro uživatele česky.

## Review Focus

1. **Titulek článku s `[` `]` nebo URL se závorkami/mezerou** → odkaz v markdownu zůstane platný (text odkazu escapuje `[`, `]`, `\`; URL s `(`, `)` nebo mezerou se zapíše jako `<url>`). Test v Task 2 (`TestRenderEscapesLinks`).
2. **Existující digest bez `-force`** → chyba, oba soubory beze změny; s `-force` se přepíšou oba (atomicky přes temp + rename). Test v Task 2 (`TestRenderForce`).
3. **Nový řádek v titulku příběhu** → chyba (rozbil by nadpis); nové řádky ve shrnutí povoleny (odstavce). Test v Task 2 (`TestRenderValidation`).
4. **Den bez triage souboru** → `candidates` vrátí 0 kandidátů a `untriaged` = počet článků dne; den bez článků → 0 a 0, bez chyby. Test v Task 1 (`TestCandidatesNoTriage`).
5. **Překlep v klíči vstupu `render`** (`"id"` místo `"ids"`) → chyba neznámého pole, ne tichý prázdný příběh. Test v Task 2 (`TestRenderValidation`).

---

### Task 1: Načtení dne a `candidates`

**Files:**
- Create: `cmd/skimdigest/main.go`, `cmd/skimdigest/day.go`, `cmd/skimdigest/candidates.go`, `cmd/skimdigest/candidates_test.go`, `cmd/skimdigest/helpers_test.go`

**Interfaces:**
- Consumes: `store.ReadArticles`, `store.ReadTriage`, `store.Article`, `store.Triage`.
- Produces:
  ```go
  // dayData: articles of day D (first occurrence per id) joined with triage of day D.
  type entry struct { store.Article; Triage *store.Triage }
  type dayData struct { Day string; Order []string; ByID map[string]*entry }
  func loadDay(dataDir, day string) (*dayData, error)

  type CandOptions struct { DataDir, Day string; Min, Excerpt int }
  type Candidate struct {
      ID, Topic string; Score int; Reason, FeedTitle, Title, URL string
      Published string `json:",omitempty"`; Excerpt string
  } // json: id, topic, score, reason, feed_title, title, url, published, excerpt
  type CandSummary struct {
      Summary bool; Day string; Candidates int; ByScore map[string]int; Untriaged int
  } // json: summary, day, candidates, by_score, untriaged
  func runCandidates(o CandOptions, out io.Writer) error
  ```
  `main.go`: podpříkazy `candidates` | `render` | `feedback`, každý s vlastním `flag.FlagSet`, společné `-data data` a `-day` (výchozí `time.Now().Local().Format("2006-01-02")`); `-day` musí projít `time.Parse("2006-01-02")`, jinak exit 1 s chybou. `render` a `feedback` zatím vrací „not implemented“ (doplní Task 2 a 3).

- [ ] **Step 1: Failing testy** (helpers: zápis článků přes `store.AppendArticles`, triage přes `store.AppendTriage`; `excerpt` ořez po runách sdílí `truncateRunes` – zkopíruj ze `cmd/skimtriage/pending.go` do `day.go`):
  - `TestCandidatesBasic`: den s články a,b,c,d se skóre 3,5,1,4 → výstup b,d,a (skóre sestupně), `by_score {"5":1,"4":1,"3":1}`, `candidates 3`, `untriaged 0`; pole `reason`, `url`, `feed_title` z archivu/triage.
  - `TestCandidatesTieKeepsFileOrder`: dvě čtyřky → pořadí jako v souboru článků.
  - `TestCandidatesMin`: `Min 4` → jen 5 a 4; `by_score` počítá jen zobrazené.
  - `TestCandidatesUntriaged`: 3 články, 2 ohodnocené → `untriaged 1`.
  - `TestCandidatesNoTriage`: článek bez triage souboru → `candidates 0`, `untriaged 1`; prázdný den → `0`, `0`.
  - `TestCandidatesExcerptRunes`: 3000× „ř“, `Excerpt 2000` → 2000 run.
- [ ] **Step 2:** `go test ./cmd/skimdigest/` → FAIL.
- [ ] **Step 3: Implementuj** `loadDay` (triage záznamy pro ID, které v článcích dne nejsou, ignoruj; při dvou triage záznamech pro jedno ID ber první) a `runCandidates` (`sort.SliceStable` podle skóre).
- [ ] **Step 4:** `go test ./cmd/skimdigest/` → PASS.

### Task 2: `render`

**Files:**
- Create: `cmd/skimdigest/render.go`, `cmd/skimdigest/markdown.go`, `cmd/skimdigest/render_test.go`, `cmd/skimdigest/testdata/digest.golden.md`

**Interfaces:**
- Consumes: `loadDay` (Task 1).
- Produces:
  ```go
  type StoryInput struct { Topic, Title, Summary string; IDs []string } // json: topic,title,summary,ids
  type DigestInput struct { Stories []StoryInput; More []string }      // json: stories, more
  type Ref struct { ID, FeedTitle, Title, URL string; Score int }       // json: id, feed_title, title, url, score
  type Story struct { N int; Topic, Title, Summary string; Score int; Articles []Ref } // json: n, topic, title, summary, score, articles
  type Digest struct { Day string; Stories []Story; More []Ref }       // json: day, stories, more
  func runRender(dataDir, day string, force bool, in io.Reader, out io.Writer) error
  func renderMarkdown(d Digest) string
  ```
  `data/digest/D.json` = `Digest` (odsazený JSON). Výstup: `{"day":D,"stories":N,"articles":M,"path":"data/digest/D.md"}` (`path` = `filepath.Join(dataDir,"digest",D+".md")`, M = články v příbězích + `more`).

**Validace** (sbírej všechny chyby, pak `nic neuloženo:\n…`): JSON s `DisallowUnknownFields`, za objektem nic dalšího; ≥ 1 příběh; u příběhu č. i: `topic` neprázdný, `title` neprázdný, bez `\n`, ≤ 120 run, `summary` neprázdný ≤ 600 run, `ids` ≥ 1; každé ID (v `ids` i `more`) existuje v `loadDay`, má triage, a vyskytuje se jen jednou v celém vstupu. Chyby formuluj `příběh 2: …` / `more: …`.

**Šablona** (`renderMarkdown`, přesně):
```
# Skim – 16. 11. 2026

2 příběhy z 3 článků · ai-novinky

## ai-novinky

### 1. <title> `<score>`
<summary>

[<feed> – <article title>](<url>) · [<feed> – <article title>](<url>)

### 2. …

## Další zprávy

- [<article title>](<url>) – <feed>
```
- Datum `D. M. RRRR` bez nul na začátku. Řádek 3: `<N> <příběh|příběhy|příběhů> z <M> <článku|článků> · <témata spojená " · ">`. Čeština: 1 příběh, 2–4 příběhy, 5+ (a 0) příběhů. Předložka vždy `z` (vědomé zjednodušení – „ze“ před 2, 3, 4, 7… neřešíme). Článků: 1 → `článku`, jinak `článků`.
- Sekce `## <topic>` v pořadí prvního výskytu; příběhy v pořadí vstupu, `N` průběžně od 1 přes celý digest.
- Skóre příběhu = max skóre jeho článků. Zdroje v pořadí `ids`, oddělené ` · `.
- `## Další zprávy` jen když `more` není prázdné.
- Escapování (Review Focus 1): text odkazu escapuje `\`, `[`, `]`; URL obsahující mezeru, `(`, `)`, `<` nebo `>` → `<url>` (a `<`/`>` v ní procentuálně zakóduj).
- Soubor končí jedním `\n`.

- [ ] **Step 1: Failing testy:**
  - `TestRenderGolden`: den se 4 ohodnocenými články, vstup se 2 příběhy (2+1 ID) a 1 `more` → markdown shodný s `testdata/digest.golden.md` (napiš golden ručně podle šablony; `-update` flag nepřidávej); `D.json` obsahuje `n` 1 a 2 a skóre = max.
  - `TestRenderValidation` (tabulkový, každý → error s očekávaným podřetězcem, nic nezapsáno): neznámé ID; ID bez triage; ID 2× (mezi příběhy, příběh vs. `more`); prázdný `stories`; prázdný `title`/`summary`/`topic`/`ids`; `title` s `\n`; `title` 121× „ř“; `summary` 601× „ř“; klíč `"id"` místo `"ids"`; dva JSON objekty za sebou.
  - `TestRenderCollectsAllErrors`: 2 chybné příběhy → error obsahuje `příběh 1` i `příběh 2`.
  - `TestRenderForce`: druhý `render` bez `force` → error, obsah `D.md` a `D.json` beze změny; s `force` → nový obsah v obou.
  - `TestRenderEscapesLinks`: článek s titulkem `A [beta] test` a URL `https://e.com/a_(b) c` → řádek obsahuje `[Feed – A \[beta\] test](<https://e.com/a_(b) c>)`.
  - `TestCzechPlural`: `plural(n, "příběh", "příběhy", "příběhů")` pro 0,1,2,4,5,11,22 → příběhů, příběh, příběhy, příběhy, příběhů, příběhů, příběhy.
- [ ] **Step 2:** `go test ./cmd/skimdigest/ -run 'Render|Czech'` → FAIL.
- [ ] **Step 3: Implementuj** `runRender`, `renderMarkdown`, `plural(n int, one, few, many string) string` (few = n%10 ∈ 2..4 a n%100 ∉ 12..14). Zápis obou souborů: temp ve stejném adresáři + `os.Rename`, nejdřív JSON, pak MD; existence se kontroluje u MD.
- [ ] **Step 4:** `go test ./cmd/skimdigest/` → PASS.

### Task 3: `feedback`

**Files:**
- Create: `cmd/skimdigest/feedback.go`, `cmd/skimdigest/feedback_test.go`, `internal/store/feedback.go`

**Interfaces:**
- Consumes: `Digest` (Task 2) načtený z `data/digest/D.json`.
- Produces:
  ```go
  // internal/store/feedback.go
  type Feedback struct { Day string; Story int; IDs []string; Vote, Note, At string } // json: day, story, ids, vote, note (omitempty), at
  func AppendFeedback(dataDir string, recs []Feedback) error // data/feedback.jsonl, via appendJSONL
  // cmd/skimdigest/feedback.go
  func runFeedback(dataDir, day string, in io.Reader, now time.Time, out io.Writer) error
  ```
  Vstup JSONL `{"story":3,"vote":"up","note":"…"}`; `DisallowUnknownFields`, za objektem nic; `vote` ∈ {`up`,`down`}; `story` existuje v digestu; `note` ≤ 300 run. Chybějící digest → error `digest za D neexistuje`. Vše, nebo nic; ≥ 1 záznam. Výstup `{"saved":N}`. `IDs` = ID článků příběhu z `D.json`. Stejný příběh víckrát (i napříč běhy) je povolen – platí poslední.

- [ ] **Step 1: Failing testy:** `TestFeedbackOK` (2 záznamy → 2 řádky v `data/feedback.jsonl` s `ids` z digestu a `at` = now RFC3339), `TestFeedbackNoDigest`, `TestFeedbackValidation` (story 99, vote `meh`, note 301 run, neznámé pole, prázdný vstup → error, nic nezapsáno), `TestFeedbackAllOrNothing` (1 platný + 1 neplatný → nic).
- [ ] **Step 2:** `go test ./cmd/skimdigest/ -run Feedback` → FAIL.
- [ ] **Step 3: Implementuj.**
- [ ] **Step 4:** `go test ./...` → PASS; `gofmt`, `go vet` čisté.

### Task 4: Skill, dokumentace, ostrý běh

**Files:**
- Create: `skills/skim-digest/SKILL.md`
- Modify: `CLAUDE.md` (Struktura: `cmd/skimdigest/`, `data/digest/`, `data/feedback.jsonl`; Skilly: `skim-digest` mezi hotové; Konvence: příkazy)

- [ ] **Step 1: Napiš `SKILL.md`** (frontmatter `name: skim-digest`; `description`: „Sestaví denní přehled (digest) nejlepších článků: sloučí stejné zprávy do příběhů, česky je shrne, uloží do data/digest/ a ukáže v chatu. Použij, když chci digest, ‚co je dnes nového‘, nebo dávám zpětnou vazbu k příběhům.“). Obsah = sekce „Skill“ ze specu (kroky 1–7) + formát vstupu `render` s příkladem + pravidla: shrnutí jen z `excerpt`/`reason`, nic nedomýšlej; jeden JSON objekt na vstup `render`, heredoc `<<'EOF'`; v textech používej české uvozovky „…“; existující digest přepisuj (`-force`) jen když o to požádám; zpětnou vazbu zapisuj přes `go run ./cmd/skimdigest feedback` po jednom řádku na příběh.
- [ ] **Step 2: Uprav `CLAUDE.md`.**
- [ ] **Step 3: Ověř:** `go test ./... && go vet ./... && gofmt -l .` čisté; `go run ./cmd/skimdigest candidates -day 2026-11-16 | tail -1` → `candidates 43`, `untriaged 0`.
- [ ] **Step 4: Ostrý běh skillu** za 2026-11-16: vznikne `data/digest/2026-11-16.md` a `.json`, příběhů ≤ 20, v chatu seznam.
