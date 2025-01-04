#!/bin/bash
set -ue

{
    echo "initialising"
    export HASS_SERVER=http://ha:8123
    export HASS_TOKEN="$(<~/.secrets/ha.token)"

    source ~/.local/node/nvm.sh || source ~/.nvm/nvm.sh
    nvm use node >/dev/null

    cd "$(dirname $0)"
    npm install --no-fund --no-audit
    echo "starting"
} 1>&2 # stdout is reserved for messages to blink11

node index.mjs
