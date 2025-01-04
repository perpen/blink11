#!/bin/bash
set -eu

PI_USER=pi
PI_UID=1000
PI_HOST=pdp11
# During dev use a dir on tmpfs, faster and better for sd card
PI_RUN_DIR=/run/user/$PI_UID/blink11
PI_INSTALL_DIR=/home/$PI_USER/blink11

pi-ssh() {
    ssh $PI_USER@$PI_HOST "$@"
}

pi-rsync-to() {
    local dir=$1
	rsync -avc --delete sounds bin scripts blink11.yaml $PI_USER@$PI_HOST:$dir/
}

build() {
	mkdir -p bin
	GOOS=linux GOARCH=arm GOARM=7 go build -o bin/blink11
	GOOS=linux GOARCH=arm GOARM=7 go build -o bin/agent11-arm ./agent11/
	# disabling cgo to get static linked binaries
	CGO_ENABLED=0 go build -o bin/agent11-amd64 ./agent11/
}

run() {
    sounds
    build
	m4 <blink11.yaml.m4 >blink11.yaml
	pi-rsync-to $PI_RUN_DIR/
	rm blink11.yaml
	# Use cat to detect disconnection from the remote side
	cat | pi-ssh $PI_RUN_DIR/scripts/blinker.sh
}

install() {
    sounds
    build
	m4 <blink11.yaml.m4 >blink11.yaml
	pi-rsync-to $PI_INSTALL_DIR/
	rm blink11.yaml
	pi-ssh pkill -9 -f '/(blink11$|agent11-arm)' || true
	pi-service $PI_INSTALL_DIR blink11 blink11
	pi-service $PI_INSTALL_DIR agent11 agent11-arm \
		-hostname $PI_HOST -server localhost:3333 \
		-net -cpu \
		-script \'./scripts/homeassistant.sh agent\'
}

pi-service() {
    local dir=$1
    local name=$2
    local bin=$3
    shift 3
    local args="$*"
    cat <<EOF | pi-svc $name
[Unit]
Description=$name
[Service]
RestartSec=1
User=pi
ExecStart=$dir/bin/$bin $args
WorkingDirectory=$dir
Environment="XDG_RUNTIME_DIR=/run/user/$PI_UID"
[Install]
WantedBy=default.target
EOF
}

# Usage: services on|off
pi-svc() {
    local name=$1
    pi-ssh sudo tee /etc/systemd/system/$name.service
	set -x
	pi-ssh sudo systemctl daemon-reload
	pi-ssh sudo systemctl enable $name
	pi-ssh sudo systemctl restart $name
	pi-ssh sudo systemctl status $name
	set +x
}

# Usage: sounds [-f]
# Use -f to re-convert all files, eg when changing audio rates/etc
sounds() {
    local file
    for file in $(ls sounds/* | grep -E '\.(mp3|wav)$'); do
        case $file in
        sounds/main-mission-to-eagle-intercomm.wav) vol=0.1 ;;
        sounds/sci-fi-interface-robot-click-901.wav) vol=0.2 ;;
        *) vol=1 ;;
        esac
        scripts/audio-convert.sh "$@" $file $vol
    done
}

edit() {
	kak2 hack.sh scripts/* README.md blink11.yaml.m4 {,agent11/,audio/,pidp11/}*.go
}

cd "$(dirname $0)"
"$@"
