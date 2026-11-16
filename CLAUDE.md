# skim

Osobní RSS AI agent, náhrada za Feedly. Místo procházení stovek nadpisů
agent sleduje vybrané zdroje, třídí články podle mých zájmů, shrnuje je
a umožňuje v nich vyhledávat.

Projekt začíná jako sada Claude skillů (úsudek: co je relevantní, shrnutí,
vyhledávání) doplněná malými Go nástroji (deterministická práce: stahování,
parsování, ukládání). Z Go nástrojů později může vyrůst samostatná aplikace.

## Struktura

- `topics.md` – témata, která mě zajímají, a co v nich chci / nechci. Zdroj pravdy pro výběr zdrojů i třídění článků.
- `feeds.yaml` – odebírané feedy. Přidávají se jen po mém schválení.
- `cmd/feedcheck/` – Go nástroj: ověří RSS/Atom feed (nebo ho najde z URL webu) a vrátí JSONL se stavem, frekvencí a ukázkou titulků.
- `cmd/skimfetch/` – Go nástroj: stáhne nové články ze všech feedů do `data/` (bez duplicit, podmíněný GET), vrátí JSONL souhrn.
- `cmd/skimtriage/` – Go nástroj pro evidenci triage: `pending` vypíše neohodnocené články, `save` zvaliduje a uloží hodnocení (vše, nebo nic).
- `cmd/skimdigest/` – Go nástroj pro digest: `candidates` (ohodnocené články dne), `render` (zvaliduje příběhy od skillu a vykreslí markdown s odkazy z archivu), `feedback` (uloží 👍/👎 k příběhům).
- `internal/` – sdílené Go balíčky: `feed` (parser RSS/Atom, HTML → text), `feedsfile` (čtení `feeds.yaml`), `store` (archiv a stav).
- `data/articles/YYYY-MM-DD.jsonl` – archiv článků podle data stažení, jen append. `data/state.json` – co už je stažené, ETagy, selhání. `data/triage/YYYY-MM-DD.jsonl` – hodnocení článků (skóre 1–5 + důvod), den shodný se souborem článku. `data/digest/YYYY-MM-DD.md` (+ `.json`) – denní digest. `data/feedback.jsonl` – zpětná vazba k příběhům. `data/fulltext/YYYY-MM-DD.jsonl` – plný text stažený u kandidátů digestu. `data/proposals.jsonl` – kdy byly naposledy navrženy úpravy `topics.md`.
- `web/digest.html` – šablona soukromé stránky s digestem (claude.ai artifact); data z `skimdigest export` do `data/web/digests.json`, URL stránky v `data/web/artifact-url`, hlasy 👍/👎 v databázi stránky (kolekce `votes`).
- `docs/superpowers/` – návrhy a plány implementace.
- `skills/` – skilly agenta (do `.claude/skills` připojené symlinkem).

## Skilly

- `skim` – **hlavní vstup na každý den**: stáhne, ohodnotí a ukáže digest ze všech témat (skládá `skim-triage` a `skim-digest`).
- `skim-discover` – k zadanému tématu najde, ověří a nabídne zdroje; po schválení je přidá do `feeds.yaml`.
- `skim-topics` – přehled témat s počty feedů; přidání, přejmenování, sloučení a smazání tématu (drží `topics.md` a `feeds.yaml` v souladu).
- `skim-fetch` – stáhne nové články do archivu a česky shrne, co přibylo a které feedy selhávají.
- `skim-triage` – ohodnotí nové články skóre 1–5 podle `topics.md` s českým důvodem.
- `skim-digest` – denní přehled: příběhy se shrnutím a odkazy do `data/digest/`, zkrácená verze v chatu, sběr zpětné vazby.

## Konvence

- Komunikace se mnou česky.
- Do `feeds.yaml` nikdy nepřidávej feed, který neprošel `feedcheck` (`ok: true`), a nikdy bez mého souhlasu.
- Go kód: jen standardní knihovna, dokud to jde; `gofmt`, `go vet` a testy musí projít.
- Ověření feedů: `go run ./cmd/feedcheck <url> [<url> ...]`
- Stažení článků: `go run ./cmd/skimfetch [-topic <slug>] [-dry-run]`
- Soubory v `data/articles/` nikdy nepřepisuj ani nemaž.
- Triage: `go run ./cmd/skimtriage pending` / `save` / `examples` (hlasy jako vodítko) / `report [-mark]` (návrh úprav témat)
- Digest: `go run ./cmd/skimdigest fulltext` / `candidates` / `render` / `feedback` / `export` (viz skill `skim-digest`, stránka viz skill `skim`)
