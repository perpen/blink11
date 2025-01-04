#!/bin/bash
# xxx error handling, eg ha api down

set -ueo pipefail

HA=http://192.168.1.32:8123
TOKEN=$(<~/.secrets/ha.token)

c() {
    curl -fs \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        "$HA/api$1" | jq .
}

get() {
    local metric="$1"
    local max="$2"
    local endpoint="$3"
    local jq="$4"
    shift 4
    c "$endpoint" | jq -r "$jq" | map "$@" | wrap $metric $max
}

# usage: echo STRING | map STRING1 INT1 ...
map() {
    local request
    read request
    [ $# -eq 0 ] && {
        echo $request
        return
    }
    local cond val
    while [ $# -ne 0 ]; do
        cond="$1"; shift
        val="$1"; shift
	[ "$cond" = "$request" ] && {
    		echo "$val"
    		return
	}
    done
    echo error
}

wrap() {
    local name="$1"
    local max="$2"
    local val
    read val
    echo "$name $(floor $val) $max"
}

floor() {
    echo $1 | sed 's/\..*//'
}

states() {
    c /states
}

agent() {
    while true; do
        # echo "sound: tts:hello from home assistant"
        # get METRIC MAX ENDPOINT JQ [STRING INT]...
        get weather.temp 40 /states/weather.forecast_home .attributes.temperature
        get dyson.temp 40 /states/sensor.dyson_temperature .state
        get dyson.state 2 /states/climate.dyson .state off 0 on 1 heat 2
        get car.status 1 /states/sensor.e208_charging_status .state Disconnected 0 Charging 1
        get car.level 100 /states/sensor.e208_battery_level .state
        sleep 60
    done
}

"$@"
