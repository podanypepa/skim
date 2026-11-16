# Plný text u nejlepších článků – návrh

## Záměr
Řada feedů dává jen titulek a perex (OpenAI, DeepMind, Lupa) nebo nic
(Hugging Face); u Redditu a Lobsters je obsah jen „Comments“ /
„submitted by“. Shrnutí v digestu jsou pak tenká. Před digestem stáhnout
plný text jen u kandidátů, kteří ho potřebují.

## Cíle / ne-cíle
- Cíl: kandidáti se skóre ≥ 3 a krátkým obsahem (< 600 znaků) dostanou plný text z webu, ~10–20 stránek denně.
- Cíl: u Redditu a Lobsters se stáhne odkazovaný článek, ne diskuze.
- Cíl: `candidates` použije plný text místo krátkého obsahu, shrnutí čerpají z něj.
- Ne-cíl: plný text před triage, u všech článků, obcházení paywallů, JavaScriptové stránky.
- Anti-cíl: přepisování archivu článků; opakované stahování stejné stránky; ignorování robots.txt.

## Přístupy
- **Go podpříkaz se stdlib extrakcí textu** – zvoleno: uložené, testovatelné, zdarma.
- Claude WebFetch – lepší extrakce, ale placené za stránku, nic se neukládá, netestovatelné.

## Návrh

### `skimdigest fulltext [-day D] [-min 3] [-short 600] [-data data]`
1. Kandidáti dne se skóre ≥ `min`, jejichž `content` má < `short` znaků a ještě nemají záznam v `data/fulltext/D.jsonl`.
2. Cílová URL:
   - Reddit (`reddit.com/r/…/comments/…`): stáhni `<url>.json`, vezmi `data.url` prvního příspěvku; když míří zpět na Reddit (self post), použij `selftext` jako obsah bez dalšího stahování.
   - Jinak URL článku (u Lobsters je to už odkazovaný článek).
3. robots.txt: před stažením ověř `Disallow` pro `*` a `skim-fetch` (jednoduchý parser, cache na doménu); zakázané → status `robots`.
4. Stažení: souběh 4, timeout 20 s, User-Agent jako skimfetch, max. 5 MB, jen `text/html`.
5. Extrakce (`internal/feed/extract.go`): odstraň `script`, `style`, `nav`, `header`, `footer`, `aside`, `form`; region = první `<article>`, jinak `<main>`, jinak celé `<body>`; z regionu vezmi text odstavců a nadpisů (`p`, `h1–h4`, `li`, `pre`, `blockquote`) přes `HTMLToText`; max. 20 000 znaků. Výsledek < 300 znaků → status `empty`.
6. Zápis (append, vše najednou na konci): `data/fulltext/D.jsonl` řádek `{id, url, status, content, fetched_at}`; `status` ∈ `ok | empty | robots | error`, u chyby `error`. Každé ID se zkouší jen jednou.
7. Výstup JSONL řádek na článek + summary `{summary, tried, ok, failed}`.

### Napojení
- `loadDay` načte `data/fulltext/D.jsonl`; když je `status: ok` a text je delší než obsah z feedu, `candidates` použije plný text jako zdroj `excerpt`.
- Skill **skim-digest** krok 1: nejdřív `fulltext`, pak `candidates`. Skill **skim** to dědí.

### Testy
Extrakce (article / main / body, odstranění nav/footer, krátký výsledek → empty), výběr kandidátů (skóre, délka, už stažené), Reddit link post i self post (httptest), robots.txt zákaz, chyby HTTP / ne-HTML, jen jeden pokus na ID, `candidates` použije plný text. `gofmt`, `go vet`, CI.

## Moje rozhodnutí
- Práh krátkého obsahu 600 znaků, minimum výsledku 300, souběh 4.
- robots.txt respektovat (osobní použití, ale slušnost).
- Neúspěšná stránka se nezkouší znovu (žádné opakované bušení do serveru).
