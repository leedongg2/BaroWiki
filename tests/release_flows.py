"""격리된 로컬 위키에서 권한·그래프·동시 편집을 실제 HTTP로 검증한다.

python tests/release_flows.py http://127.0.0.1:39311
tests/seed.ps1로 만든 admin 계정이 필요하다. 테스트 문서를 새로 생성한다.
"""
import concurrent.futures
import html
import http.cookiejar
import re
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

BASE = sys.argv[1].rstrip('/')
assert urllib.parse.urlparse(BASE).hostname in ('127.0.0.1', 'localhost', '::1'), 'Local test servers only'
PREFIX = 'ReleaseCheck' + str(time.time_ns())
count = 0


def client():
    return urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))


def fetch(opener, path, data=None, headers=None):
    body = urllib.parse.urlencode(data).encode() if data is not None else None
    request = urllib.request.Request(BASE + urllib.parse.quote(path, safe='/?=&%'), data=body, headers=headers or {})
    try:
        response = opener.open(request, timeout=30)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        return response.status, response.read().decode('utf-8'), response.headers


def check(condition, message):
    global count
    assert condition, message
    count += 1
    print('PASS:', message)


def edit(opener, doc, text, revision):
    return fetch(opener, '/edit/' + doc, {'content': text, 'ver': str(revision), 'send': 'release test', 'copyright_agreement': 'on'})


admin, guest, member = client(), client(), client()
fetch(admin, '/login', {'id': 'admin', 'password': 'admin1234!'})
fetch(member, '/register', {'id': PREFIX, 'password': 'release12345!', 'password_check': 'release12345!'})
fetch(member, '/login', {'id': PREFIX, 'password': 'release12345!'})
check(fetch(admin, '/readyz')[0] == 200, 'DB readiness')
check(fetch(guest, '/login', {'id': 'admin'}, {'Origin': 'https://attacker.example'})[0] == 403, 'cross-origin POST rejected')
check(fetch(admin, '/user')[2].get('Cache-Control') == 'private, no-store', 'authenticated pages cannot be cached publicly')

public, private = PREFIX + 'Public', PREFIX + 'Private'
edit(admin, public, 'Public body', 0)
edit(admin, private, 'Private body', 0)
fetch(admin, '/baro/doc/graph/' + public, {'rel_person': 'KnownPerson'})
fetch(admin, '/baro/doc/graph/' + private, {'rel_person': 'SecretRelationValue'})
check('KnownPerson' in fetch(guest, '/baro/doc/graph/' + public)[1], 'graph displays saved relations to readers')
fetch(admin, '/acl/' + private, {'view': 'admin', 'document_edit_acl': 'admin'})
check('SecretRelationValue' not in fetch(guest, '/baro/doc/graph/' + private)[1], 'graph respects document read ACL')
check(private not in fetch(guest, '/baro/doc/graph?q=' + PREFIX)[1], 'graph list excludes restricted documents')
fetch(admin, '/acl/' + public, {'document_edit_acl': 'admin'})
fetch(member, '/baro/doc/graph/' + public, {'rel_person': 'ForbiddenReplacement'})
check('ForbiddenReplacement' not in fetch(admin, '/baro/doc/graph/' + public)[1], 'graph respects document edit ACL')

doc = PREFIX + 'Conflict'
edit(admin, doc, 'first\nsecond\nthird', 0)
edit(admin, doc, 'first\nsecond\nthird changed', 1)
conflict = edit(admin, doc, 'first changed\nsecond\nthird', 1)[1]
match = re.search(r'id="baro_conflict_area"[^>]*>(.*?)</textarea>', conflict, re.S)
check(match is not None, 'stale revision opens conflict resolution')
merged = html.unescape(match.group(1))
check('first changed' in merged and 'third changed' in merged, 'three-way merge preserves both edits')
edit(admin, doc, merged, 2)
check('first changed' in fetch(admin, '/w/' + doc)[1], 'merged revision can be saved')

race_doc = PREFIX + 'Concurrent'
edit(admin, race_doc, 'base', 0)
# 같은 쿠키를 사용하는 독립 HTTP 클라이언트로 같은 판을 동시에 저장한다.
cookie_header = next(h.cookiejar for h in admin.handlers if isinstance(h, urllib.request.HTTPCookieProcessor))
cookie = '; '.join(c.name + '=' + c.value for c in cookie_header)
def save(index):
    return fetch(client(), '/edit/' + race_doc, {'content': 'writer-' + str(index), 'ver': '1', 'send': 'parallel test', 'copyright_agreement': 'on'}, {'Cookie': cookie})[1]
with concurrent.futures.ThreadPoolExecutor(max_workers=8) as workers:
    pages = list(workers.map(save, range(8)))
check(sum('id="baro_conflict_area"' in page for page in pages) == 7, 'simultaneous edits yield one save and seven conflicts')
revision = fetch(admin, '/edit/' + race_doc)[1]
check(re.search(r'name="ver" value="2"', revision) is not None, 'concurrent saves create exactly one new revision')
print('PASS:', count, '/ FAIL: 0')
