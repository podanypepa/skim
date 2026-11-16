# skim (denní běh) – návrh

## Záměr
Jeden příkaz na každý den: stáhnout, ohodnotit a ukázat digest ze všech
témat. Cíl je přehled, ne hledání (`skim-search` se ruší z plánu).

## Cíle / ne-cíle
- Cíl: `skim` = fetch → triage → digest → výsledek v chatu.
- Ne-cíl: nový Go kód, plánované spouštění, vlastní pravidla (bere je ze `skim-triage` a `skim-digest`).

## Přístupy
- **Skill skládající existující skilly a nástroje** – zvoleno; nástroje i pravidla existují.
- Go orchestrátor – zdvojil by logiku a neumí úsudek (triage, digest). Zamítnuto.

## Návrh (`skills/skim/SKILL.md`)
1. `go run ./cmd/skimfetch` → zapamatuj počet nových a feedy s `ok:false` (a `failures ≥ 3`).
2. Triage podle `skim-triage` (smyčka pending → save), dokud `pending` není 0.
3. Digest za dnešek podle `skim-digest`:
   - `data/digest/<dnes>.json` neexistuje → sestav;
   - existuje a všechna ID kandidátů se skóre ≥ 4 už v něm jsou → jen ukaž existující;
   - existuje a přibyly nové ≥ 4 → sestav znovu s `-force` a řekni to.
4. Chat: řádek statistiky (staženo / ohodnoceno / příběhů / selhalo feedů) + digest po tématech jako `skim-digest` krok 4; u feedů s `failures ≥ 3` nabídni prověření.
5. Zpětná vazba dál přes `skim-digest`.

## Moje rozhodnutí
- Přepis existujícího digestu bez ptaní, když přibylo nové ≥ 4 (spuštění `skim` = chci aktuální stav; hlasy jsou uložené po článcích).
- `CLAUDE.md`: `skim` jako hlavní vstup, `skim-search` pryč z plánovaných.
