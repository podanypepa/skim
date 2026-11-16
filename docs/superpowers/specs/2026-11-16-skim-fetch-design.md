# skim-fetch – návrh

## Záměr

Stáhnout nové články ze všech feedů v `feeds.yaml` do lokálního archivu,
ze kterého budou číst další skilly (`skim-triage`, `skim-digest`,
`skim-search`). Deterministická práce → Go nástroj; skill jen spouští
a česky shrnuje výsledek.

## Cíle / ne-cíle

- Cíl: opakovatelné spuštění stáhne jen nové články, bez duplicit.
- Cíl: jeden rozbitý feed neshodí celý běh.
- Ne-cíl: stahování plného textu z webu (rozhodnuto: jen obsah feedu;
  plný text později jen u článků, které triage označí jako zajímavé).
- Ne-cíl: hodnocení relevance, shrnutí, plánované spouštění.
- Anti-cíl: přepisování už stažených dat; cizí Go závislosti.

## Přístupy

- **A) Denní JSONL soubory + stavový soubor** – zvoleno. Digest pracuje
  po dnech, snadno se grepuje, stejná konvence jako `feedcheck`,
  surová data oddělená od úsudku (triage píše vedle).
- B) Soubor na článek (markdown + hlavička) – čitelné, ale tisíce souborů
  a triage by přepisovala data.
- C) Jeden `articles.jsonl` – neomezeně roste, digest by četl vše.

## Návrh

### Kód
- `internal/feed/` – parser RSS 2.0 / RSS 1.0 / Atom vytažený z
  `feedcheck` a rozšířený o: guid/id, odkaz, autora, obsah
  (`content:encoded` > `description`, Atom `content` > `summary`), datum.
  `feedcheck` se přepne na tento balíček (chování beze změny).
- `internal/feedsfile/` – minimální parser `feeds.yaml` jen pro jeho
  schéma (seznam plochých map, hodnoty v uvozovkách i bez, komentáře).
  Nepodporovaný tvar → jasná chyba, ne tichý odhad.
- `cmd/skimfetch/` – nástroj.

### Úložiště (`data/`)
- `data/articles/YYYY-MM-DD.jsonl` – podle **data stažení** (lokální čas),
  append-only. Řádek:
  `id, feed_url, feed_title, topic, title, url, author, published,
  fetched_at, content` (content = prostý text: HTML tagy pryč,
  entity dekódované, max. 20 000 znaků).
- `id` = prvních 16 hex z sha256(`feed_url` + (guid || odkaz || titulek+datum)).
- `data/state.json` – na feed: `etag`, `last_modified`, `last_ok`,
  `last_error`, `failures` (po sobě jdoucí), `seen` (ID z posledních
  90 dní). Zápis atomicky (temp soubor + rename).

### Běh
- Feedy souběžně (max. 8), timeout 20 s, User-Agent jako feedcheck.
- Podmíněný GET (`If-None-Match`, `If-Modified-Since`); 304 = nic nového.
- První stažení feedu: jen články z posledních 7 dní (`-backfill-days`).
- Přepínače: `-feeds feeds.yaml`, `-data data`, `-topic <slug>`,
  `-backfill-days 7`, `-dry-run` (nic nezapíše).
- Výstup na stdout JSONL: řádek na feed
  `{feed_url, title, topic, ok, status, new, error, failures}`
  a závěrečný souhrn `{summary: true, feeds, ok, failed, new}`.
- Exit 0 i při selhání jednotlivých feedů; nenulový jen při chybě
  konfigurace / úložiště.

### Skill `skills/skim-fetch/`
Spustí `go run ./cmd/skimfetch`, česky shrne: počet nových článků
po tématech a feedech, selhávající feedy; feed s `failures` ≥ 3 navrhne
prověřit (přes `feedcheck`, případně odebrat po schválení).

### Testy
Parser feedů (RSS, RDF, Atom, obsah, guid), parser `feeds.yaml`
(včetně odmítnutí nepodporovaného tvaru), deduplikace, backfill,
podmíněný GET a selhání feedu přes `httptest`. `gofmt`, `go vet`.

### Dokumentace
Aktualizovat `CLAUDE.md` (struktura, skilly).

## Moje rozhodnutí (k ověření)

- Soubor podle data stažení, ne publikace (digest = „co přišlo od minule“).
- Backfill 7 dní, `seen` 90 dní, obsah max. 20 000 znaků, souběh 8.
- HTML → text regexem + `html.UnescapeString` (stdlib; `x/net/html` není stdlib).
- Projekt není git repo → `data/` zatím ničím neignoruji; až bude git, přidat do `.gitignore`.
