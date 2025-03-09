#!/bin/python -u
# Matrix display for letters.

import os.path
exec(open(os.path.dirname(__file__)+"/lib.py").read())

# from https://github.com/GirishJoshi/3x3-LED-Matrix-Arduino/blob/master/3x3-LED-Matrix.ino
LETTERS_PIXELS = [
    [   # A
        0, 1, 0,
        1, 1, 1,
        1, 0, 1
    ],
    [   # B
        1, 1, 0,
        1, 1, 1,
        1, 1, 0
    ],
    [   # C
        1, 1, 1,
        1, 0, 0,
        1, 1, 1
    ],
    [   # D
        1, 1, 0,
        1, 0, 1,
        1, 1, 0
    ],
    [   # E
        1, 1, 1,
        1, 1, 0,
        1, 1, 1
    ],
    [   # F
        1, 1, 1,
        1, 1, 0,
        1, 0, 0
    ],
    [   # G
        0, 1, 1,
        1, 1, 1,
        0, 1, 1
    ],
    [   # H
        1, 0, 1,
        1, 1, 1,
        1, 0, 1
    ],
    [   # I
        1, 1, 1,
        0, 1, 0,
        1, 1, 1
    ],
    [   # J
        1, 1, 1,
        0, 1, 0,
        1, 1, 0
    ],
    [   # K
        1, 0, 1,
        1, 1, 0,
        1, 0, 1
    ],
    [   # L
        1, 0, 0,
        1, 0, 0,
        1, 1, 1
    ],
    [   # M
        1, 1, 1,
        1, 1, 1,
        1, 0, 1
    ],
    [   # N
        1, 0, 1,
        1, 1, 1,
        1, 0, 1
    ],
    [   # N
        0, 1, 0,
        1, 0, 1,
        0, 1, 0
    ],
    [   # P
        1, 1, 0,
        1, 1, 0,
        1, 0, 0
    ],
    [   # Q
        1, 1, 0,
        1, 1, 0,
        0, 0, 1
    ],
    [   # R
        1, 1, 0,
        1, 1, 0,
        1, 0, 1
    ],
    [   # S
        1, 1, 0,
        1, 1, 1,
        0, 1, 1
    ],
    [   # T
        1, 1, 1,
        0, 1, 0,
        0, 1, 0
    ],
    [   # U
        1, 0, 1,
        1, 0, 1,
        1, 1, 1
    ],
    [   # V
        1, 0, 1,
        1, 0, 1,
        0, 1, 0
    ],
    [   # W
        1, 0, 1,
        1, 1, 1,
        1, 0, 1
    ],
    [   # X
        1, 0, 1,
        0, 1, 0,
        1, 0, 1
    ],
    [   # Y
        1, 0, 1,
        0, 1, 0,
        0, 1, 0
    ],
    [   # Z
        1, 1, 1,
        0, 1, 0,
        1, 1, 1
    ],
]

letters_binary = []

for letter_pixels in LETTERS_PIXELS:
    i = 0
    binary = 0
    for pixel in letter_pixels:
        binary = binary | pixel << i
        i += 1
    letters_binary.append(binary)


def display(word):
    log(f"displaying: {word}")
    letter_idx = 0
    for c in word[:4]:
        letter_idx += 1
        letter_rank = ord(c.upper()) - ord("A")
        if letter_rank < 0 or letter_rank >= 26:
            log(f"cannot represent letter {c}")
            continue
        binary = letters_binary[letter_rank]
        emit(f"metric alphabet.{letter_idx} {binary} {binary}")
    while letter_idx < 4:
        letter_idx += 1
        emit(f"metric alphabet.{letter_idx} 0 0")


cur_word = 0


def next_word():
    words = [
        # "abcd", "efgh", "ijkl", "mnop", "qrst", "uvwx", "yz"
        "a b", "c d", "e f", "g h", "i j", "k l", "m n", "o p", "q r", "s t", "u v", "w x", "y z"
    ]
    global cur_word
    word = words[cur_word]
    display(word)
    emit(f"sound tts:{word}")
    cur_word = (cur_word + 1) % len(words)


# event(KEY, STATE), eg event("EXAM", true), event(1, false)
def event(switch, state):
    if switch == "CONT":
    	next_word()


def start(epoch_ms):
    next_word()


def stop(epoch_ms):
    pass


def tick(epoch_ms):
    pass


eventloop()
