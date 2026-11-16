# Jeden program `skim` a hotové binárky – návrh

## Záměr
Aby skim mohl používat i někdo bez Go: jeden spustitelný soubor `skim`
pro macOS / Linux / Windows ke stažení z GitHub Releases. Druhá
implementace (Python) zamítnuta – dvojí údržba, riziko rozjetí formátů.

## Cíle / ne-cíle
- Cíl: `skim <příkaz>` nahrazuje čtyři nástroje; chování a výstupy beze změny.
- Cíl: současné `go run ./cmd/skimfetch` atd. funguje dál.
- Cíl: skilly fungují s binárkou i bez ní (fallback na `go run`).
- Cíl: release workflow při tagu `v*` sestaví binárky + checksumy.
- Ne-cíl: Homebrew tap, auto-update, podepisování binárek.
- Měřeno: `go run` 0,11 s vs. binárka 0,07 s na volání – rychlost není důvod.

## Přístupy
- **Jeden program s podpříkazy + tenké staré `cmd/*`** – zvoleno.
- Čtyři binárky – víc souborů ke stažení a v PATH.
- Paralelní implementace v Pythonu – zamítnuto.

## Návrh
- Logika nástrojů se přesune z `package main` do `internal/app/{check,fetch,triage,digest}`,
  každý s `func Main(args []string, stdout, stderr io.Writer) int` (návratový kód).
  Stávající testy se přesunou s kódem.
- `cmd/feedcheck`, `cmd/skimfetch`, `cmd/skimtriage`, `cmd/skimdigest` zůstanou jako
  tenké obálky: `os.Exit(app.Main(os.Args[1:], os.Stdout, os.Stderr))`.
- `cmd/skim`: `skim check …` (feedcheck), `skim fetch …`, `skim triage <pending|save|examples|report> …`,
  `skim digest <fulltext|candidates|render|feedback|export> …`, `skim version`, `skim help`;
  neznámý příkaz → nápověda na stderr, kód 2. Verze přes `-ldflags "-X main.version=…"`.
- Skript `./skim` v kořeni repa (bash): když je v PATH program `skim`, spustí ho, jinak
  `go run ./cmd/skim`. Skilly a CLAUDE.md volají jednotně `./skim …`.
- `.github/workflows/release.yml`: na tag `v*` sestaví `skim` pro darwin/amd64, darwin/arm64,
  linux/amd64, linux/arm64, windows/amd64 (`CGO_ENABLED=0`, `-trimpath`), zabalí
  (`tar.gz`, Windows `zip`), `SHA256SUMS`, vytvoří GitHub Release (`gh release create`).
- README: instalace (stáhnout z Releases a dát do PATH, nebo `go install …/cmd/skim@latest`,
  nebo nic – `./skim` použije Go), tabulka příkazů s novými jmény.

## Testy
`cmd/skim` dispatcher: směrování na podpříkazy, neznámý příkaz → 2, `version`.
Přesunuté testy beze změny procházejí. Release workflow: ruční ověření
`go build` pro všechny cíle lokálně (`GOOS/GOARCH`), první release po souhlasu.

## Moje rozhodnutí
- Jméno programu `skim`, příkaz pro feedcheck `skim check`.
- Wrapper `./skim` je bash (Claude Code na Windows běží v bashi).
- První release (tag `v0.1.0`) až po tvém souhlasu – je to veřejná akce.
