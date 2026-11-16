---
name: skim
description: Denní přehled jedním příkazem – stáhne nové články, ohodnotí je a ukáže digest ze všech témat. Použij, když chci denní přehled, „co je nového“ nebo prostě „skim“.
---

# skim – denní běh

Skládá tři kroky; jejich pravidla jsou v příslušných skillech a tady se
neopakují. Den = dnešek (lokální čas).

## 1. Stáhni

```
go run ./cmd/skimfetch
```

- Nenulový exit (rozbitý `feeds.yaml`, úložiště) → vypiš chybu a skonči.
- Zapamatuj si z posledního řádku `new` a ze řádků feedů ty s `ok:false`
  (zvlášť ty s `failures >= 3`).

## 2. Ohodnoť

Postupuj podle skillu **skim-triage** (přečti `topics.md`, smyčka
`go run ./cmd/skimtriage pending` → ohodnoť → `save`), dokud `pending`
není 0. Když je `pending` hned 0, krok přeskoč. Závěrečné shrnutí triage
nevypisuj zvlášť – stačí počet ohodnocených do statistiky.

## 3. Digest za dnešek

Zjisti stav:
- `data/digest/<dnes>.json` neexistuje → sestav digest podle skillu **skim-digest** (kroky 1–3).
- Existuje → porovnej ID z `go run ./cmd/skimdigest candidates -min 4`
  s ID v `stories[].articles[].id` a `more[].id` v tom JSON:
  - všechna už tam jsou → digest nepřestavuj, jen ho ukaž,
  - přibyla nová → sestav digest znovu (celý, ze všech kandidátů dne) a pošli ho s `-force`; v chatu řekni, že byl aktualizován.
- `candidates` je 0 a digest neexistuje → řekni, že dnes není z čeho skládat.

## 4. Ukaž

1. Jeden řádek statistiky: `Staženo N · ohodnoceno M · příběhů K · selhalo F feedů`.
2. Digest jako v kroku 4 skillu **skim-digest**: po tématech, číslo, **titulek**, jedna věta, odkaz na hlavní zdroj (URL z `data/digest/<dnes>.json`, `articles[0].url`), cesta k souboru, výzva ke zpětné vazbě.
3. Feedy s `failures >= 3`: vyjmenuj je a nabídni prověření (`go run ./cmd/feedcheck <url>`); změny `feeds.yaml` jen s mým souhlasem.

## 5. Zpětná vazba

Řeší ji skill **skim-digest** (krok 5) – vždy s `-day` dne digestu.

## 6. Návrh úprav témat

```
go run ./cmd/skimtriage report
```

Když `due` je `true`:
1. Z `by_feed`, `by_score`, `notes` a z `go run ./cmd/skimtriage examples`
   najdi 1–3 jasné vzory (např. „r/golang: 6× 👎 u začátečnických dotazů“).
2. Navrhni konkrétní úpravy polí **Zajímá mě / Nezajímá mě** v `topics.md`
   s doložením (feed, počty, 2–3 titulky).
3. Uprav `topics.md` jen po mém souhlasu.
4. Pak vždy (i při odmítnutí) `go run ./cmd/skimtriage report -mark`.

Když `due` je `false`, krok přeskoč a nic nevypisuj.
