# -*- coding: utf-8 -*-
"""편집 충돌 3자 병합 흐름 시험

사용법 : python conflict_test.py <포트>
"""
import io
import re
import sys
import html
import urllib.parse
import urllib.request

PORT = sys.argv[1] if len(sys.argv) > 1 else "39307"
BASE = "http://127.0.0.1:" + PORT
DOC = "충돌시험2"
URL = BASE + "/edit/" + urllib.parse.quote(DOC)


def post(content, send, ver):
    body = urllib.parse.urlencode({
        "content": content, "send": send, "ver": ver,
        "copyright_agreement": "on",
    }, encoding="utf-8").encode()
    req = urllib.request.Request(URL, data=body, method="POST")
    req.add_header("Content-Type", "application/x-www-form-urlencoded")
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            return r.status, r.read().decode("utf-8", "replace")
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode("utf-8", "replace")


out = []

s1, _ = post("첫째 줄\n둘째 줄\n셋째 줄", "최초 작성", "0")
out.append("1) 기준 판 r1 만들기 -> %s" % s1)

s2, _ = post("첫째 줄\n둘째 줄\n셋째 줄 (남이 고침)", "남의 편집", "1")
out.append("2) 다른 사람이 셋째 줄 수정 (r2) -> %s" % s2)

s3, page = post("첫째 줄 (내가 고침)\n둘째 줄\n셋째 줄", "내 편집", "1")
out.append("3) 내가 r1 기준으로 첫째 줄 수정 -> %s  (302 면 충돌 미감지, 200 이면 충돌 화면)" % s3)
out.append("")

for m in re.findall(r'<div class="bw_alert[^"]*">(.*?)</div>', page, re.S)[:3]:
    out.append("  안내 : " + re.sub("<[^>]+>", "", m).strip()[:150])

for m in re.findall(r'<div class="bw_stat_v">(.*?)</div>\s*<div class="bw_stat_l">(.*?)</div>', page, re.S)[:4]:
    out.append("  지표 : %s = %s" % (re.sub("<[^>]+>", "", m[1]).strip(),
                                    re.sub("<[^>]+>", "", m[0]).strip()))

ta = re.search(r'id="baro_conflict_area"[^>]*>(.*?)</textarea>', page, re.S)
if ta:
    out.append("")
    out.append("  --- 자동 병합된 본문 ---")
    for line in html.unescape(ta.group(1)).split("\n"):
        out.append("    " + line)
else:
    out.append("  (편집창을 찾지 못했습니다)")

out.append("")
out.append("  초안 보존 안내 있음 : %s" % ("예" if "초안" in page else "아니오"))
out.append("  세 가지 고르기 버튼 있음 : %s" % ("예" if "baro_conflict_pick" in page else "아니오"))
out.append("  비교 표 있음 : %s" % ("예" if "bw_diff" in page else "아니오"))

io.open("conflict_result.txt", "w", encoding="utf-8").write("\n".join(out))
print("wrote conflict_result.txt")
