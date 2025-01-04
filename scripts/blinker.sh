#!/bin/bash
# to be run from rpi
set -eu

RUN_DIR=/run/user/1000/blink11

cd $RUN_DIR

# If it's currently running as a service
sudo systemctl stop blink11 agent11

./bin/blink11 2>&1 | tee /tmp/oo &

./bin/agent11-arm -hostname pdp11 -server localhost:3333 -net -cpu \
    -script './scripts/homeassistant.sh agent' &

# We use cat to detect ssh disconnection so we can kill the processes
cat
pkill -f '/(blink11$|agent11-arm)' || true
