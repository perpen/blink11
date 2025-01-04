#!/bin/python -u
# Counts down for the duration indicated in address 0 in
# the following format, with 6 bits for each of hours, minutes,
# seconds - we call this "trinity" in the code:
# - number of seconds in bits 0-5
# - number of minutes in bits 6-11
# - number of hours in bits 12-15
# The time remaining is emitted in the same format.
# If address 1 content is >0, plays a tick sound every second.

import sys

METRIC = "python-timer-counter"
METRIC_PROGRESS = "python-timer-progress"
TICK_SOUND = "folder.raw"
FINAL_SOUND = "main-mission-to-eagle-intercomm.raw"

mem = {}
timer_total_sec = 0
elapsed_sec = 0
audiotick = False


def trinity_to_seconds(trinity):
    s = trinity & 0o77
    m = trinity >> 6 & 0o77
    h = trinity >> 12 & 0o77
    return h*60*60 + m*60 + s


def seconds_to_trinity(sec):
    h, m, s = seconds_to_hms(sec)
    return h << 12 | m << 6 | s


def seconds_to_hms(sec):
    h = int(sec / 60 / 60)
    sec -= h * 60 * 60
    m = int(sec / 60)
    sec -= m * 60
    s = int(sec)
    return h, m, s


def speak_time(pre, sec, post):
    txt = ""
    h, m, s = seconds_to_hms(sec)
    if h > 0:
        txt += f"{h} hours "
    if m > 0:
        txt += f"{m} minutes "
    if s > 0:
        txt += f"{s} seconds "
    if txt == "":
        txt = "no time at all"
    emit(f"sound tts: {pre} {txt} {post}")


# show in blink11 log
def log(msg):
    print(msg, file=sys.stderr)


# sends message to blink11
def emit(msg):
    log(msg)
    print(msg)


def read(addr):
    return mem.get(addr, 0)


def start():
    log("start")
    global count, timer_total_sec, elapsed_sec, audiotick
    timer_total_sec = trinity_to_seconds(read(0))
    audiotick = read(1) != 0
    speak_time("starting timer for", timer_total_sec, "")
    elapsed_sec = 0
    tick()


def stop():
    log("stop")
    global elapsed_sec
    speak_time("stopping after", elapsed_sec, "")


def tick():
    log("tick")
    global elapsed_sec, timer_total_sec
    remaining_sec = timer_total_sec - elapsed_sec
    trinity = seconds_to_trinity(remaining_sec)
    if audiotick:
        emit(f"sound {TICK_SOUND}")
    emit(f"metric {METRIC} {trinity} {trinity}")
    emit(f"metric {METRIC_PROGRESS} {remaining_sec} {timer_total_sec}")
    if remaining_sec == 0:
        emit(f"sound {FINAL_SOUND}")
        speak_time("", timer_total_sec, "elapsed")
        # Tell blink that we stopped so it stops sending ticks
        emit("control stop")
    elapsed_sec += 1


def event(switch, state):
    log(f"got event: {switch}/{state}")


for line in sys.stdin:
    tokens = line.strip().split(" ")
    first = tokens[0]
    if first == "start":
        start()
    elif first == "stop":
        stop()
    elif first == "memory":
        addr = int(tokens[1])
        data = int(tokens[2])
        log(f"mem[{addr}]={data}")
        mem[addr] = data
    elif first == "tick":
        tick()
    elif first == "event":
        event(tokens[1], tokens[2] == "true")
