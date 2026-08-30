# -*- coding: utf-8 -*-
"""바로위키 링크 크롤 검사

/baro 에서 시작해 화면에 실제로 찍혀 나오는 링크를 따라가며
404 나 500 이 나는 곳을 찾는다. 사용자가 실제로 눌러 볼 수 있는 길만 본다.

사용법 : python crawl.py <포트> [최대건수]
"""
import io
import re
import sys
import html
import json
import collections
import urllib.parse
import urllib.request
import urllib.error

PORT = sys.argv[1] if len(sys.argv) > 1 else "39330"
LIMIT = int(sys.argv[2]) if len(sys.argv) > 2 else 1200
BASE = "http://127.0.0.1:" + PORT

href_re = re.compile(r'href="([^"]+)"')

seen = set()
queue = collections.deque(["/baro"])
found_by = {"/baro": "(시작점)"}
bad = []
ok = 0
skipped = 0


def fetch(path):
    url = BASE + urllib.parse.quote(path, safe="/?=&%#+")
    req = urllib.request.Request(url, headers={"User-Agent": "baro-crawl"})
    try:
        with urllib.request.urlopen(req, timeout=25) as r:
            ctype = r.headers.get("Content-Type", "")
            body = r.read()
            return r.status, ctype, body
    except urllib.error.HTTPError as e:
        try:
            body = e.read()
        except Exception:
            body = b""
        return e.code, e.headers.get("Content-Type", "") if e.headers else "", body
    except Exception as e:
        return 0, "", str(e).encode()


while queue and len(seen) < LIMIT:
    path = queue.popleft()
    if path in seen:
        continue
    seen.add(path)

    status, ctype, body = fetch(path)

    if status in (200, 301, 302, 303, 304):
        ok += 1
    else:
        bad.append((path, status, found_by.get(path, "?")))
        continue

    if "html" not in ctype:
        continue

    text = body.decode("utf-8", "replace")
    for raw in href_re.findall(text):
        link = html.unescape(raw).strip()
        if not link.startswith("/baro"):
            continue
        link = link.split("#")[0]
        if not link or link in seen or link in found_by:
            continue
        # 내려받기·되돌릴 수 없는 동작으로 보이는 주소는 크롤하지 않는다
        if any(k in link for k in ("/download", "/file?", "/dump/file", "/backup/file",
                                   "/export/", "/delete", "/undo", "/execute", "/run",
                                   "/approve", "/close", "/leave", "/cancel", "/reset")):
            skipped += 1
            continue
        found_by[link] = path
        queue.append(link)

result = {
    "visited": len(seen),
    "ok": ok,
    "skipped_dangerous": skipped,
    "bad": [{"url": u, "status": s, "linked_from": f} for u, s, f in bad],
}

io.open("crawl_result.json", "w", encoding="utf-8").write(
    json.dumps(result, ensure_ascii=False, indent=1))

lines = ["방문 %d곳 · 정상 %d · 문제 %d (내려받기·파괴 동작 %d곳은 건너뜀)"
         % (len(seen), ok, len(bad), skipped)]
if bad:
    lines.append("")
    lines.append("[문제 있는 링크]")
    for u, s, f in bad:
        lines.append("  [%s] %-46s  <- %s" % (s, u, f))
else:
    lines.append("끊어진 링크 없음.")

io.open("crawl_report.txt", "w", encoding="utf-8").write("\n".join(lines))
sys.stdout.buffer.write(("\n".join(lines)).encode("utf-8"))
