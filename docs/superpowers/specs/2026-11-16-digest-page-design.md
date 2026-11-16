# Digest jako webová stránka – návrh

## Záměr
Denní digest číst jako soukromou stránku na claude.ai (i z telefonu),
s jedním stálým odkazem, přepínačem posledních 14 dní a tlačítky 👍/👎
u příběhů. Hlasy se při dalším `skim` převezmou do `data/feedback.jsonl`,
takže učení triage (bod 1) funguje beze změny.

## Cíle / ne-cíle
- Cíl: jedna stránka, stálý odkaz, nejnovější den nahoře, přepínač 14 dní.
- Cíl: 👍/👎 u příběhu, hlas vidět i po přenačtení; převzetí do feedbacku bez duplicit.
- Cíl: odkazy a texty jen z `data/digest/*.json` (archiv), nikdy psané ručně.
- Ne-cíl: veřejné sdílení, plné texty článků na stránce, okamžité promítnutí hlasů do triage, push notifikace.
- Anti-cíl: osobní data v public repu (do repa jde jen šablona).

## Přístupy
- A) Go generuje celé HTML – vzhled v Go řetězcích, nepohodlné.
- **B) Šablona `web/digest.html` + data z `skimdigest export`** – zvoleno.
- C) Claude píše HTML denně – kolísavý vzhled, ruční odkazy. Zamítnuto.

## Návrh

### `skimdigest export [-days 14] [-out data/web/digests.json] [-data data]`
- Načte posledních N souborů `data/digest/*.json` (podle data v názvu), nejnovější první.
- Zapíše JSON: `{"generated_at": RFC3339, "days": [{"day", "stories": [{"n","topic","title","summary","score","articles":[{"id","feed_title","title","url"}]}], "more": [{"id","feed_title","title","url","score"}]}]}`.
- Zápis atomicky; bez digestů → `days: []`.

### Šablona `web/digest.html` (v repu)
- Načte `digests.json` (relativní cesta). Selhání → srozumitelná hláška.
- Hlavička: „Skim“, vybraný den (česky „16. 11. 2026“), přepínač dnů (čipy/šipky).
- Sekce po tématech; karta příběhu: číslo, titulek, skóre (štítek), shrnutí, zdroje jako odkazy (`target=_blank`, `rel=noopener`), tlačítka 👍/👎.
- „Další zprávy“ sbalitelné.
- Mobil první (16px okraj, žádný horizontální scroll), světlý i tmavý režim přes tokeny.
- Texty vkládat přes `textContent`, URL jen `http(s)`.

### Hlasy (capability `db`)
- Kolekce `votes`, dokument `<day>_<n>`: `{day, story, title, vote: "up"|"down", at, imported: false}`.
- Klik nastaví hlas; klik na stejný hlas ho zruší (smaže dokument, jen pokud `imported` je false; převzatý hlas jde jen přepnout).
- Stav tlačítek z `onSnapshot`; bez `db` (null) se tlačítka skryjí a stránka funguje jen ke čtení.
- Pravidla výchozí (zapisuje vlastník/editoři); stránka je soukromá.

### Převzetí hlasů (skill `skim`, na začátku běhu)
1. `ArtifactData list votes` → dokumenty s `imported: false`.
2. Po dnech poslat `go run ./cmd/skimdigest feedback -day D` řádky `{"story":n,"vote":v,"title":t}`.
3. `skimdigest feedback` nově přijímá volitelné `title`: když nesedí s titulkem příběhu v `D.json` (digest mezitím přestavěn), je to chyba řádku (vše, nebo nic); skill takové řádky vynechá, zbytek pošle znovu a nahlásí je.
4. Převzaté dokumenty `ArtifactData batch update imported: true`, vynechané `imported: "skipped"` (dávky po 50, vždy s `if_version`). Stránka ukazuje hlas jen u příběhu se stejným titulkem a „převzato“ jen pro `imported === true`.

### Publikace (skill `skim`, po digestu)
- `skimdigest export` → `data/web/digests.json`.
- Artifact publish: `file_path: web/digest.html`, `files: {"digests.json": "data/web/digests.json"}`, capabilities `{db: {}}`.
- URL stránky v `data/web/artifact-url` (git-ignored); existuje → publish na tu URL, jinak první publish a URL uložit.
- V chatu odkaz na stránku vedle krátkého seznamu.

### Testy
`export`: výběr N dní, pořadí, struktura, prázdný stav. `feedback` s `title`: shoda / neshoda / bez `title`. Šablona: ruční ověření publikací a jedním `ArtifactData list`. `gofmt`, `go vet`, CI.

## Moje rozhodnutí
- 14 dní historie; pořadí dnů od nejnovějšího.
- Převzatý hlas nejde smazat, jen přepnout (feedback platí poslední).
- Kontrola titulku chrání před hlasem k jinému příběhu po `-force`.
- Vzhled: střídmý, čitelný, karty; doladíme po prvním publikování.
