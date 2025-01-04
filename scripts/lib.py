import sys

log_enabled = True
mem = {}


# show in blink11 log
def log(msg):
    if log_enabled:
        print(msg, file=sys.stderr)


# sends message to blink11
def emit(msg):
    log(msg)
    print(msg)


def read(addr):
    return mem.get(addr, 0)


def start(epoch_ms):
    log(f"start {epoch_ms} - not implemented")


def stop(epoch_ms):
    log(f"stop {epoch_ms} - not implemented")


def tick(epoch_ms):
    log(f"tick {epoch_ms} - not implemented")


# event(KEY, STATE), eg event("EXAM", true), event(1, false)
def event(switch, state):
    log(f"event {switch} {state} - not implemented")


def eventloop():
    for line in sys.stdin:
        tokens = line.strip().split(" ")
        first = tokens[0]
        if first == "start":
            start(tokens[1])
        elif first == "stop":
            stop(tokens[1])
        elif first == "memory":
            addr = int(tokens[1])
            data = int(tokens[2])
            log(f"mem[{addr}]={data}")
            mem[addr] = data
        elif first == "tick":
            epoch_ms = int(tokens[1])
            tick(epoch_ms)
        elif first == "event":
            key = tokens[1]
            if len(tokens[1]) == 1:
                key = int(tokens[1])
            on = tokens[2] == "true"
            event(key, on)
        else:
            log(f"invalid message: {line}")
