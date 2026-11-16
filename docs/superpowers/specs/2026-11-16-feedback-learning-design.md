# Učení ze zpětné vazby – návrh

## Záměr
Hlasy 👍/👎 k příběhům digestu (`data/feedback.jsonl`) mají zpřesňovat
triage: jako příklady při hodnocení a jako podklad pro týdenní návrh
úprav `topics.md`. Pravidla v `topics.md` zůstávají zdrojem pravdy.

## Cíle / ne-cíle
- Cíl: triage vidí posledních ~30 hlasů (hlavně nesoulady) jako vodítko.
- Cíl: jednou za čas (týden / 20 hlasů) návrh úprav `topics.md` s doložením; projde jen schválené.
- Ne-cíl: automatické přepisování `topics.md`, přehodnocení už ohodnocených článků, webová stránka (bod 3, zvlášť).
- Anti-cíl: příklady přebijí `topics.md`; otravné opakované návrhy.

## Přístupy
- A) Jen skilly – Claude ručně spojuje tři soubory, křehké.
- **B) Rozšíření `skimtriage` + skilly** – zvoleno; spojování a počítání v Go s testy, formulace návrhů ve skillu.

## Návrh

### `skimtriage examples [-n 30] [-data data]`
- Načte `feedback.jsonl`, každé ID z `ids` spojí s článkem (celý archiv, první výskyt) a jeho triage záznamem.
- Na článek platí **poslední** hlas (dřívější přepíše).
- Řádek JSONL: `id, topic, feed_title, title, score, reason, vote, note, day, mismatch`;
  `mismatch` = (`down` a skóre ≥ 4) nebo (`up` a skóre ≤ 3).
- Pořadí: nejdřív nesoulady, pak ostatní; v rámci skupiny od nejnovějšího. Max. `-n`.
- Články bez triage nebo mimo archiv přeskočí. Bez `feedback.jsonl` → prázdný výstup + summary.
- Poslední řádek `{"summary":true,"examples":N,"mismatches":M}`.

### `skimtriage report [-mark] [-data data]`
- Agregace hlasů (na článek, poslední hlas platí):
  - `by_feed`: `{feed_title, topic, up, down, avg_score}`,
  - `by_score`: pro skóre 1–5 `{up, down}`,
  - `notes`: poznámky z hlasů od posledního návrhu.
- „Od posledního návrhu“ = hlasy s `at` po posledním záznamu v `data/proposals.jsonl`.
- `due` = nových hlasů ≥ 20, nebo (≥ 5 a od posledního návrhu ≥ 7 dní; bez návrhu se počítá od prvního hlasu).
- Výstup jeden JSON objekt: `{due, new_votes, total_votes, days_since_last, by_feed, by_score, notes}`.
- `-mark` připíše `{at, votes}` do `data/proposals.jsonl` (volá skill po předložení návrhu, ať už schváleném či ne).

### Skilly
- **skim-triage**: před první dávkou spusť `examples`; příklady ber jako vodítko pro jemné rozdíly, při rozporu s `topics.md` má `topics.md` přednost.
- **skim** (konec běhu): spusť `report`; když `due`, navrhni 1–3 konkrétní úpravy polí **Zajímá mě / Nezajímá mě** s doložením (feed, počty, příklady titulků), uprav jen po souhlasu; pak `report -mark`.
- **skim-digest**: výzvu ke zpětné vazbě ponech; u příběhů z víc zdrojů platí hlas pro všechny jejich články (už tak funguje).

### Testy
`examples`: spojení, poslední hlas vyhrává, mismatch, pořadí, limit, chybějící feedback/triage.
`report`: agregace, `due` (prahy 20 / 5+7 dní, bez historie), `-mark` připíše, notes jen nové.
`gofmt`, `go vet`, CI.

## Moje rozhodnutí
- Limit příkladů 30, prahy 20 hlasů / 5 hlasů + 7 dní.
- Návrh se označí jako předložený i při odmítnutí (žádné opakované otravování).
- Hlas k příběhu platí pro všechny jeho články.
