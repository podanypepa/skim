# skim-triage – návrh

## Záměr

Ke každému nově staženému článku určit, jak moc odpovídá mým zájmům
podle `topics.md`, s krátkým důvodem. Z hodnocení pak `skim-digest`
vybere 10–20 článků denně. Úsudek dělá Claude ve skillu; Go nástroj
hlídá evidenci (co je ohodnocené) a platnost zápisu.

## Cíle / ne-cíle

- Cíl: každý článek ohodnocen právě jednou, se skóre a důvodem česky.
- Cíl: chyby triage jdou snadno pochopit (důvod) a opravit (úprava `topics.md`).
- Ne-cíl: výběr do digestu, deduplikace stejné zprávy z více feedů, shrnutí (to je `skim-digest`).
- Ne-cíl (zatím): učení ze zpětné vazby 👍/👎 – přijde s digestem.
- Ne-cíl: volání Claude API z Go.
- Anti-cíl: úprava `data/articles/`; nečitelná nebo nevysvětlená skóre.

## Přístupy

- A) Jen skill – Claude sám čte archiv a píše JSON. Křehké: evidence, ruční JSON, plné texty v kontextu.
- **B) Skill + Go nástroj `skimtriage` (pending/save)** – zvoleno. Drží rozdělení projektu (úsudek ve skillu, deterministická práce v Go).
- C) Go volá Claude API – nezávislé na Claude Code, ale API klíč, platba zvlášť, proti současnému rozdělení. Možné později nad stejnými daty.

## Návrh

### Škála (zapsat do skillu)
- **5** – musím vidět: zásadní novinka v mých tématech (nový velký model, důležitý nástroj, klíčová regulace).
- **4** – stojí za přečtení: relevantní a s obsahem.
- **3** – možná: k tématu, ale okrajové nebo slabší.
- **2** – spíš ne: slabá souvislost, převážně PR.
- **1** – šum: mimo téma, clickbait, nebo něco z „Nezajímá mě“.

Digest pak bere skóre ≥ 4 (do limitu **Objem** tématu), skóre 3 jako zásobu.

### Úložiště
`data/triage/YYYY-MM-DD.jsonl` – den = stejný jako soubor článku
v `data/articles/`, jen append. Řádek:
`id, topic, score (1–5), reason (česky, max. 300 znaků), triaged_at (RFC3339)`.

### Nástroj `cmd/skimtriage`
- `skimtriage pending [-limit 25] [-days 7] [-topic slug] [-data data] [-excerpt 1500]`
  – projde `data/articles/` za posledních N dní, vynechá ID už
  v `data/triage/`, vypíše JSONL nejstarší první:
  `id, day, topic, feed_title, title, url, published, excerpt`
  (prvních N znaků obsahu, ořez po runách). Poslední řádek
  `{"summary":true,"pending":<celkem>,"shown":<v dávce>}`.
- `skimtriage save [-data data]` – ze stdin čte JSONL `{id, score, reason}`.
  Ověří: ID existuje v archivu a ještě není ohodnocené, není v dávce 2×,
  `score` je celé číslo 1–5, `reason` není prázdný a má ≤ 300 znaků.
  **Vše, nebo nic**: při jakékoli chybě nic nezapíše a vypíše všechny
  chyby s číslem řádku. Při úspěchu doplní `topic` a den z článku,
  připíše do `data/triage/<den>.jsonl` a vypíše `{"saved":N}`.
- Archiv čte přes `internal/store` (přidat čtecí funkce); jen stdlib.

### Skill `skills/skim-triage/`
1. Přečti `topics.md` (pole Zajímá mě / Nezajímá mě / Hloubka).
2. Smyčka: `skimtriage pending` → ohodnoť dávku podle škály → `skimtriage save` (heredoc) → dokud `pending` není 0.
3. Článek s prázdným nebo krátkým obsahem hodnoť podle titulku a feedu; v důvodu to nezmiňuj zbytečně.
4. Shrň česky: počty podle skóre, seznam skóre 5 a 4 (titulek, feed, důvod), kolik článků je šum.
5. Když vidíš opakovaný vzor (např. „případové studie zákazníků OpenAI“ vždy 1–2), navrhni doplnění `topics.md` – jen po mém souhlasu.

### Testy
`pending`: vynechá ohodnocené, filtr dní a tématu, limit, ořez po runách,
pořadí. `save`: úspěch, každé pravidlo validace, vše-nebo-nic, duplicita
v dávce i vůči už uloženým. `gofmt`, `go vet`.

### Dokumentace
`CLAUDE.md`: struktura (`cmd/skimtriage/`, `data/triage/`), skilly.

## Moje rozhodnutí (k ověření)

- Škála 1–5 s významy výše; práh pro digest ≥ 4.
- Hodnotí se titulek + prvních 1 500 znaků, ne celý text.
- Dávka 25 článků; okno 7 dní (starší neohodnocené se ignorují).
- Přehodnocení už ohodnoceného článku zatím nejde (YAGNI); případně později `-force`.
- Duplicitní zprávy z více feedů řeší až digest.
