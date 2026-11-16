---
name: skim-fetch
description: Stáhne nové články ze všech odebíraných feedů (feeds.yaml) do archivu data/. Použij, když chci stáhnout nebo aktualizovat články, ptám se „co je nového“, nebo před triage či digestem.
---

# skim-fetch

Stahování je deterministická práce – dělá ji Go nástroj `./skim fetch`.
Tvoje role: spustit ho, česky shrnout výsledek a ohlídat selhávající feedy.

## 1. Spusť

```
./skim fetch
```

Jen jedno téma: `-topic <slug>`. Jen zkouška bez zápisu: `-dry-run`.

Výstup je JSONL: řádek na feed
(`feed_url`, `title`, `topic`, `ok`, `status`, `new`, `error`, `failures`)
a na konci souhrn (`summary: true`, `feeds`, `ok`, `failed`, `new`).
`status`: `new` (přibyly články), `no_new`, `not_modified` (server
hlásí beze změny), `error`.

Nenulový exit = chyba konfigurace nebo úložiště (např. rozbitý
`feeds.yaml`) – vypiš chybu a nic dalšího nedělej.

## 2. Shrň

Česky a stručně:
- celkem nových článků a do kterého souboru přibyly (`data/articles/<dnešní datum>.jsonl`),
- tabulka po tématech a feedech – jen feedy s `new > 0` (téma, feed, nových),
- selhané feedy s chybou (pokud nějaké jsou).

## 3. Ohlídej selhávající feedy

Feed s `failures >= 3` (selhal 3× po sobě):
1. ověř ho: `./skim check <url>`,
2. když feedcheck najde jinou funkční URL, navrhni opravu `url` ve `feeds.yaml`,
3. když je zdroj mrtvý, navrhni odebrání.

Změny ve `feeds.yaml` proveď jen po mém výslovném souhlasu.

## 4. Nabídni další krok

Až bude existovat `skim-triage`, nabídni roztřídění nových článků.

## Pravidla

- Nikdy nemaž ani neupravuj soubory v `data/articles/` – archiv je append-only.
- `data/state.json` needituj ručně (pamatuje si, co už je stažené).
- Nestahuj plný text článků z webu – archiv obsahuje jen to, co je ve feedu.
