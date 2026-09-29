"""Drive the real fest binary under a pty and snapshot the rendered screen.

The bytes go through pyte, a VT100 emulator, so the screen read back is what a
terminal draws. Nothing strips ANSI escapes: a comparison over flattened frames
would pass even if the colour changed, which is the one thing this evidence is
for.
"""
import fcntl
import json
import os
import pty
import select
import struct
import sys
import termios
import time
from importlib.metadata import version

import pyte

binary, workdir, home, state, out_dir = sys.argv[1:6]

COLS, ROWS = 120, 40
PIXEL_WIDTH, PIXEL_HEIGHT = 1200, 800

screen = pyte.Screen(COLS, ROWS)
stream = pyte.Stream(screen)
transcript = bytearray()

env = dict(
    os.environ,
    HOME=home,
    TERM="xterm-256color",
    COLORTERM="truecolor",
    LINES=str(ROWS),
    COLUMNS=str(COLS),
    FEST_CONFIG_DIR=os.path.join(home, "config"),
)
env.pop("NO_COLOR", None)
env.pop("CI", None)

pid, fd = pty.fork()
if pid == 0:
    os.chdir(workdir)
    os.execve(binary, [binary, "watch"], env)

fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", ROWS, COLS, PIXEL_HEIGHT, PIXEL_WIDTH))
os.set_blocking(fd, False)


def pump(seconds):
    end = time.time() + seconds
    while time.time() < end:
        r, _, _ = select.select([fd], [], [], 0.1)
        if not r:
            continue
        try:
            chunk = os.read(fd, 65536)
        except OSError:
            return
        if not chunk:
            return
        transcript.extend(chunk)
        stream.feed(chunk.decode("utf-8", "replace"))


pump(3.0)

display = list(screen.display)
cursor = {"x": screen.cursor.x, "y": screen.cursor.y}

# Per-cell attributes for every row that drew something, trimmed to the last
# column it used. screen.display flattens styling, so a comparison over it
# alone would pass even if the icon changed colour. Blank rows and trailing
# blank cells are left out so the snapshot stays small enough to review.
attributes = {}
for row_index, line in enumerate(display):
    used = len(line.rstrip())
    if used == 0:
        continue
    buffer_row = screen.buffer[row_index]
    attributes[str(row_index)] = [
        [
            buffer_row[col].data,
            buffer_row[col].fg,
            buffer_row[col].bg,
            buffer_row[col].bold,
            buffer_row[col].italics,
            buffer_row[col].underscore,
            buffer_row[col].reverse,
            buffer_row[col].strikethrough,
            buffer_row[col].blink,
        ]
        for col in range(used)
    ]

os.write(fd, b"\x03")
pump(0.5)
try:
    os.close(fd)
except OSError:
    pass
try:
    os.waitpid(pid, 0)
except ChildProcessError:
    pass

os.makedirs(out_dir, exist_ok=True)
with open(os.path.join(out_dir, f"screen-{state}.json"), "w", encoding="utf-8") as handle:
    json.dump(
        {
            "renderer": "pyte",
            "pyte_version": version("pyte"),
            "terminal": {
                "columns": COLS,
                "rows": ROWS,
                "pixel_width": PIXEL_WIDTH,
                "pixel_height": PIXEL_HEIGHT,
                "mode": "xterm-256color",
            },
            "snapshot": {
                "name": state,
                "display": display,
                "cursor": cursor,
                "attributes": attributes,
            },
        },
        handle,
        indent=2,
    )
with open(os.path.join(out_dir, f"transcript-{state}.txt"), "wb") as handle:
    handle.write(bytes(transcript))

for line in display:
    if line.strip():
        print(line.rstrip())
