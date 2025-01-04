#!/bin/awk -f
# Script must be run with mawk -W interactive -f

function dbg(msg) {
    # print msg >"/dev/stderr"
}

/^start/ {
    dbg("start")
}

/^stop/ {
    dbg("stop")
}

# tick EPOCH_MS
/^tick/ {
    epoch = $2
    dbg("tick epoch=" epoch)
}

# event KEY STATE, eg "event EXAM true", "event 1 false"
/^event/ {
    switch = $2
    on = $3 == "true"
    dbg("event key=" switch " on=" on)
    if (switch > 10) {
        next
    }
    val = on*switch
    print "metric effects.lumen." switch " " val " 10"
    print "metric effects.flash." switch " " val " 10"
    print "metric effects.strobe." switch " " val " 10"
    if (on) {
        print "sound tts:" val*10 "%"
    }
}

# memory ADDR DATA, eg "memory 0 3"
/^memory/ {
    addr = $2
    data = $3
    dbg("memory addr=" addr " data=" data)
    mem[addr] = data
}
