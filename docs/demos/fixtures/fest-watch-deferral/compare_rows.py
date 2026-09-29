"""Compare the fest watch row for the deferred task, before and after.

The comparison is over pyte's rendered cells, with the character, its
foreground, its background and every attribute bit. Nothing is flattened by
stripping ANSI escapes: a text-only comparison would pass even if the icon
turned a different colour, which is the exact promise this evidence is for.
"""
import json
import sys

evidence_dir, task_name = sys.argv[1], sys.argv[2]

ATTRS = ["char", "fg", "bg", "bold", "italics", "underscore", "reverse",
         "strikethrough", "blink"]


def load(state):
    with open(f"{evidence_dir}/screen-{state}.json", encoding="utf-8") as handle:
        return json.load(handle)


def row_index(snapshot, needle):
    for index, line in enumerate(snapshot["display"]):
        if needle in line:
            return index
    raise SystemExit(f"row for {needle} not found in the {snapshot['name']} screen")


before, after = load("blocked"), load("deferred")
b_snap, a_snap = before["snapshot"], after["snapshot"]

b_row, a_row = row_index(b_snap, task_name), row_index(a_snap, task_name)
print(f"row index: blocked={b_row} deferred={a_row}")
if b_row != a_row:
    print("FAIL: the row moved")
    raise SystemExit(1)

b_cells, a_cells = b_snap["attributes"][str(b_row)], a_snap["attributes"][str(a_row)]
print(f"rendered row (blocked):  {b_snap['display'][b_row].rstrip()!r}")
print(f"rendered row (deferred): {a_snap['display'][a_row].rstrip()!r}")

differences = []
for column, (b_cell, a_cell) in enumerate(zip(b_cells, a_cells)):
    if b_cell != a_cell:
        differences.append(
            (column, dict(zip(ATTRS, b_cell)), dict(zip(ATTRS, a_cell)))
        )

print(f"columns compared: {len(b_cells)} (the full drawn width of the row)")
print(f"columns masked: none. The row carries no timestamp or elapsed-time "
      f"field, so nothing was masked.")
if len(b_cells) != len(a_cells):
    print(f"FAIL: the row changed width, {len(b_cells)} -> {len(a_cells)}")
    raise SystemExit(1)

if not differences:
    print("cell diff: empty. Character, foreground, background and every "
          "attribute bit are identical in both recordings.")
    print()
    print("whole-screen comparison, every drawn row:")
    rows = sorted(set(b_snap["attributes"]) | set(a_snap["attributes"]), key=int)
    changed = [
        row for row in rows
        if b_snap["attributes"].get(row) != a_snap["attributes"].get(row)
    ]
    if not changed:
        print("  no row differs. fest watch draws the two states identically.")
    else:
        for row in changed:
            print(f"  row {row}:")
            print(f"    blocked:  {b_snap['display'][int(row)].rstrip()!r}")
            print(f"    deferred: {a_snap['display'][int(row)].rstrip()!r}")
    raise SystemExit(0)

print(f"cell diff: {len(differences)} differing columns")
for column, b_cell, a_cell in differences:
    print(f"  column {column}: {b_cell} -> {a_cell}")
raise SystemExit(1)
