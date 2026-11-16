# skim-digest – návrh

## Záměr

Denní přehled místo procházení Feedly: z ohodnocených článků vybrat to
nejlepší, sloučit stejné zprávy z více zdrojů do „příběhů“, každý česky
shrnout a dát odkazy. Výsledek jako markdown soubor (archiv) + krátká
verze v chatu. Zpětnou vazbu (👍/👎) zatím jen sbírat.

## Cíle / ne-cíle

- Cíl: ~10–20 příběhů denně (pole **Objem** tématu), nejdůležitější nahoře.
- Cíl: odkazy vždy správné – nikdy je nepíše Claude ručně.
- Cíl: stálý formát; struktura uložená i strojově (JSON) pro budoucí stránku / `skim-search`.
- Cíl: zpětná vazba z chatu („3 dobrý, 7 nezajímavé“) se uloží.
- Ne-cíl: učení triage ze zpětné vazby (později), webová stránka, e-mail, plný text z webu.
- Anti-cíl: vymyšlené informace ve shrnutí (jen z toho, co je v článcích), rozbité odkazy.

## Přístupy

- A) Jen skill – Claude píše markdown i odkazy sám. Riziko špatných odkazů, kolísavý formát.
- B) Go `candidates`, markdown píše Claude – odkazy pořád v rukou Clauda.
- **C) Go `candidates` + `render` ze struktury od Clauda** – zvoleno. Claude dodá příběhy (titulek, shrnutí, ID článků), Go ověří a vykreslí podle šablony s odkazy z archivu.

## Návrh

### Den
Digest za den D = ohodnocené články ze souboru `data/articles/D.jsonl`
(den stažení). Výchozí D = dnešek (lokální čas), přepínač `-day`.

### Nástroj `cmd/skimdigest`
- `skimdigest candidates [-day D] [-min 3] [-excerpt 2000] [-data data]`
  – spojí `data/triage/D.jsonl` s články dne, vypíše JSONL se skóre ≥ min,
  seřazené skóre sestupně (při shodě pořadí v souboru):
  `id, topic, score, reason, feed_title, title, url, published, excerpt`.
  Poslední řádek `{"summary":true,"day":D,"candidates":N,"by_score":{"5":a,"4":b,"3":c},"untriaged":U}`
  (`untriaged` = články dne bez hodnocení – skill pak nabídne nejdřív triage).
- `skimdigest render [-day D] [-force] [-data data]` – ze stdin čte JSON:
  ```json
  {"stories":[{"topic":"ai-novinky","title":"…","summary":"…","ids":["…","…"]}],
   "more":["id", "…"]}
  ```
  Ověří (vše, nebo nic, chyby vypíše všechny): každé ID existuje, patří
  dni D a je ohodnocené; žádné ID dvakrát (v příbězích ani v `more`);
  příběh má ≥ 1 ID, neprázdný `title` (≤ 120 znaků) a `summary`
  (≤ 600 znaků); aspoň 1 příběh. Když `data/digest/D.md` existuje a není
  `-force` → chyba. Zapíše `data/digest/D.json` (vstup obohacený o
  číslo příběhu a skóre) a `data/digest/D.md`, vypíše
  `{"day":D,"stories":N,"articles":M,"path":"data/digest/D.md"}`.
- `skimdigest feedback [-day D] [-data data]` – ze stdin JSONL
  `{"story":3,"vote":"up"|"down","note":"…"}` (note volitelná ≤ 300 znaků).
  Ověří, že digest dne existuje a číslo příběhu v něm je; připíše do
  `data/feedback.jsonl` `{day, story, ids, vote, note, at}`. Vše, nebo nic.

### Šablona markdownu
```
# Skim – 16. 11. 2026

12 příběhů z 21 článků · ai-novinky

## ai-novinky

### 1. Claude Fable 5.2 je nový nejlepší model `5`
Shrnutí…

[Latent Space – Fable 5.2 retakes…](url) · [One Useful Thing – Fable 5.2 and…](url)

### 2. …

## Další zprávy
- [Titulek](url) – Feed
```
- Čísla příběhů jsou průběžná přes celý digest (na ně odkazuje zpětná vazba).
- Skóre příběhu = nejvyšší skóre jeho článků.
- Témata v pořadí prvního výskytu v `stories`; příběhy v pořadí, jak je Claude poslal.
- Sekce „Další zprávy“ jen když `more` není prázdné.

### Skill `skills/skim-digest/`
1. `candidates`; když `untriaged` > 0, nabídni nejdřív `skim-triage`.
2. Seskup kandidáty do příběhů podle události (stejný model, stejná kauza).
3. Do digestu dej příběhy, jejichž nejlepší článek má skóre ≥ 4, nejdůležitější první, max. horní mez **Objem** tématu. Skóre 3: vyber pár nejzajímavějších do `more` (max. 10), zbytek vynech.
4. Shrnutí česky, 2–4 věty, jen z `excerpt` a `reason` – nic nedomýšlej; u příběhu z víc zdrojů použij to nejlepší z nich. První ID = nejlepší zdroj (nejobsažnější).
5. `render` (heredoc), při chybě oprav a pošli znovu.
6. V chatu: seznam příběhů (číslo, titulek, jedna věta, odkaz na hlavní zdroj) + cesta k souboru. Vyzvi ke zpětné vazbě čísly.
7. Zpětnou vazbu z chatu zapiš přes `feedback`.

### Testy
`candidates`: spojení, filtr min, řazení, `untriaged`, chybějící den.
`render`: vykreslený markdown (golden test), každé pravidlo validace,
vše-nebo-nic, `-force`, odkazy z archivu. `feedback`: úspěch, neexistující
digest / číslo, neplatný vote, vše-nebo-nic. `gofmt`, `go vet`.

### Dokumentace
`CLAUDE.md`: struktura (`cmd/skimdigest/`, `data/digest/`, `data/feedback.jsonl`), skilly, příkazy.

## Moje rozhodnutí (k ověření)

- Digest za den stažení, ne za den publikace.
- Shrnutí 2–4 věty, max. 600 znaků; titulek příběhu píše Claude česky.
- Skóre 3 jako „Další zprávy“ jen titulky s odkazy, max. 10.
- Přepsání existujícího digestu jen s `-force`.
- Zpětná vazba po příbězích (číslo), ne po jednotlivých článcích.
