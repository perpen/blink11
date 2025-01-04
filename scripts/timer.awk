#!/bin/mawk -f
# Script must be run with mawk -W interactive -f

function dbg(msg) {
    print msg >"/dev/stderr"
}

/^start/ {
    dbg("start")
    initial = mem[0]
    if (initial == 0) {
        print "sound tts: stopping after " 0 " seconds"
        print "control stop"
    }
    counter = initial
    print "metric awk-timer-progress " counter " " initial
    print "metric awk-timer " counter " " counter
    print "sound tts: counting down " initial " seconds"
}

/^stop/ {
    dbg("stop")
    print "sound tts: stopping after " initial-counter " seconds"
}

# tick EPOCH_MS
/^tick/ {
    epoch = $2
    dbg("tick epoch=" epoch)
    if (counter > 0) {
        counter--
    }
    print "metric awk-timer-progress " counter+0 " " initial
    print "metric awk-timer " counter " " counter
    if (counter == 0) {
        print "sound tts: stopping after " initial " seconds"
        print "control stop"
    }
}

# event KEY STATE, eg "event EXAM true", "event 1 false"
/^event/ {
    switch = $2
    on = $3 == "true"
    dbg("event switch=" switch " on=" on)
}

# memory ADDR DATA, eg "memory 0 3"
/^memory/ {
    addr = $2
    data = $3
    dbg("memory addr=" addr " data=" data)
    mem[addr] = data
}
