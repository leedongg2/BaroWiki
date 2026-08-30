"""바로위키 라우트 스모크 테스트

routes_baro_*.go 에 등록된 모든 GET 라우트를 뽑아 실제로 호출하고,
500 이 나오는 화면을 찾아낸다. (권한 없음 화면은 200 이어야 정상)

사용법 : python smoke.py <저장소경로> <포트>
"""
import io
import os
import re
import sys
import glob
import json
import urllib.parse
import urllib.request
import urllib.error

REPO = sys.argv[1]
PORT = sys.argv[2] if len(sys.argv) > 2 else "39300"
BASE = "http://127.0.0.1:" + PORT

# 경로 변수에 넣어 볼 시험 값 (존재하지 않는 값이어도 500 이 나면 안 된다)
SAMPLE = {
    "num": "1",
    "page": "1",
    "id": "1",
    "key": "history",
    "name": "history",
    "code": "TESTCODE",
    "user": "테스트사용자",
    "field": "history",
    "year": "2026",
    "rev": "1",
    "doc_name": "FrontPage",
    "title": "FrontPage",
    "asset_id": "1",
    "old": "1",
    "new": "2",
}

route_re = re.compile(r'r\.(GET|POST)\(\s*"([^"]+)"')
# 도우미 함수로 등록하는 모듈도 있으므로 "/baro/..." 문자열을 전부 훑는다
path_re = re.compile(r'"(/baro(?:/[^"\s]*)?)"')


def collect():
    routes = {}
    files = sorted(glob.glob(os.path.join(REPO, "routes_baro_*.go")))
    for path in files:
        src = io.open(path, encoding="utf-8").read()
        mod = os.path.basename(path)
        for method, url in route_re.findall(src):
            routes.setdefault((method, url), mod)
        for url in path_re.findall(src):
            routes.setdefault(("GET", url), mod)
    return routes, files


def fill(url):
    out = []
    for part in url.split("/"):
        if part.startswith(":"):
            out.append(SAMPLE.get(part[1:], "1"))
        elif part.startswith("*"):
            out.append(SAMPLE.get(part[1:], "FrontPage"))
        else:
            out.append(part)
    return "/".join(out)


def fetch(url):
    full = BASE + urllib.parse.quote(fill(url), safe="/?=&")
    req = urllib.request.Request(full, headers={"User-Agent": "baro-smoke"})
    try:
        with urllib.request.urlopen(req, timeout=25) as resp:
            return resp.status, len(resp.read())
    except urllib.error.HTTPError as err:
        try:
            body = err.read()
        except Exception:
            body = b""
        return err.code, len(body)
    except Exception as err:
        return 0, str(err)


def main():
    routes, files = collect()
    gets = sorted([u for (m, u) in routes if m == "GET"])
    posts = sorted([u for (m, u) in routes if m == "POST"])

    print("등록 파일 %d개 · GET %d개 · POST %d개" % (len(files), len(gets), len(posts)))
    print("-" * 70)

    bad = []
    ok = 0
    for url in gets:
        status, info = fetch(url)
        if status in (200, 301, 302, 303, 304, 401, 403, 404):
            ok += 1
            continue
        bad.append((url, status, info, routes.get(("GET", url), "?")))
        print("  [%s] %-52s (%s)" % (status, url, routes.get(("GET", url), "?")))

    print("-" * 70)
    print("정상 %d / %d" % (ok, len(gets)))
    if bad:
        print("문제 %d건" % len(bad))
    else:
        print("모든 GET 화면이 오류 없이 응답했습니다.")

    io.open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "smoke_result.json"),
            "w", encoding="utf-8").write(
        json.dumps({"total": len(gets), "ok": ok,
                    "bad": [{"url": b[0], "status": b[1], "module": b[3]} for b in bad],
                    "post_routes": posts}, ensure_ascii=False, indent=1))

    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main())
