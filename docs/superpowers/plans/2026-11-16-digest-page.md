# Digest page Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Soukromá stránka na claude.ai s digestem posledních 14 dní a 👍/👎, jejíž hlasy `skim` převezme do `data/feedback.jsonl`.

**Architecture:** `skimdigest export` vyrobí `data/web/digests.json` z `data/digest/*.json`; statická šablona `web/digest.html` ho načte a vykreslí, hlasy ukládá do `db` capability. Skill `skim` hlasy převezme (ArtifactData → `skimdigest feedback`) a stránku po digestu znovu publikuje.

**Tech Stack:** Go (stdlib), HTML/CSS/JS bez knihoven, Artifact `db` capability.

**Spec:** `docs/superpowers/specs/2026-11-16-digest-page-design.md`

## Global Constraints

- Jen stdlib; `gofmt`, `go vet`, `go test ./...` po každém tasku.
- Do repa jen `web/digest.html`; `data/web/` je git-ignored (v `data/`).
- Odkazy a texty na stránce jen z exportu; vkládat přes `textContent`, href jen `http(s)`.
- 14 dní historie; den = den digestu; česky „16. 11. 2026“.
- Git: větev `digest-page`, commit na konci; merge + push po review (uživatel už souhlasil s postupem „po review merge a push“ u minulé dávky – pro tuto se zeptám znovu).

## Review Focus

1. **`title` v hlasu nesedí (digest přestavěn s `-force`)** → řádek je chyba, nic se nezapíše; bez `title` funguje jako dřív. Test v Task 2 (`TestFeedbackTitleCheck`).
2. **Méně digestů než 14 / žádný** → export vrátí, co je (`days: []`), stránka ukáže prázdný stav. Test v Task 1 (`TestExportFewDays`).
3. **Soubor v `data/digest/` s jiným názvem (`.md`, poškozený JSON)** → ignorovat / přeskočit, ne spadnout. Test v Task 1 (`TestExportSkipsJunk`).
4. **URL článku není http(s) (`javascript:`)** → stránka ho nevykreslí jako odkaz. Ověří kontrola šablony (Task 3) + review.
5. **Stránka bez `db` (null) nebo bez sítě k `digests.json`** → čtení funguje / srozumitelná hláška, tlačítka skrytá. Ověří Task 3.

---

### Task 1: `skimdigest export`

**Files:** Create `cmd/skimdigest/export.go`, `cmd/skimdigest/export_test.go`; Modify `cmd/skimdigest/main.go`.

**Interfaces – Produces:**
```go
type ExportArticle struct { ID, FeedTitle, Title, URL string; Score int `json:"score,omitempty"` } // json: id, feed_title, title, url
type ExportStory struct { N int; Topic, Title, Summary string; Score int; Articles []ExportArticle }
type ExportDay struct { Day string; Stories []ExportStory; More []ExportArticle }
type Export struct { GeneratedAt string; Days []ExportDay } // json: generated_at, days
func runExport(dataDir string, days int, outPath string, now time.Time) error
```
Načte `store.Days(dataDir, "digest")` – pozor: `Days` hledá `*.jsonl`; **Ruling na místě:** přidej `store.DaysExt(dataDir, sub, ext string)` a `Days` ho volá s `.jsonl`; export použije `.json`. Posledních N, nejnovější první; poškozený JSON přeskoč. Zápis přes temp + rename, `MkdirAll` adresáře. `-out` výchozí `<data>/web/digests.json`.
- [ ] Testy: `TestExportBasic` (2 digesty → nejnovější první, pole přenesena), `TestExportFewDays` (0 digestů → `days: []`; 3 digesty a `days 2` → 2), `TestExportSkipsJunk` (`.md` soubor a poškozený `.json` → ignorováno) → FAIL → implementuj → PASS.

### Task 2: `feedback` s volitelným `title`

**Files:** Modify `cmd/skimdigest/feedback.go`, `cmd/skimdigest/feedback_test.go`.

`feedbackInput` dostane `Title string json:"title"`; když neprázdné a `!= stories[n].Title` → `bad("titulek nesedí: příběh %d je teď %q", ...)`.
- [ ] Test `TestFeedbackTitleCheck`: shoda → uloženo; neshoda → chyba „řádek 1“, nic nezapsáno; bez title → uloženo → FAIL → implementuj → PASS.

### Task 3: Šablona `web/digest.html`

**Files:** Create `web/digest.html`.

Před psaním načti skill `artifact-design` (kontrakt stránky: title, tokeny, dark mode, mobil). Obsah dle specu: načtení `digests.json` (`fetch('digests.json')`), přepínač dnů, sekce po tématech, karty, „Další zprávy“ (`<details>`), 👍/👎 přes `claude.use("db")` → `db.collection("votes")`, `onSnapshot`, `doc(day+"_"+n).set({day, story, title, vote, at, imported:false})`, zrušení = `delete()` jen když `!imported`; `null` db → tlačítka skrýt. Odkaz jen když `/^https?:\/\//`. Zobrazení skóre jako štítek. 10 s timeout na db.
- [ ] Ověř lokálně: `go run ./cmd/skimdigest export` → `data/web/digests.json` existuje; šablonu projdi očima (žádné innerHTML s daty).

### Task 4: Skilly, dokumentace, publikace

**Files:** Modify `skills/skim/SKILL.md`, `skills/skim-digest/SKILL.md`, `CLAUDE.md`, `README.md`.

- [ ] skim: krok 0 „Převzetí hlasů ze stránky“ (když existuje `data/web/artifact-url`: `ArtifactData list votes` → `imported:false` → po dnech `skimdigest feedback -day D` s `{story, vote, title}`; neshody vynech a nahlas; `batch update imported:true`). Krok „Publikuj stránku“ po digestu: `skimdigest export`, Artifact publish `web/digest.html` + `files {"digests.json": "data/web/digests.json"}` + `capabilities {db:{}}`; URL z/do `data/web/artifact-url`; odkaz v chatu.
- [ ] skim-digest krok 5: zmínka, že hlasy jdou dát i na stránce.
- [ ] CLAUDE.md + README: stránka, `export`, `web/`, `data/web/`.
- [ ] Ověř testy/vet/gofmt; ostrá publikace stránky; jeden `ArtifactData list votes` (prázdný); uložit URL.
- [ ] Commit na větvi `digest-page`.
