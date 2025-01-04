#!/bin/bash
set -eu

_pi-ssh() {
    ssh $PI_USER@$PI_HOST "$@"
}

PI_USER=pi
PI_HOST=pdp11

# When installing as a service:
PI_INSTALL_DIR=/home/$PI_USER/blink11

# Sounds, memory file, caches
PI_PERSISTENT_DIR=/var/tmp/blink11

# During dev use a dir on tmpfs, faster and better for sd card
PI_UID=$(_pi-ssh id -u $PI_USER)
PI_DEV_DIR=/run/user/$PI_UID/blink11

_pi-rsync-to() {
    local run_dir=$1
    echo copy
    _pi-ssh mkdir -p "$PI_PERSISTENT_DIR/sounds" "$run_dir"
    # Only push the .raw files
    rsync -avcq --delete sounds/ \
        --include='*/' --include='*.raw' --exclude='*' \
        "$PI_USER@$PI_HOST:$PI_PERSISTENT_DIR/sounds/"
    rsync -avcq --delete bin scripts blink11.yaml "$PI_USER@$PI_HOST:$run_dir/"
}

_pi-service() {
    local dir=$1
    local name=$2
    local delay=$3
    local bin=$4
    shift 4
    local args="$*"
    local tmp=/var/tmp/blink11-hack.tmp

    echo "service $name"

    cat <<EOF >$tmp
[Unit]
Description=$name
After=wireplumber.service
[Service]
Restart=on-success
RestartSec=1
# Wait for pipewire to be functional on boot, After wireplumber not enough xxx
ExecStartPre=/bin/sleep $delay
ExecStart=$dir/bin/$bin $args
WorkingDirectory=$dir
[Install]
WantedBy=default.target
EOF

    cat "$tmp" | _pi-ssh tee "/home/$PI_USER/.config/systemd/user/$name.service" >/dev/null
    rm "$tmp"
    _pi-ssh systemctl --quiet --user daemon-reload
    _pi-ssh systemctl --quiet --user enable "$name"
    _pi-ssh systemctl --quiet --user restart "$name"
    # _pi-ssh systemctl --quiet --user status "$name"
}

_common() {
    local run_dir=$1
    m4 \
        -D "PI_PERSISTENT_DIR=$PI_PERSISTENT_DIR" \
        -D "PI_RUN_DIR=$run_dir" \
        <blink11.yaml.m4 >blink11.yaml
    sounds
    build
    _pi-rsync-to "$run_dir/"
    rm blink11.yaml
}

build() {
    local arm_flags_32="GOOS=linux GOARCH=arm GOARM=7"
    local arm_flags_64="GOOS=linux GOARCH=arm64"
    local arm_flags=$arm_flags_64
    echo build $arm_flags

	cd src
    mkdir -p ../bin
    eval $arm_flags go build -o ../bin/blink11
    eval $arm_flags go build -o ../bin/agent11-arm ./agent11/

    # Static linking for portability of the agents
    CGO_ENABLED=0 go build -o ../bin/agent11-$(go env GOARCH) ./agent11/
    cd - >/dev/null
}

# Run from tmpfs
run() {
    _common "$PI_DEV_DIR"
    _pi-ssh "$PI_DEV_DIR/scripts/runner" run "$PI_DEV_DIR"
}

agent() {
    build
    pkill agent11-amd64  || true
    ./bin/agent11-amd64 -hostname $(hostname -s) -server $PI_HOST:3333 "$@"
}

# Install services on rpi
services() {
    _common "$PI_INSTALL_DIR/"
    _pi-ssh mkdir -p /home/$PI_USER/.config/systemd/user
    _pi-ssh "$PI_INSTALL_DIR/scripts/runner" kill
    _pi-service "$PI_INSTALL_DIR" blink11 5 blink11 blink11.yaml
    _pi-service "$PI_INSTALL_DIR" agent11 0 agent11-arm \
        -hostname $PI_HOST -server localhost:3333 \
        -net -cpu -disk
}

# Usage: sounds [-f]
# Convert all formats to raw, adjust volume.
# Use -f to re-convert all files, eg when changing volumes, audio rates/etc
sounds() {
    echo "sounds"
    local speed=1.0
    local file
    cd sounds
    find . -regextype egrep -regex '.*\.(wav|mp3)' \
    | cut -b3- \
    | while read file; do
        local vol=1
        case $file in
        main-mission-to-eagle-intercomm.wav) vol=0.1 ;;
        sci-fi-interface-robot-click-901.wav) vol=0.2 ;;
        interface-hint-notification-911.wav) vol=0.2 ;;
        sensor_sweep.wav) vol=0.3 ;;
        folder.wav) vol=0.8 ;;
        stdin.wav) vol=0.5 ;;
        esac
        ../scripts/audio-convert "$@" $vol $speed "$file"
    done
    cd - >/dev/null
}

edit() {
    kak2 blink11.yaml.m4 src/*/*.go src/*.go
}

cd "$(dirname "$0")"
"$@"
