# skim-fetch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Go nástroj `cmd/skimfetch` + skill `skim-fetch`, které stáhnou nové články ze všech feedů v `feeds.yaml` do `data/articles/YYYY-MM-DD.jsonl` bez duplicit.

**Architecture:** Parser feedů se vytáhne z `cmd/feedcheck` do `internal/feed` (sdílí oba nástroje). `internal/feedsfile` čte `feeds.yaml`, `internal/store` spravuje archiv a `data/state.json`. `cmd/skimfetch` stahuje feedy souběžně s podmíněným GETem a vypisuje JSONL souhrn; skill ho spustí a česky shrne.

**Tech Stack:** Go 1.24+ (module `skim`), jen standardní knihovna.

**Spec:** `docs/superpowers/specs/2026-11-16-skim-fetch-design.md`

## Global Constraints

- Jen standardní knihovna Go, žádné nové závislosti v `go.mod`.
- `gofmt -l .` prázdné, `go vet ./...` a `go test ./...` projdou po každém tasku.
- Už zapsané řádky v `data/articles/*.jsonl` se nikdy nepřepisují (jen append).
- `data/state.json` se zapisuje atomicky (temp soubor ve stejném adresáři + `os.Rename`).
- Souběh max. 8 feedů, HTTP timeout 20 s, User-Agent `skim-fetch/0.1 (+personal RSS reader)`.
- Výchozí hodnoty: `-backfill-days 7`, `seen` 90 dní, `content` max. 20 000 znaků (run, ne bajtů).
- Projekt není git repo: kroky „Commit“ vynech; místo nich spusť `gofmt -l . && go vet ./... && go test ./...`.
- Texty pro uživatele (skill, CLAUDE.md) česky.

## Review Focus

1. **Článek vypadne z okna `seen` (90 dní), ale feed ho pořád obsahuje** → nesmí se znovu objevit jako nový. Test v Task 4 (`TestPruneKeepsIDsStillInFeed`).
2. **Položka bez guid i odkazu** → ID z titulku+data je stabilní mezi běhy, druhý běh nic nepřidá. Test v Task 5 (`TestNoGUIDNoLinkStableID`).
3. **Stejná položka 2× v jednom dokumentu feedu** → jeden článek. Test v Task 5 (`TestDuplicateItemsInOneDocument`).
4. **Server vrátí 200 s HTML (chybová stránka) místo feedu** → `ok:false`, `failures`+1, ETag/Last-Modified ani `seen` se nemění. Test v Task 5 (`TestHTMLInsteadOfFeed`).
5. **Položky bez data při prvním stažení** → backfill je nemůže posoudit; berou se jako nové (`published` prázdné). Test v Task 5 (`TestBackfillKeepsUndated`).

---

### Task 1: Sdílený parser `internal/feed`

**Files:**
- Create: `internal/feed/feed.go`, `internal/feed/feed_test.go`
- Modify: `cmd/feedcheck/main.go` (odstranit `rssDoc`, `rssItem`, `atomDoc`, `looksLikeFeed`, `parseDate`, `dateLayouts`, `clean`, `wsRe`; `parseInto` volá `feed.Parse`)

**Interfaces:**
- Produces:
  ```go
  type Feed struct { Format, Title, Site string; Items []Item }  // Format: "rss" | "atom"
  type Item struct {
      GUID      string    // RSS <guid>, Atom <id>
      Title     string    // clean()
      Link      string    // RSS <link>; Atom <link rel="alternate"|""> href
      Author    string    // RSS <author> / dc:creator; Atom author/name
      Content   string    // raw HTML: content:encoded > description; Atom content > summary
      Published time.Time // pubDate > dc:date; Atom published > updated; zero = neznámé
  }
  func Parse(body []byte) (Feed, error)
  func LooksLikeFeed(body []byte, contentType string) bool
  func ParseDate(s string) time.Time
  func Clean(s string) string
  ```

- [ ] **Step 1: Napiš failing testy** v `internal/feed/feed_test.go`:
  - `TestParseRSSFull`: RSS 2.0 s `xmlns:content` a `xmlns:dc`; položka s `<guid>g1</guid>`, `<link>https://e.com/1</link>`, `<dc:creator>Jana</dc:creator>`, `<description>short</description>`, `<content:encoded><![CDATA[<p>full</p>]]></content:encoded>` → `Items[0]` = {GUID "g1", Link "https://e.com/1", Author "Jana", Content "<p>full</p>"}, `Format == "rss"`.
  - `TestParseRSSDescriptionFallback`: bez `content:encoded` → `Content == "short"`.
  - `TestParseRDF`: RSS 1.0 (`<rdf:RDF>` s `<item>` vedle `<channel>`) → 1 položka, `Published` z `dc:date` `2026-11-10T08:00:00Z`.
  - `TestParseAtom`: entry s `<id>tag:x,1</id>`, `<link rel="alternate" href="https://a.com/1"/>` a `<link rel="self" href="https://a.com/self"/>`, `<author><name>Petr</name></author>`, `<summary>s</summary>`, `<content type="html">&lt;b&gt;c&lt;/b&gt;</content>` → GUID "tag:x,1", Link "https://a.com/1", Author "Petr", Content "<b>c</b>".
  - `TestParseGarbage`: `Parse([]byte("<html>"))` vrátí error.
- [ ] **Step 2:** `go test ./internal/feed/` → FAIL (package/funkce neexistují).
- [ ] **Step 3: Implementuj** `internal/feed/feed.go`: struktury a funkce přesuň z `feedcheck` a rozšiř o nová pole. Rozlišení formátu jako dnes (`<feed` v prvních 2048 bajtech = Atom). XML namespace pro `content:encoded` je `http://purl.org/rss/1.0/modules/content/`, pro `dc:` `http://purl.org/dc/elements/1.1/`.
- [ ] **Step 4: Přepni `cmd/feedcheck`** na `feed.Parse` / `feed.LooksLikeFeed`; výstup `Result` beze změny.
- [ ] **Step 5:** `go test ./...` → PASS (nové testy i stávající `cmd/feedcheck` testy).

### Task 2: HTML → text

**Files:**
- Create: `internal/feed/text.go`, `internal/feed/text_test.go`

**Interfaces:**
- Produces: `func HTMLToText(s string, max int) string`

- [ ] **Step 1: Failing testy** (`TestHTMLToText`, tabulkový):
  - `"<p>Ahoj&nbsp;<b>světe</b></p><p>dál</p>"` → `"Ahoj světe\n\ndál"`
  - `"<script>x()</script><style>a{}</style>text"` → `"text"`
  - `"a<br>b"` → `"a\nb"`
  - `"&lt;tag&gt; &amp; &#233;"` → `"<tag> & é"`
  - `strings.Repeat("é", 30)` s `max=10` → přesně 10 run `é` (řez po runách, ne bajtech).
- [ ] **Step 2:** `go test ./internal/feed/ -run HTMLToText` → FAIL.
- [ ] **Step 3: Implementuj** regexy: odstraň `<script>…</script>`, `<style>…</style>` (case-insensitive, víceřádkově); `<br>` → `\n`; konce bloků (`</p>`, `</div>`, `</li>`, `</h1-6>`, `</blockquote>`, `</tr>`) → `\n\n`; ostatní tagy pryč; `html.UnescapeString`; ` ` → mezera; mezery v řádku sloučit, víc než 2 `\n` za sebou → `\n\n`; trim; ořez na `max` run.
- [ ] **Step 4:** `go test ./internal/feed/` → PASS.

### Task 3: Parser `feeds.yaml`

**Files:**
- Create: `internal/feedsfile/feedsfile.go`, `internal/feedsfile/feedsfile_test.go`

**Interfaces:**
- Produces:
  ```go
  type Feed struct { Title, URL, Site, Topic, Kind, Added, Why string }
  func Parse(r io.Reader) ([]Feed, error)
  func Load(path string) ([]Feed, error)
  ```

- [ ] **Step 1: Failing testy:**
  - `TestParseRealFile`: `Load("../../feeds.yaml")` → 13 feedů, `[0].Title == "OpenAI News"`, `[0].URL == "https://openai.com/news/rss.xml"`, `[0].Topic == "ai-novinky"`; `[1].Why` obsahuje `"Olshansk/rss-feeds."` (dvojtečky a středníky uvnitř uvozovek zůstanou).
  - `TestParseEmptyList`: `"# c\nfeeds: []\n"` → 0 feedů, bez chyby.
  - `TestParseUnquotedAndComments`: neuvozovkové hodnoty, komentáře na samostatných řádcích, prázdné řádky → správně načteno; `'single quoted'` taky.
  - `TestParseRejects`: chyba (s číslem řádku ve zprávě) pro: neznámý klíč na úrovni položky (`foo: x`), vnořený seznam/mapa, chybějící `url`, chybějící `feeds:`.
- [ ] **Step 2:** `go test ./internal/feedsfile/` → FAIL.
- [ ] **Step 3: Implementuj** řádkový parser: ignoruj prázdné a `#` řádky; očekávej `feeds:` (nebo `feeds: []`); položka začíná `  - key: value`, další klíče `    key: value`; hodnota trim, odstraň obklopující `"…"` (s unescape `\"` a `\\`) nebo `'…'`; povolené klíče = pole `Feed` (lowercase); `url` povinné. Cokoli jiného → `fmt.Errorf("feeds.yaml:%d: …")`.
- [ ] **Step 4:** `go test ./internal/feedsfile/` → PASS.

### Task 4: Úložiště `internal/store`

**Files:**
- Create: `internal/store/store.go`, `internal/store/store_test.go`

**Interfaces:**
- Produces:
  ```go
  type Article struct {
      ID        string `json:"id"`
      FeedURL   string `json:"feed_url"`
      FeedTitle string `json:"feed_title"`
      Topic     string `json:"topic"`
      Title     string `json:"title"`
      URL       string `json:"url"`
      Author    string `json:"author,omitempty"`
      Published string `json:"published,omitempty"` // RFC3339, prázdné když neznámé
      FetchedAt string `json:"fetched_at"`          // RFC3339
      Content   string `json:"content"`
  }
  type FeedState struct {
      ETag         string            `json:"etag,omitempty"`
      LastModified string            `json:"last_modified,omitempty"`
      LastOK       string            `json:"last_ok,omitempty"`
      LastError    string            `json:"last_error,omitempty"`
      Failures     int               `json:"failures"`
      Seen         map[string]string `json:"seen"` // id -> RFC3339 čas prvního/posledního spatření
  }
  type State struct { Feeds map[string]*FeedState `json:"feeds"` } // klíč = feed URL

  func ArticleID(feedURL, guid, link, title string, published time.Time) string
  func LoadState(dataDir string) (*State, error)   // chybějící soubor = prázdný stav
  func (s *State) Save(dataDir string) error        // atomicky
  func (fs *FeedState) Prune(now time.Time, inFeed map[string]bool, keep time.Duration)
  func AppendArticles(dataDir string, day time.Time, arts []Article) error
  ```

- [ ] **Step 1: Failing testy:**
  - `TestArticleID`: 16 hex znaků; priorita guid > link > title+published (stejný guid, jiný link → stejné ID; prázdný guid → podle linku); různé feed URL → různé ID.
  - `TestStateRoundTrip`: `LoadState` na prázdném `t.TempDir()` → prázdný stav bez chyby; Save + Load → shodný obsah; po Save v adresáři není žádný dočasný soubor.
  - `TestPruneKeepsIDsStillInFeed`: Seen {"old": now-100d, "gone": now-100d, "fresh": now-1d}, inFeed {"old"} → zůstane "old" (s časem obnoveným na now) a "fresh", "gone" zmizí.
  - `TestAppendArticles`: dvě volání pro stejný den → `data/articles/2026-11-16.jsonl` má 4 řádky v pořadí, každý validní JSON; HTML znaky (`<`, `&`) nejsou escapované (`SetEscapeHTML(false)`).
- [ ] **Step 2:** `go test ./internal/store/` → FAIL.
- [ ] **Step 3: Implementuj.** `ArticleID`: `sha256(feedURL + "\n" + key)`, kde key = guid, jinak link, jinak `title + "\n" + published.UTC().Format(time.RFC3339)` (zero time → prázdný řetězec); prvních 16 hex. `AppendArticles`: `MkdirAll(dataDir/articles)`, soubor `day.Format("2006-01-02") + ".jsonl"`, `O_APPEND|O_CREATE|O_WRONLY`, 0644. `Save`: `os.CreateTemp(dataDir, "state-*.json")`, JSON s odsazením, `Close`, `Rename` na `state.json`.
- [ ] **Step 4:** `go test ./internal/store/` → PASS.

### Task 5: Nástroj `cmd/skimfetch`

**Files:**
- Create: `cmd/skimfetch/main.go`, `cmd/skimfetch/main_test.go`

**Interfaces:**
- Consumes: `feed.Parse`, `feed.LooksLikeFeed`, `feed.HTMLToText`, `feedsfile.Load`, `store.*` (viz výše).
- Produces (testovatelné jádro, `main` jen parsuje flagy a volá):
  ```go
  type Options struct {
      FeedsPath, DataDir, Topic string
      BackfillDays              int
      DryRun                    bool
      Now                       time.Time
      Client                    *http.Client
  }
  type FeedResult struct {
      FeedURL  string `json:"feed_url"`
      Title    string `json:"title"`
      Topic    string `json:"topic"`
      OK       bool   `json:"ok"`
      Status   string `json:"status"` // "new" | "not_modified" | "no_new" | "error"
      New      int    `json:"new"`
      Error    string `json:"error,omitempty"`
      Failures int    `json:"failures"`
  }
  type Summary struct {
      Summary bool `json:"summary"` // vždy true
      Feeds, OK, Failed, New int    // json: feeds, ok, failed, new
  }
  func Run(opts Options, out io.Writer) (Summary, error) // error jen pro konfiguraci/úložiště
  ```
  Flagy: `-feeds feeds.yaml`, `-data data`, `-topic ""`, `-backfill-days 7`, `-dry-run`. Exit 1 když `Run` vrátí error, jinak 0.

**Chování `Run`** (pořadí je podstatné):
1. `feedsfile.Load`, filtr podle `-topic`; `store.LoadState`.
2. Feedy souběžně (semafor 8). Pro každý: GET s `If-None-Match`/`If-Modified-Since` ze stavu. 304 → `not_modified`. HTTP ≥ 400, síťová chyba, `!LooksLikeFeed` nebo chyba `Parse` → `error`, `Failures++`, `LastError`; ETag/LastModified/Seen beze změny.
3. Úspěch: pro každou položku `ArticleID`; přeskoč ID v `Seen` nebo už viděné v tomto dokumentu. Pokud feed ve stavu ještě nebyl (první stažení) a položka má `Published` starší než `Now - BackfillDays`, jen ji přidej do `Seen`, článek nevytvářej. Nové → `Article` (`Content = HTMLToText(item.Content, 20000)`, `FeedTitle` z feeds.yaml), `Seen[id] = Now`. Pak `Prune(Now, idsVDokumentu, 90 dní)`, ulož ETag/Last-Modified z odpovědi, `LastOK`, `Failures = 0`, `LastError = ""`.
4. Po doběhnutí všech: `AppendArticles(DataDir, Now (lokální datum), všechny nové v pořadí feedů z feeds.yaml)`, **teprve potom** `state.Save` (pád mezi tím → nanejvýš duplicita, nikdy ztráta). `-dry-run` nezapíše ani jedno.
5. Na `out` JSONL: `FeedResult` v pořadí feeds.yaml, pak `Summary`.

- [ ] **Step 1: Failing testy** (httptest server s počitadlem požadavků a přepínatelnými odpověďmi; `Now = 2026-11-16 12:00 UTC`; `DataDir = t.TempDir()`; feeds.yaml zapsaný do temp souboru):
  - `TestFirstRunAndDedup`: RSS se 3 položkami z posledních dní → 1. běh `New == 3` a soubor `articles/2026-11-16.jsonl` má 3 řádky; 2. běh `New == 0`, status `no_new`, soubor pořád 3 řádky.
  - `TestConditionalGet`: server posílá `ETag: "v1"` a na `If-None-Match: "v1"` vrací 304 → 2. běh status `not_modified`, server viděl hlavičku.
  - `TestBackfill`: položky staré 1 a 30 dní → 1. běh `New == 1`; 2. běh se stejným feedem `New == 0` (stará se nepřidá ani později).
  - `TestBackfillKeepsUndated`: položka bez data při prvním běhu → `New == 1`, `published` v JSON chybí.
  - `TestNoGUIDNoLinkStableID`: položka jen s titulkem a datem → 2 běhy, celkem 1 článek.
  - `TestDuplicateItemsInOneDocument`: dva `<item>` se stejným `<guid>` → `New == 1`.
  - `TestHTMLInsteadOfFeed`: 1. běh OK s `ETag: "v1"`; 2. běh server vrátí 200 `text/html` `<html>` → `ok:false`, `failures == 1`; ve state.json ETag pořád `"v1"` a `seen` nezměněný.
  - `TestOneFeedFailsOthersContinue`: dva feedy, jeden 500 → `Summary{Feeds:2, OK:1, Failed:1, New:3}`, `Run` vrátí nil error.
  - `TestTopicFilter` a `TestDryRun`: filtr zpracuje jen feedy tématu; dry-run nevytvoří `articles/` ani `state.json`.
  - `TestBadFeedsFile`: neexistující `-feeds` → `Run` vrátí error.
- [ ] **Step 2:** `go test ./cmd/skimfetch/` → FAIL.
- [ ] **Step 3: Implementuj** `Run` a `main` podle chování výše (zápisy do stavu pod `sync.Mutex`, nebo per-feed výsledky sesbírat a stav aktualizovat po `wg.Wait()`).
- [ ] **Step 4:** `go test ./...` → PASS; `gofmt -l .` prázdné; `go vet ./...` čisté.
- [ ] **Step 5: Ověř naživo:** `go run ./cmd/skimfetch -dry-run` → 13 řádků + souhrn, většina `ok:true`. Pak bez `-dry-run` dvakrát po sobě: 2. běh má `new` 0 (nebo jen nové od 1. běhu) a soubor se nezdvojí.

### Task 6: Skill a dokumentace

**Files:**
- Create: `skills/skim-fetch/SKILL.md` (adresář `.claude/skills` je symlink na `skills/`, nic dalšího netřeba)
- Modify: `CLAUDE.md` (sekce Struktura: `cmd/skimfetch/`, `internal/`, `data/`; sekce Skilly: přesunout `skim-fetch` z plánovaných mezi hotové; Konvence: příkaz `go run ./cmd/skimfetch`)

- [ ] **Step 1: Napiš `SKILL.md`** ve stylu `skills/skim-discover/SKILL.md` (frontmatter `name: skim-fetch`, česká `description`: „Stáhne nové články ze všech odebíraných feedů do archivu data/. Použij, když chci stáhnout/aktualizovat články, ‚co je nového‘ před triage nebo digestem.“). Obsah:
  1. Spusť `go run ./cmd/skimfetch` (volitelně `-topic <slug>`, když chci jen jedno téma).
  2. Česky shrň: celkem nových článků; tabulka po tématech a feedech (jen feedy s `new > 0`); selhané feedy s chybou.
  3. Feed s `failures >= 3`: ověř ho `go run ./cmd/feedcheck <url>` a navrhni opravu URL nebo odebrání z `feeds.yaml` – změny jen po mém souhlasu.
  4. Nabídni další krok (`skim-triage`, až bude existovat).
  - Pravidla: nikdy nemaž ani neupravuj soubory v `data/articles/`; `data/state.json` needituj ručně.
- [ ] **Step 2: Uprav `CLAUDE.md`** dle Files výše.
- [ ] **Step 3: Ověř:** `go test ./... && go vet ./... && gofmt -l .` čisté; skill je vidět v `.claude/skills/skim-fetch/SKILL.md`.
