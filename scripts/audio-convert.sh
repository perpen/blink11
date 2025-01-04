#!/bin/bash
set -ueo pipefail

RATE=48000

force=n
[ $# -gt 0 -a "$1" = "-f" ] && {
    force=y
    shift
}

[ $# = 2 ] || {
    echo "Usage: $0 PATH VOL"
    exit 2
}

path="$1"
vol="$2"
case "$path" in
*.wav|*.mp3) ;;
*) echo "unknown extension: $path"; exit 1 ;;
esac

raw="$(echo "$path" | sed -r 's/\.(wav|mp3)$/.raw/')"
[ $force = y ] && rm -f "$raw"

[ -f $raw -a $raw -nt $path ] &&  exit

echo "$path → $raw"
ffmpeg -i "$path" -acodec pcm_s16le -f s16le -ac 2 -ar $RATE -filter:a "volume=$vol" "$raw"
