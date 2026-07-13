#!/usr/bin/env python3
"""Syntax-check the inline <script> blocks of every wiki HTML page.

The wiki pages build whole sections as JS template literals, so a stray
backtick in an embedded shell snippet silently terminates the literal and
makes the entire script fail to parse. The browser then renders nothing and
the page hangs on its static "Loading…" placeholder — exactly the TASK-125
outage. A parse error is invisible to review, so gate it in CI.
"""

import pathlib
import re
import subprocess
import sys
import tempfile

WIKI = pathlib.Path(__file__).parent / "wiki"
INLINE_SCRIPT = re.compile(r"<script(?![^>]*\bsrc=)[^>]*>(.*?)</script>", re.S)

failed = False

for page in sorted(WIKI.glob("*.html")):
    src = page.read_text(encoding="utf-8")
    for i, block in enumerate(INLINE_SCRIPT.findall(src)):
        # Report errors against the real line in the HTML file, not the block.
        offset = src[: src.index(block)].count("\n")
        with tempfile.NamedTemporaryFile("w", suffix=".js", encoding="utf-8") as tmp:
            tmp.write(block)
            tmp.flush()
            proc = subprocess.run(
                ["node", "--check", tmp.name], capture_output=True, text=True
            )
        if proc.returncode == 0:
            print(f"ok    {page.name} (inline script {i})")
            continue

        failed = True
        print(f"FAIL  {page.name} (inline script {i}) — starts at line {offset + 1}")
        for line in proc.stderr.splitlines():
            hit = re.search(rf"{re.escape(tmp.name)}:(\d+)", line)
            if hit:
                line = line.replace(
                    f"{tmp.name}:{hit.group(1)}",
                    f"{page.name}:{offset + int(hit.group(1))}",
                )
            print(f"      {line}")

sys.exit(1 if failed else 0)
