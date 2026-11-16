---
name: skim-triage
description: Ohodnotí nově stažené články skóre 1–5 podle topics.md a zapíše důvod. Použij po skim-fetch, když chci roztřídit články nebo vědět, co z nového stojí za přečtení.
---

# skim-triage

Úsudek (co je relevantní) děláš ty. Evidenci a validaci dělá Go nástroj
`cmd/skimtriage`: `pending` vypíše neohodnocené články, `save` uloží
hodnocení do `data/triage/<den>.jsonl` (vše, nebo nic).

## Škála

- **5** – musím vidět: zásadní novinka v mých tématech (nový velký model, důležitý nástroj, klíčová regulace).
- **4** – stojí za přečtení: relevantní a s obsahem.
- **3** – možná: k tématu, ale okrajové nebo slabší.
- **2** – spíš ne: slabá souvislost, převážně PR.
- **1** – šum: mimo téma, clickbait, nebo něco z „Nezajímá mě“.

Digest bere skóre ≥ 4 (do limitu **Objem** tématu), skóre 3 jako zásobu.
Buď přísný: pětek je málo, čtyřky musí mít skutečný obsah.

## Postup

1. Přečti `topics.md` – u každého tématu pole **Zajímá mě**, **Nezajímá mě**, **Hloubka**.
   Pak `go run ./cmd/skimtriage examples` – moje nedávné hlasy 👍/👎 k článkům
   (nesoulady s tvým skóre jsou první). Ber je jako vodítko pro jemné
   rozdíly; při rozporu s `topics.md` platí `topics.md`. Žádné hlasy → pokračuj.
2. Vypiš dávku:
   ```
   go run ./cmd/skimtriage pending
   ```
   JSONL: `id, day, topic, feed_title, title, url, published, excerpt`;
   poslední řádek `{"summary":true,"pending":N,"shown":M}`. Když je
   `pending` 0, přeskoč na shrnutí.
3. Ohodnoť každý článek dávky vůči **jeho tématu** (`topic`):
   - „Nezajímá mě“ → 1–2.
   - Krátký nebo prázdný `excerpt` → hodnoť podle titulku a feedu.
   - `reason`: jedna věta česky, konkrétní – co článek přináší, nebo proč je šum (ne „relevantní článek o AI“).
4. Ulož celou dávku najednou:
   ```
   go run ./cmd/skimtriage save <<'EOF'
   {"id":"…","score":4,"reason":"…"}
   EOF
   ```
   `score` je celé číslo bez uvozovek. Při chybě se neuloží nic a vypíší
   se chybné řádky – oprav je a pošli celou dávku znovu.
5. Opakuj od kroku 2, dokud `pending` není 0.

## Shrnutí

Česky a stručně:
- počty podle skóre (5 / 4 / 3 / 2 / 1),
- tabulka skóre 5 a 4: titulek (jako odkaz na `url`), feed, důvod,
- kolik článků je šum (1–2) a odkud hlavně.

Když vidíš opakovaný vzor (např. „případové studie zákazníků OpenAI“
vždy 1–2), navrhni konkrétní doplnění pole **Nezajímá mě** / **Zajímá mě**
v `topics.md`. Uprav ho jen po mém souhlasu.

## Pravidla

- Neměň soubory v `data/articles/` ani existující řádky v `data/triage/`.
- Neodhaduj obsah článku, který v `excerpt` není; nestahuj články z webu.
- Už ohodnocený článek nepřehodnocuj.
