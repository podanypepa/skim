# skim (denní běh) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Skill `skim`, který jedním spuštěním stáhne, ohodnotí a ukáže digest ze všech témat.

**Architecture:** Jen markdown skill skládající `skimfetch`, `skim-triage` a `skim-digest`; žádný nový Go kód.

**Tech Stack:** Claude skill (markdown).

**Spec:** `docs/superpowers/specs/2026-11-16-skim-daily-design.md`

## Global Constraints

- Žádný nový Go kód; `go test ./...` musí dál projít.
- Pravidla triage a digestu se nekopírují – skill odkazuje na `skim-triage` a `skim-digest`.
- Texty česky.

## Review Focus

1. **Digest dne existuje a nic nového ≥ 4 nepřibylo** → ukázat existující, nepřepisovat. Ověří ostrý běh (Task 2), druhé spuštění.
2. **Všechny feedy selžou / nic nového** → triage se přeskočí, ukáže se existující digest nebo zpráva „nic nového“. Ověří čtení skillu.
3. **Fetch skončí nenulovým exitem** (rozbitý `feeds.yaml`) → skill skončí s chybou, netriáží. Ověří čtení skillu.
4. **Velká dávka triage (151 článků)** → smyčka po dávkách do `pending` 0. Ověří ostrý běh.
5. **Nové ≥ 4 přibyly** → přepis s `-force` a zmínka v chatu. Ověří ostrý běh.

---

### Task 1: Skill a dokumentace

**Files:**
- Create: `skills/skim/SKILL.md`
- Modify: `CLAUDE.md` (Skilly: `skim` na první místo jako hlavní vstup; Plánované: odstranit `skim-search`)

- [ ] **Step 1:** Napiš `SKILL.md` (frontmatter `name: skim`; `description`: „Denní přehled jedním příkazem: stáhne nové články, ohodnotí je a ukáže digest ze všech témat. Použij, když chci denní přehled, ‚co je nového‘ nebo prostě ‚skim‘.“) s kroky 1–5 ze specu, včetně pravidla porovnání ID z `data/digest/<dnes>.json` s kandidáty ≥ 4 (`go run ./cmd/skimdigest candidates -min 4`).
- [ ] **Step 2:** Uprav `CLAUDE.md`.
- [ ] **Step 3:** Ověř: `go test ./...` → ok; `.claude/skills/skim/SKILL.md` existuje.

### Task 2: Ostrý běh

- [ ] **Step 1:** Spusť skill `skim` (dnes je digest jen s AI a čeká 151 nových článků) → triage do `pending` 0, digest přepsán s `-force` (přibyly ≥ 4 z Go/Pythonu), v chatu statistika + digest po tématech.
- [ ] **Step 2:** Ověř Review Focus 1: znovu `candidates -min 4` vs. `data/digest/<dnes>.json` → žádné nové ID → druhý běh by jen ukázal existující.
