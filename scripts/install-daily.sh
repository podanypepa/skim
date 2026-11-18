#!/bin/bash
# Nainstaluje (nebo odinstaluje) ranní spouštění skim přes launchd.
#
#   ./scripts/install-daily.sh            # každý den v 6:40
#   ./scripts/install-daily.sh 7 15       # každý den v 7:15
#   ./scripts/install-daily.sh uninstall
#
# Když Mac v daný čas spí, launchd zmeškaný běh spustí hned po probuzení.

set -eu

LABEL="cz.podany.skim.daily"
PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"
DOMAIN="gui/$(id -u)"
ROOT=$(cd "$(dirname "$0")/.." && pwd)

if [ "${1:-}" = "uninstall" ]; then
	launchctl bootout "$DOMAIN/$LABEL" 2>/dev/null || true
	rm -f "$PLIST"
	echo "Odinstalováno."
	exit 0
fi

HOUR=${1:-6}
MINUTE=${2:-40}
mkdir -p "$HOME/Library/LaunchAgents" "$ROOT/data/logs"
chmod +x "$ROOT/scripts/daily.sh"

cat >"$PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>$LABEL</string>
	<key>ProgramArguments</key>
	<array>
		<string>/bin/bash</string>
		<string>-lc</string>
		<string>"$ROOT/scripts/daily.sh"</string>
	</array>
	<key>WorkingDirectory</key>
	<string>$ROOT</string>
	<key>StartCalendarInterval</key>
	<dict>
		<key>Hour</key>
		<integer>$HOUR</integer>
		<key>Minute</key>
		<integer>$MINUTE</integer>
	</dict>
	<key>StandardOutPath</key>
	<string>$ROOT/data/logs/launchd.out</string>
	<key>StandardErrorPath</key>
	<string>$ROOT/data/logs/launchd.err</string>
</dict>
</plist>
EOF

plutil -lint "$PLIST" >/dev/null
launchctl bootout "$DOMAIN/$LABEL" 2>/dev/null || true
launchctl bootstrap "$DOMAIN" "$PLIST"
printf 'Nainstalováno: každý den v %d:%02d.\n' "$HOUR" "$MINUTE"
echo "Spustit hned na zkoušku: launchctl kickstart $DOMAIN/$LABEL"
