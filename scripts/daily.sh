#!/bin/bash
# Denní běh skim bez obsluhy: stáhne, ohodnotí, sestaví digest a publikuje
# stránku. Spouští ho launchd (viz scripts/install-daily.sh), jde ale pustit
# i ručně: ./scripts/daily.sh
#
# Výsledek: data/logs/daily-YYYY-MM-DD.json (výstup Claude Code) a notifikace
# v macOS. Kompatibilní s /bin/bash 3.2 (výchozí bash na macOS).

set -u

cd "$(dirname "$0")/.." || exit 1
ROOT=$(pwd)

# launchd dává minimální PATH – přidej obvyklá umístění claude a go.
export PATH="$HOME/.local/bin:$HOME/.claude/local:/opt/homebrew/bin:/usr/local/bin:/usr/local/go/bin:$HOME/go/bin:$PATH"

CLAUDE_BIN=${CLAUDE_BIN:-$(command -v claude || true)}
if [ -z "$CLAUDE_BIN" ]; then
	echo "skim daily: claude nenalezen v PATH (nastav CLAUDE_BIN)" >&2
	osascript -e 'display notification "claude nenalezen – nastav CLAUDE_BIN" with title "Skim: denní běh selhal"' 2>/dev/null
	exit 1
fi

DAY=$(date +%F)
LOGDIR="$ROOT/data/logs"
LOG="$LOGDIR/daily-$DAY.json"
ERR="$LOGDIR/daily-$DAY.err"
LOCK="$LOGDIR/.daily.lock"
mkdir -p "$LOGDIR"

# Jeden běh najednou (launchd může po probuzení spustit zmeškaný běh).
if ! mkdir "$LOCK" 2>/dev/null; then
	# Zámek starší než 2 hodiny je pozůstatek spadlého běhu.
	if [ -n "$(find "$LOCK" -maxdepth 0 -mmin +120 2>/dev/null)" ]; then
		rmdir "$LOCK" && mkdir "$LOCK" || exit 1
	else
		echo "skim daily: už běží" >&2
		exit 0
	fi
fi
trap 'rmdir "$LOCK" 2>/dev/null' EXIT

PROMPT="Spusť skill skim (denní běh) v režimu bez obsluhy pro den $DAY. \
Nikdo u toho není: na nic se neptej, nic neměň v feeds.yaml ani topics.md, \
postupuj podle sekce „Běh bez obsluhy“ ve skillu skim."

# Povolené nástroje: jen to, co denní běh potřebuje. Bash smí ./skim a pár
# čtecích příkazů; ToolSearch načítá odložené nástroje (Artifact, ArtifactData).
ALLOWED="Read,Glob,Grep,Write,Edit,Skill,ToolSearch,Artifact,ArtifactData"
for cmd in ./skim date echo cat head tail grep wc; do
	ALLOWED="$ALLOWED,Bash($cmd:*),Bash($cmd *)"
done

"$CLAUDE_BIN" -p "$PROMPT" \
	--permission-mode dontAsk \
	--allowedTools "$ALLOWED" \
	--max-turns 300 \
	--output-format json \
	>"$LOG" 2>"$ERR"
status=$?

# Data pro stránku připraví kód, ne model: data/web/digests.json.
# Na stránku je nahraje naplánovaná úloha v claude.ai (Claude Code bez
# obsluhy nemá nástroje pro artifacty).
./skim digest export >>"$ERR" 2>&1 || status=1

# Shrnutí do notifikace: první řádek výsledku (řádek statistiky ze skillu).
summary=$(grep -o '"result":"[^"]*' "$LOG" 2>/dev/null | head -1 | sed 's/^"result":"//; s/\\n.*//' | cut -c1-180)
denied=$(grep -o '"permission_denials":\[[^]]' "$LOG" 2>/dev/null)

if [ $status -ne 0 ] || grep -q '"is_error":true' "$LOG" 2>/dev/null || [ "${summary#CHYBA:}" != "$summary" ]; then
	title="Skim: denní běh selhal"
	msg=${summary:-"Detaily v data/logs/daily-$DAY.*"}
	[ $status -eq 0 ] && status=1
elif [ -n "$denied" ]; then
	title="Skim: hotovo, ale něco bylo zamítnuto"
	msg="Viz permission_denials v data/logs/daily-$DAY.json"
else
	title="Skim: digest je připravený"
	msg=${summary:-"Stránka je aktualizovaná."}
fi

# Uvozovky by rozbily AppleScript.
msg=$(printf '%s' "$msg" | tr '"' "'")
osascript -e "display notification \"$msg\" with title \"$title\"" 2>/dev/null
echo "$title – $msg"
exit $status
