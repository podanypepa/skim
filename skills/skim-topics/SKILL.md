---
name: skim-topics
description: Správa témat ve skim – vypíše přehled témat s počty feedů, přidá, přejmenuje nebo smaže téma (včetně jeho feedů) a najde nesrovnalosti mezi topics.md a feeds.yaml. Použij, když se ptám „jaká mám témata“, chci téma přidat, přejmenovat, sloučit nebo zrušit.
---

# skim-topics

Témata žijí v `topics.md` (sekce `## <slug>`), feedy v `feeds.yaml`
(pole `topic` odkazuje na slug). Tento skill drží oba soubory v souladu.

Podle toho, co chci, proveď jednu z akcí. Bez upřesnění = **přehled**.

## Přehled (výchozí)

Přečti `topics.md` a `feeds.yaml` a vypiš tabulku:

| téma | popis | feedů | objem (cíl) | poslední přidaný feed |

- `popis` zkrať na jednu krátkou větu,
- `objem (cíl)` je hodnota pole **Objem** z `topics.md`,
- `poslední přidaný feed` = nejnovější `added` u feedů tématu.

Pod tabulku vypiš jen pokud existují:
- **Témata bez feedů** – nabídni `/skim-discover <téma>`,
- **Nevyplněná témata** – pole s `_(doplní se)_`,
- **Osiřelé feedy** – `topic` ve `feeds.yaml`, který v `topics.md` neexistuje; nabídni přiřazení k existujícímu tématu nebo založení tématu.

## Přidat téma

- Když chci rovnou i zdroje (nebo neřeknu), předej práci skillu `skim-discover` – ten se doptá a sekci zapíše.
- Když chci jen založit téma bez zdrojů, doptej se na pole ze šablony v `topics.md` (max. 4 otázky v jedné zprávě) a přidej sekci na konec souboru.
- Slug: malá písmena, pomlčky, bez diakritiky (`ai-novinky`, `rust-go`). Ověř, že ještě neexistuje.

## Přejmenovat téma

1. Ověř, že nový slug neexistuje.
2. V `topics.md` změň nadpis `## <starý>` na `## <nový>`.
3. Ve `feeds.yaml` změň `topic: <starý>` na `topic: <nový>` u všech feedů.
4. Oznam, kolik feedů se přesunulo.

## Sloučit témata

Postup jako přejmenování starého tématu na cílové; sekci starého tématu
z `topics.md` odstraň a důležité body z jeho polí **Zajímá mě** / **Nezajímá mě**
navrhni připojit k cílovému tématu (se schválením).

## Smazat téma

1. Ukaž, co se smaže: sekci tématu a seznam jeho feedů (název, URL).
2. Zeptej se, co s feedy: **smazat**, nebo **přesunout** pod jiné téma.
3. Teprve po výslovném souhlasu proveď změny v obou souborech.

## Pravidla

- Mazání a slučování vždy až po výslovném souhlasu.
- Měň jen dotčené řádky; zachovej komentáře, pořadí a ostatní položky v obou souborech.
- Po každé změně ověř, že `feeds.yaml` je stále validní YAML (např. `python3 -c "import yaml,sys; yaml.safe_load(open('feeds.yaml'))"`, pokud je PyYAML k dispozici) a že žádný feed neodkazuje na neexistující téma.
