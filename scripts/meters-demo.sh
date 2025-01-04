#!/bin/bash

source $(dirname $0)/lib.sh

start() {
    ticks=0
    tick
}

stop() {
    true
}

log() {
    true
}

# tick EPOCH_MS
tick() {
    local metric
    for metric in dot bar lumen flash strobe binary; do
        local flag
        eval "flag=\$$metric"
        local metric_ticks=$ticks
        [[ "$flag" == true ]] || metric_ticks=0
        local max=10
        emit "metric demo.$metric $metric_ticks $max"
    done
    ticks=$((ticks+1))
    [[ $ticks == 11 ]] && ticks=0
}

# event KEY STATE, eg "event EXAM true", "event 1 false"
event() {
    local switch=$1
    local on=$2
    log "event: switch=$switch on=$on"
    case $switch in
    0) thing=binary ;;
    1) thing=lumen ;;
    2) thing=flash ;;
    3) thing=strobe ;;
    4) thing=bar ;;
    5) thing=dot ;;
    esac
    eval "$thing=$on"
}

eventloop
