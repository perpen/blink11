#!/bin/bash
set -ue

source $(dirname $0)/lib.sh

METRIC_SECONDS=shell-stopwatch
METRIC_TENTHS=shell-stopwatch-tenths

# disable logging by overriding the default definition of `log`
log() {
    true
}

start() {
	tenths=0
	seconds=0
}

stop() {
    true
}

# tick EPOCH_MS
tick() {
    local epoch=$1
    log "tick: epoch=$epoch"
    tenths=$((tenths+1))
    [[ $tenths == 10 ]] && {
        tenths=0
        seconds=$((seconds+1))
    }
	log "seconds=$seconds tenths=$tenths"
    emit "metric $METRIC_TENTHS $((tenths+1)) 11"
    emit "metric $METRIC_SECONDS $seconds $seconds"
}

# event KEY STATE, eg "event EXAM true", "event 1 false"
event() {
    local switch=$1
    local on=$2
    log "event: switch=$switch on=$on"
}

eventloop
