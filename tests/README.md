# 기능 테스트 스크립트

새로 추가된 기능(이름공간 / 차단 / 경고 / 권한 / ACL / 게시판 / 업로드 / 문단 편집)을 실제 서버에 요청을 보내 확인하는 PowerShell 스크립트입니다.

## 준비

```powershell
# 1. 위키 실행 (테스트용 폴더에서 실행하는 것을 권장)
.\onamu.exe 3010

# 2. 최초 1회 : 관리자 계정 + 예시 문서 생성
.\tests\seed.ps1
```

`seed.ps1` 은 `admin / admin1234!` 계정을 만듭니다. (첫 계정이므로 소유자 권한을 가집니다)

## 실행

```powershell
.\tests\test_features.ps1     # 이름공간 / 휴지통 / 차단 / 경고 / 권한 / 이름공간 ACL
.\tests\test_bbs_upload.ps1   # 게시판 / 게시판 차단 / 게시판 잠금 / mp3 · mp4 업로드 / 파일휴지통
.\tests\test_section.ps1      # 문단([편집]) 편집
.\tests\test_privacy.ps1      # 휴지통 문서 노출 차단 (검색 / 목록 / 열람)
.\tests\test_crawl.ps1        # 주요 페이지 오류(500 / panic) 점검
```

각 스크립트는 마지막 줄에 `PASS: n / FAIL: n` 을 출력합니다.

## 주의

- 스크립트는 문서와 계정을 실제로 만들고 지웁니다. **운영 중인 위키가 아닌 테스트용 폴더에서 실행하세요.**
- 기본 접속 주소는 `http://127.0.0.1:3010` 입니다. 다른 포트를 쓰려면 `-BaseUrl` 인자를 주세요.

```powershell
.\tests\test_features.ps1 -BaseUrl http://127.0.0.1:3000
```

## 바로위키 확인 스크립트

```powershell
# 1) 서버를 한 번 띄웠다 내려서 표를 만든다
.\start.ps1        # 뜨면 Ctrl+C

# 2) 확인용 데이터를 넣는다 (시험용 사본에만!)
python testsaro_seed.py data.db

# 3) 서버를 띄운 채로 다른 창에서
python testsaro_smoke.py . 3000          # /baro 화면 전부가 오류 없이 뜨는지
python testsaro_conflict_test.py 3000    # 편집 충돌 3자 병합이 되는지
```

- `baro_smoke.py` : `routes_baro_*.go` 에 등록된 모든 GET 주소를 실제로 불러
  500 이 나는 화면을 찾아낸다. 결과는 `smoke_result.json` 에 남는다.
- `baro_conflict_test.py` : 같은 문서를 두 사람이 동시에 고치는 상황을 재현해
  양쪽 수정이 모두 살아남는지 확인한다. 결과는 `conflict_result.txt` 에 남는다.
- `baro_crawl.py` : /baro 에서 시작해 실제로 눌러 볼 수 있는 링크를 따라가며
  404 · 500 이 나는 곳을 찾는다. 내려받기와 되돌릴 수 없는 동작은 건너뛴다.
  결과는 `crawl_report.txt` 와 `crawl_result.json` 에 남는다.
- `baro_seed.py` : 목록 화면이 비어 있으면 버그가 보이지 않으므로 확인용 데이터를 넣는다.

단위 시험은 Go 쪽에 있다.

```powershell
go test ./...
go test -race ./...
go vet ./...
node tests/editor_runtime_test.js
```

`release_flows.py`는 테스트용 서버에서 지식 그래프 저장·권한, 외부 출처 POST 차단,
편집 충돌 병합, 동일한 판의 동시 저장을 확인합니다. `seed.ps1` 실행 후 사용하세요.

```powershell
python tests/release_flows.py http://127.0.0.1:3010
```

추가 성능 검사는 문서 10,001개와 편집 기록 50,000개를 별도 DB에 넣고,
6개 화면을 각각 예열 1회 후 동시 요청 8개로 32회 측정합니다.
`benchmark_seed.py`는 원본을 읽기 전용으로 열며 대상 파일이 있으면 중단합니다.

```powershell
# 초기화한 테스트 DB에서 별도 측정용 DB 생성
python tests/benchmark_seed.py path/to/test-source.db .cache/benchmark/data.db
# .cache/benchmark 폴더를 작업 경로로 하여 비교할 실행 파일을 39321 포트에서 실행
python tests/benchmark_http.py http://127.0.0.1:39321 .cache/benchmark/result.json
```

두 실행 파일을 비교할 때는 동일한 초기 DB 사본을 사용하고, 빌드나 다른 부하 검사와
동시에 측정하지 마세요. `wiki_setting_cache_test.go`는 설정 변경의 즉시 반영,
민감 설정의 캐시 제외, 동시 접근, SQLite의 실제 인덱스 사용을 검사합니다.
`history_performance_test.go`는 삭제된 판의 공백과 숨김 이력의 분리를,
`document_lookup_test.go`는 다음 렌더에서 문서 생성·삭제가 반영되는지를 검사합니다.

- 3자 병합 · 줄 비교 · 편집 분류 · 초성 · 개척률 · IP 가림 · 각주/분류/링크 세기
- 2단계 인증(TOTP) RFC 6238 공식 시험값
