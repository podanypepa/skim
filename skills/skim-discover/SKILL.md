---
name: skim-discover
description: Najde, ověří a nabídne RSS/Atom zdroje k zadanému tématu a po schválení je přidá do feeds.yaml. Použij, když chci začít sledovat nové téma, rozšířit nebo pročistit zdroje k existujícímu tématu, nebo se ptám „kde sledovat X“.
---

# skim-discover

Cíl: k tématu najít malou sadu kvalitních, živých zdrojů s RSS/Atom feedem,
ne dlouhý seznam všeho, co existuje. Raději 8 výborných než 30 průměrných.

## 1. Upřesni téma

Přečti `topics.md`. Pokud téma chybí nebo má nevyplněná pole, zeptej se
mě v jedné zprávě (max. 4 krátké otázky, ideálně s nabídkou možností) na:
- co přesně mě v tématu zajímá a co ne,
- hloubku: rychlé novinky / praktické věci (nástroje, návody) / do hloubky (výzkum, analýzy),
- jazyky (cs, en, …),
- objem: kolik článků denně z tématu chci zhruba vidět.

Odpovědi zapiš do sekce tématu v `topics.md` (podle šablony v souboru).
Pokud je téma už vyplněné, neptej se znovu a pokračuj.

## 2. Zjisti, co už odebírám

Přečti `feeds.yaml`, ať nenavrhuješ duplicity, a u existujícího tématu
víš, co chybí.

## 3. Hledej kandidáty

Hledej z více úhlů, ne jedním dotazem. Cíl je 15–30 kandidátů:
- přímé zdroje: blogy firem, laboratoří a projektů, osobní blogy uznávaných lidí v oboru,
- kurátorské newslettery (Substack a Buttondown mají RSS: `<web>/feed`, resp. `/rss`),
- seznamy „best X blogs“, „awesome-X“ repozitáře na GitHubu, doporučení z komunit,
- agregátory a komunity s RSS – jen jako doplněk, mají vysoký objem:
  - Hacker News s filtrem: `https://hnrss.org/newest?q=<dotaz>&points=100`
  - Reddit: `https://www.reddit.com/r/<sub>/top/.rss?t=week`
  - arXiv kategorie: `https://rss.arxiv.org/rss/<kategorie>` (např. `cs.AI`)
  - YouTube kanál: `https://www.youtube.com/feeds/videos.xml?channel_id=<id>`
  - GitHub releases: `https://github.com/<owner>/<repo>/releases.atom`
- u českých zdrojů hledej i česky.

URL feedu si nevymýšlej. Stačí URL webu – `feedcheck` feed najde sám.

## 4. Ověř všechny kandidáty najednou

```
go run ./cmd/feedcheck <url1> <url2> ...
```

Výstup je JSONL: `ok`, `feed_url`, `title`, `items`, `latest`,
`days_since_latest`, `posts_per_week`, `samples` (poslední titulky).

Vyřaď:
- `ok: false` (zkus nanejvýš jednu alternativní URL, pak vzdej),
- mrtvé zdroje: `days_since_latest` > 90 (výjimka: vzácný, vysoce kvalitní zdroj s nízkou frekvencí – uveď to),
- zdroje, jejichž `samples` neodpovídají tématu nebo jsou převážně PR, clickbait či obsah mimo moje zájmy.

## 5. Vyhodnoť a nabídni

U každého zbylého zdroje posuď podle `samples` a `topics.md`:
relevanci, poměr signál/šum, originalitu (primární zdroj vs. přebírání),
překryv s ostatními a objem (`posts_per_week`) vůči mému limitu.

Nabídni 8–15 zdrojů jako tabulku rozdělenou do skupin:
- **Jádro** – doporučuji odebírat,
- **Doplňkové** – užitečné, ale volitelné,
- **Vysoký objem** – agregátory a komunity; vhodné jen s filtrem.

Sloupce: název, typ, frekvence (příspěvků/týden), poslední příspěvek,
proč ano (jedna věta). Pod tabulku dej odhad celkového objemu doporučené
sady za den a porovnej ho s mým limitem. Pak se zeptej, které přidat
(výchozí návrh: celé Jádro).

## 6. Po schválení zapiš

Do `feeds.yaml` přidej jen schválené zdroje podle schématu v hlavičce
souboru. `url` = `feed_url` z feedcheck, `added` = dnešní datum, `why` =
jedna věta. Zachovej existující položky a komentáře. Na závěr shrň, kolik
feedů téma teď má.

## Pravidla

- Nikdy nepřidávej nic bez mého výslovného souhlasu.
- Nikdy nepřidávej feed, který neprošel feedcheck s `ok: true`.
- Preferuj primární zdroje před agregátory, které jen přebírají.
- Když chci téma pročistit, navrhni i odebrání (mrtvé, nerelevantní, duplicitní) – stejným postupem, se schválením.
