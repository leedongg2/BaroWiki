# 바로위키 구현 계약서 (BARO_CONTRACT)

이 문서는 바로위키 확장 모듈을 만드는 모든 작업자가 **먼저 읽고 그대로 따라야 하는 규칙**이다.
기반 계층(스키마 · 공통 함수 · UI 부품)은 이미 완성되어 있다. 새로 만들지 말고 **가져다 쓴다**.

---

## 0. 프로젝트 구조

openNAMU 를 Go 로 포팅한 위키 엔진이다. (Gin + pongo2 + SQLite/MySQL)

```
app.py / main.go          진입점
routes.go                 라우트 등록 묶음 (package main)
routes_*.go               기능별 라우트 등록 (package main)
route/view_*.go           화면 함수 (package route) — HTML 문자열을 돌려준다
route/api_*.go            API 함수 (package route)
route/tool/*.go           공통 도구 (package tool)
route/tool/baro_*.go      ★ 바로위키 기반 계층 (수정 금지)
views/                    스킨 (embed)
```

### 화면 함수의 표준 모양

```go
// route/view_baro_xxx.go
package route

import (
	"net/url"
	"opennamu/route/tool"
)

// View_baro_xxx : 설명
func View_baro_xxx(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Baro_can(db, config.IP, "review") {
		return tool.Baro_deny(db, config, "")
	}

	body := tool.Baro_card("제목", "내용")

	return tool.Baro_page(db, config, "쪽 제목", body, [][]any{{"baro", "바로위키"}})
}
```

- `values` 가 `nil` 이면 GET(보여주기), `nil` 이 아니면 POST(처리)로 다루는 것이 이 코드베이스의 관례다.
- 처리 후에는 `tool.Get_redirect("/baro/...")` 를 돌려준다. (`write_data` 가 302 로 바꿔 준다)
- 절대 `panic` 을 일부러 내지 않는다. `Query_DB` / `Exec_DB` 는 실패 시 스스로 panic 하고 전역 복구기가 잡는다.

### 라우트 등록의 표준 모양

```go
// routes_baro_xxx.go
package main

import (
	"net/http"
	"opennamu/route"
	"github.com/gin-gonic/gin"
)

func register_baro_xxx_routes(r *gin.Engine) {
	r.GET("/baro/xxx", func(c *gin.Context) {
		data := route.View_baro_xxx(make_route_config(c), "1", nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.POST("/baro/xxx", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_baro_xxx(make_route_config(c), "1", c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})
}
```

쓸 수 있는 도우미 (package main, 이미 존재):
`make_route_config(c) tool.Config` · `write_data(c, status, contentType, []byte)` · `doc_param(c) string` · `captcha_response(c) string`

---

## 1. 절대 규칙

1. **기반 파일을 수정하지 않는다.**
   `route/tool/baro_schema.go`, `baro_core.go`, `baro_ui.go`, `baro_init.go`, `route/tool/init.go`,
   그 밖의 기존 openNAMU 파일 전부. (읽는 것은 자유)
2. **내 모듈 파일만 새로 만든다.** 파일 이름은 아래 모듈 표의 접두사를 그대로 쓴다.
3. **package route 의 비공개 함수·변수·타입 이름은 반드시 `baro_<모듈키>_` 로 시작한다.**
   (예: `baro_project_row_html`) 공개 함수는 `View_baro_<모듈키>_...` 로 시작한다.
   16개 모듈이 같은 패키지에 들어가므로 이름이 겹치면 전체가 깨진다.
4. **URL 은 반드시 배정받은 접두사 아래에만 만든다.** (`/baro/<모듈접두사>/...`)
   접두사 바로 다음 칸에 `:param` 을 두지 않는다. (gin 라우트 충돌)
5. **다른 모듈의 Go 함수를 부르지 않는다.** 다른 모듈 데이터가 필요하면 **테이블을 직접 SQL 로 읽는다.**
   (표 정의는 이 문서 5장에 전부 있다)
6. **새 테이블·새 컬럼을 만들지 않는다.** 필요한 것은 이미 다 있다. 정말 없으면 `baro_doc_meta`,
   `baro_user_set`, `baro_set` 의 이름-값 저장소나 JSON 컬럼(`data`, `detail`, `options`)을 쓴다.
7. **모든 출력은 `tool.HTML_escape` 를 거친다.** UI 부품 중 `Baro_table` 의 각 칸과 `Baro_alert`,
   `Baro_card` 의 본문은 "이미 HTML" 로 취급하니 그 안에 넣는 사용자 입력은 직접 escape 한다.
8. **SQL 은 반드시 `?` 자리표시자를 쓴다.** 문자열을 이어 붙여 값을 넣지 않는다.
9. `rows := tool.Query_DB(...)` 뒤에는 **항상** `defer rows.Close()`.
10. 화면 문구는 **한국어**로 쓴다. 코드 주석도 한국어로 쓴다.
11. **화면에 나가는 한국어는 `tool.Baro_t(...)` 로 감싼다.** 한국어 원문 자체가 열쇠말이므로
    따로 키를 만들지 않는다. 번역이 없으면 한국어가 그대로 나오므로 화면이 비지 않는다.
    - `Baro_t` / `Baro_t_esc` : 문구 하나
    - `Baro_t_list` / `Baro_t_pairs` / `Baro_t_menu` : 목록 · 라벨-값 쌍 · 메뉴
    - 번역표 : `lang/baro-en-US.json`, `lang/baro-ja-JP.json` (열쇠말 = 한국어 원문)
    - 언어는 `other` 표의 `language` 값 하나로 위키 전체가 정한다. (`tool.Wiki_language`)
12. **DB 에 저장하거나 값을 비교하는 자리에서는 `Baro_t` 를 부르지 않는다.**
    문서 제목 · 본문 · 편집 요약 · 공개/감사 로그 · 설정값 · 정규식 · SQL 조각 · 씨앗 데이터는
    언어와 무관하게 항상 한국어로 저장한다. (언어를 바꿔도 기록이 어긋나지 않게)
    - 다만 **알림(`Baro_notify`)** 은 사람에게 보내는 말이므로 번역해도 된다.
      기록(증거)과 메시지를 나누어 본다.
13. **이름공간 표시 이름은 `tool.Namespace_name_now(ns)` 로 얻는다.**
    `ns.Name` 은 ACL 열쇠말이라 언제나 한국어다. 화면에 그대로 쓰지 않는다.

---

## 2. 0장 고정 원칙 — 코드로 지켜야 하는 것

| 원칙 | 코드에서의 의미 |
| --- | --- |
| 1. 제재와 콘텐츠 판단 분리 | 사용자 제재 화면에 "이 사용자 문서 전부 삭제" 같은 버튼을 만들지 않는다. 제재는 `baro_sanction` 만 건드린다. |
| 2. 삭제보다 보존·복구 | 삭제 기본 경로는 `baro_quarantine` 임시 격리다. 완전 삭제는 별도 확인 화면 + 사유 필수. |
| 3. 대량 작업 안전장치 | `tool.Baro_mass_threshold(db)` 개를 넘는 작업은 `baro_mass_op` 를 거친다. 대상 수 표시 → 목록 공개 → 2인 승인 → 스냅샷 → 유예 → 실행 → 공지 → 일괄 복구. |
| 4. 횟수보다 가치 | 편집 횟수를 그대로 점수로 쓰지 않는다. `baro_contrib_event.kind` 로 행동을 나누어 기록한다. |
| 5. 근거 공개 · 개인정보 보호 | 공개용은 `tool.Baro_public_log`, 내부용은 `tool.Baro_audit`. 공개 화면에 IP·이메일·신고 원문을 그대로 쓰지 않는다. |

---

## 3. 기반 계층 API (package tool)

> 아래 함수는 이미 구현되어 있다. **그대로 호출**한다. 시그니처를 바꾸지 않는다.

### 3-1. openNAMU 본체에서 쓸 것

```go
DB_connect() *sql.DB                  // defer tool.DB_close(db)
DB_close(db *sql.DB)
Query_DB(db, query string, values ...any) *sql.Rows
QueryRow_DB(db, query string, var_list []any, values ...any) bool
Exec_DB(db, query string, values ...any)

Get_time() string                     // "2006-01-02 15:04:05"
Get_date() string                     // "2006-01-02"
Get_month() string                    // "2006-01"
Str_to_int(data string) int
Get_len(data string) int              // 글자 수 (rune)
Get_slice(data string, s, e int) string
HTML_escape(data string) string
Url_parser(data string) string        // URL 안전 인코딩
JS_escape(data string) string
Get_random_key(n int) string
Sha224(data string) string

Config{IP, Cookies, UserAgent string; Session sessions.Session}
IP_or_user(ip string) bool            // true 면 IP(비로그인)
IP_parser(db, ip, my_ip string) string
Check_acl(db, doc_name, topic_num, tool_name, ip string) bool
Get_user_auth(db, ip string) string
Get_user_gravatar_url(db, user string, size int) string
Get_error_page(db, config, error_name string) string
Get_redirect(target string) string    // 화면 함수의 반환값으로 쓴다
Get_template(db, config, title, data string, other []any, menu [][]any, option map[string]string) string
Get_language(db, key string, safe bool) string
Send_alarm(db, from, target, data string)          // openNAMU 기본 알림
Namespace_display(doc_name string) string
Namespace_normalize(doc_name string) string
Do_add_history(db, doc_name, data, date, ip, send, length, mode, type_check string)
Get_document_revision(db, doc_name string) string
```

### 3-2. 바로위키 공통 (route/tool/baro_core.go)

```go
const Baro_ver = "1.0"
const Baro_page_size = 30

// 설정
Baro_get(db, name string) string
Baro_get_def(db, name, def string) string
Baro_get_int(db, name string, def int) int
Baro_get_on(db, name string) bool
Baro_put(db, name, data string)

// ID / 시간 / 문자열
Baro_next_id(db, table_name string) string     // table_name 은 반드시 baro_ 로 시작
Baro_now() string
Baro_today() string
Baro_date_only(data string) string
Baro_parse_time(data string) time.Time
Baro_time_add(data string, days, hours, minutes int) string
Baro_days_between(a, b string) int
Baro_is_past(data string) bool                 // "" 는 무기한 → false
Baro_time_ago(data string) string              // "3일 전"
Baro_duration_left(end string) string          // "5일 남음" / "무기한"
Baro_itoa(num int) string
Baro_comma(num int) string                     // 1,234
Baro_pct(done, total int) float64
Baro_pct_text(done, total int) string          // "31.2%"
Baro_trunc(data string, max int) string
Baro_one_line(data string) string
Baro_chosung(data string) string               // 초성 추출
Baro_is_chosung_only(data string) bool
Baro_norm(data string) string                  // 검색 정규화
Baro_split_lines(data string) []string

// JSON
Baro_json_enc(value any) string
Baro_json_dec(data string) map[string]any
Baro_json_dec_str(data string) map[string]string
Baro_json_dec_list(data string) []string

// 사용자 / 권한
Baro_is_user(ip string) bool                   // 로그인 사용자인가
Baro_is_admin(db, ip string) bool
Baro_is_owner(db, ip string) bool
Baro_role_list() [][]string                    // {키, 이름, 설명}
Baro_role_name(role string) string
Baro_user_roles(db, user string) []string
Baro_has_role(db, user, role string) bool
Baro_grant_role(db, user, role, by, memo string)
Baro_revoke_role(db, user, role, by, memo string)
Baro_can(db, ip, capability string) bool
Baro_deny(db, config, message string) string           // 권한 없음 화면
Baro_login_wall(db, config, next string) (string, bool) // 두 번째 값이 true 면 그대로 반환

// 기록
Baro_public_log(db, kind, actor, target, reason, detail string)
Baro_audit(db, config, action, target, before, after, reason string)
Baro_mask_ip(who string) string

// 분야
type Baro_field_row struct {
	Key, Name, Description string
	Sort, TargetCount, Recruit int
	RecruitOpen bool
	Kind string   // "closed"(분모 확정) / "open"
}
Baro_field_default() []Baro_field_row
Baro_field_list(db) []Baro_field_row
Baro_field_get(db, key string) Baro_field_row
Baro_field_name(db, key string) string
Baro_field_save(db, row Baro_field_row)
Baro_user_interest(db, user string) []string

// 알림
Baro_notify_kind_list() [][]string              // {키, 이름}
Baro_notify(db, user, kind, title, url, data string)
Baro_notify_count(db, user string) int

// 기여
Baro_contrib_kind_list() [][]string             // {키, 이름, 기본가중치}
Baro_contrib_kind_name(kind string) string
Baro_contrib_kind_weight(kind string) int
Baro_contrib_add(db, user, kind, title, rev, project, detail string) string  // 이벤트 ID 반환
Baro_calendar_bump(db, user, kind string)
Baro_impact_get(db, user, name string) int
Baro_impact_set(db, user, name string, value int)
Baro_impact_bump(db, user, name string, delta int)

// 문서 부가 정보
Baro_doc_meta_get(db, title, name string) string
Baro_doc_meta_set(db, title, name, data string)
Baro_doc_field(db, title string) string
Baro_doc_status_list() [][]string               // {키, 이름, 색상종류}
Baro_doc_status_name(status string) string
Baro_doc_status_kind(status string) string
Baro_doc_status_get(db, title string) string
Baro_doc_status_set(db, title, status, reason, actor string)
Baro_doc_quarantined(db, title string) bool

// 제재
Baro_sanction_kind_list() [][]string
Baro_sanction_name(kind string) string
Baro_sanction_active(db, user string) [][]string   // {id, kind, scope, reason, end_date, case_id}
Baro_sanction_has(db, user, kind, scope string) bool

// 대량 작업
Baro_mass_threshold(db) int
Baro_mass_grace_hours(db) int
Baro_mass_kind_list() [][]string
Baro_mass_kind_name(kind string) string

// 기능 플래그 / 창립
Baro_flag_on(db, user, key string) bool
Baro_founder_number(db, user string) string     // "" 면 창립 기여자 아님

// 고유 번호 (판 · 계정)
Baro_uuid_new() string                              // RFC 4122 판 4
Baro_uuid_is(data string) bool                      // 바깥에서 온 값은 반드시 이걸로 먼저 거른다
Baro_rev_uuid(db, title, rev string) string         // 판 고유 번호 (없으면 그 자리에서 붙인다)
Baro_rev_uuid_list(db, pairs [][2]string) map[string]string  // 목록용 일괄 조회, 열쇠말은 "문서\n판"
Baro_rev_from_uuid(db, title, value string) string  // 번호 → 판 번호 ("" 면 못 찾음)
Baro_rev_resolve(db, title, rev string) string      // 판 자리 값 정리 (번호가 아니면 그대로)
Baro_user_uuid(db, user string) string              // 계정 고유 번호 (아이피는 "")
Baro_user_from_uuid(db, value string) string        // 번호 → 계정
Baro_user_name_log_add(db, user, old_name, new_name, actor string)  // 계정명 변경 이력

// 밐 (가상 화폐) · 바로샵
const Baro_mik_unit = "밐"
Baro_mik_get(db, user string) int
Baro_mik_add(db, user string, delta int, kind, reason, actor, target string) bool  // 음수면 잔액 안에서만
Baro_mik_text(value int) string                     // "1,234밐"
Baro_mik_kind_name(kind string) string
Baro_mik_attend(db, user string) int                // 하루 한 번 접속 보상 (이미 받았으면 0)
Baro_mik_tick(db, user string)                      // 화면 그릴 때마다 부르는 안전한 겉옷
Baro_mik_attend_date(db, user string) string
Baro_mik_attend_streak(db, user string) int
Baro_mik_attend_amount(db) int
Baro_mik_badge_default(db) int                      // 따로 정하지 않은 배지에 쓰는 값
Baro_mik_badge_amount(db, badge_key string) int     // 배지마다 따로 정할 수 있다
Baro_mik_badge_own(db, badge_key string) int        // 따로 정해 둔 값 (없으면 -1)
Baro_mik_badge_set(db, badge_key, value string)     // "" 를 주면 기본값을 따른다
Baro_mik_badge_reward(db, user, badge_key, badge_name string) int

// 하루 양도 한도 (baro_set 의 mik_send_daily, 기본 100)
Baro_mik_send_limit(db) int
Baro_mik_send_today(db, user string) int
Baro_mik_send_left(db, user string) int

// 밐 순위 (지금 · 최대 · 누적)
Baro_mik_best(db, user string) int                  // 지금까지 가졌던 가장 많은 밐
Baro_mik_total(db, user string) int                 // 지금까지 들어온 밐의 합
Baro_mik_rank_kind_list() [][]string                // {키, 이름, 설명}
Baro_mik_rank(db, kind string, offset, size int) []Baro_mik_rank_row
Baro_mik_rank_count(db, kind string) int
Baro_mik_backfill(db)                               // 누계가 생기기 전에 쌓인 밐 되살리기 (한 번만)
Baro_mik_title(db, user string) string              // 달고 있는 칭호
Baro_mik_title_list(db, user string) []string       // 가진 칭호
Baro_mik_title_set(db, user, title string)
Baro_mik_captcha_until(db, user string) string      // "" 면 면제 없음
Baro_mik_captcha_free(db, user string) bool
Baro_mik_captcha_extend(db, user string, days int) string

// 바로 캡차 (위키가 스스로 내는 문제)
//   갈래 : Baro_captcha_gate_pass / _once / _baro / _out
Baro_captcha_gate(db, ip string) string             // 그리는 자리와 검사하는 자리가 이것 하나를 본다
Baro_captcha_mode(db) string                        // off / ip / all
Baro_captcha_mode_list() [][]string
Baro_captcha_target(db, ip string) bool             // 물을 무리에 드는가 (면제는 안 따짐)
Baro_captcha_exempt(db, ip string) (bool, string)   // 면제인가 + 까닭
Baro_captcha_need(db, ip string) bool
Baro_captcha_new(db, who string) string             // 문제 하나 → 이름표
Baro_captcha_answer(db, code string) string         // 그림 그릴 때만 쓴다
Baro_captcha_verify(db, code, typed string) bool    // 맞든 틀리든 그 문제는 사라진다
Baro_captcha_png(answer string) []byte
Baro_captcha_pack(code, typed string) string        // "bwcap:<이름표>:<답>"
Baro_captcha_unpack(data string) (string, string, bool)
Baro_captcha_ui(db, ip string) string
Baro_captcha_once_left(db, user string) int
Baro_captcha_once_add(db, user string, num int) int
Baro_captcha_once_use(db, user string) bool

// 사용자 지정 테마 (스킨의 --namu-* 토큰을 덮어쓴다)
Baro_theme_token_list() []Baro_theme_token          // {Key, Var, Group, Name, Light, Dark}
Baro_theme_group_list() []string
Baro_theme_token_get(key string) (Baro_theme_token, bool)
Baro_theme_color_norm(data string) string           // 쓸 수 없는 값은 "" — 담을 때도 그릴 때도 거른다
Baro_theme_value_join(light, dark string) string    // "밝은쪽|어두운쪽"
Baro_theme_value_split(data string) (string, string)
Baro_theme_get(db, uuid string) (Baro_theme_row, bool)
Baro_theme_list(db, kind, maker string, offset, size int) []Baro_theme_row  // official / public / mine
Baro_theme_count(db, kind, maker string) int
Baro_theme_add(db, row Baro_theme_row) string       // 고유 번호를 돌려준다
Baro_theme_save(db, row Baro_theme_row)
Baro_theme_official_set(db, uuid string, on bool)
Baro_theme_delete(db, uuid string)                  // 쓰던 사람은 기본 색으로 돌아간다
Baro_theme_can_edit(db, row Baro_theme_row, ip string) bool
Baro_theme_now(db, ip string) string                // 쓰고 있는 테마
Baro_theme_use(db, ip, uuid string)
Baro_theme_css(row Baro_theme_row, dark bool) string
Baro_theme_head(db, ip string, dark bool) string    // 화면 그릴 때 넣는 <style>
Baro_theme_preview(row Baro_theme_row, dark bool) string

// 칭호 (글자색까지)
Baro_mik_title_rows(db, user string) []Baro_mik_title_row   // {Name, Color, Dark}
Baro_mik_title_get(db, user string) Baro_mik_title_row
Baro_mik_title_html(row Baro_mik_title_row) string
Baro_mik_title_user_html(db, user string) string

// 편집방
//   갈래 : Baro_coop_public / _code / _private, 상태 : _open / _closing / _closed
//   자리 : Baro_coop_owner / _sub / _edit / _watch
Baro_coop_link_new() string                         // 소리 나는 대로 읽히는 주소 이름표
Baro_coop_code_new() string                         // 12자리 숫자 열쇠
Baro_coop_ban_parse(data string) []Baro_coop_ban_word
Baro_coop_ban_apply(data string, list []Baro_coop_ban_word) (string, []string)
Baro_coop_ban_notice(hit []string, after string) string
Baro_coop_room_get(db, link string) (Baro_coop_room_row, bool)      // 읽을 때 닫을 때를 함께 본다
Baro_coop_room_by_code(db, code string) (Baro_coop_room_row, bool)
Baro_coop_room_list(db, kind, user string, offset, size int) []Baro_coop_room_row
Baro_coop_room_add(db, row Baro_coop_room_row) string
Baro_coop_room_save(db, row Baro_coop_room_row)
Baro_coop_room_close(db, room string) string        // 10분 뒤에 닫힌다
Baro_coop_room_reopen(db, room string)
Baro_coop_member_get(db, room, user string) (Baro_coop_member_row, bool)
Baro_coop_member_list(db, room, status string) []Baro_coop_member_row
Baro_coop_member_set(db, room, user, role string)   // 들어오기 · 자리 바꾸기
Baro_coop_member_out(db, room, user string)         // 나가기 (기록은 남는다)
Baro_coop_member_block(db, room, user string, on bool)
Baro_coop_count(db, room string) (int, int)         // 편집 인원, 관전 인원
Baro_coop_full(db, row Baro_coop_room_row) bool
Baro_coop_owner_away(db, row Baro_coop_room_row) bool
Baro_coop_can_manage(db, row Baro_coop_room_row, ip string) bool
Baro_coop_can_open(db, ip string) bool              // baro_set 의 coop_open_role + coop_open 권한
Baro_coop_can_view(db, ip string) bool              // coop_view 권한
Baro_coop_can_join(db, ip string) bool              // coop_join 권한
Baro_coop_auth(db, ip, name string) bool            // 권한 그룹이 준 편집방 권한
Baro_coop_can(db, row, ip, act string) bool         // 방이 정한 범위 + 위키 권한을 함께 본다
Baro_coop_act_list() [][]string                     // {열쇠, 이름, 기본 범위, 설명}
Baro_coop_scope_list() [][]string                   // 방장만 / 부방장까지 / 참여자 모두
Baro_coop_acl_get(row, act string) string
Baro_coop_acl_text(pick map[string]string) string
Baro_coop_full_for(db, row, user string) bool       // 방장은 정원 밖
Baro_coop_room_seal(db, room, by, reason string)    // 영구 폐쇄 (관리자만)
Baro_coop_room_unseal(db, room string)              // 영구 폐쇄 해제 (관리자만)
Baro_coop_room_owner_set(db, row, next string) bool // 방장 넘기기 (참여한 적 있는 사람만)
Baro_coop_room_ever_list(db, room string) []Baro_coop_member_row
Baro_coop_room_oldest(db, row) string               // 가장 먼저 들어온 사람
Baro_coop_invite(db, row, by, target string) bool   // 알림 + 「불림」 자리 올리기
Baro_coop_doc_list(db, room string) []Baro_coop_doc_row
Baro_coop_doc_bump(db, room, title string) int      // 같은 문서를 또 고치면 횟수가 오른다
Baro_coop_doc_open(db, room, title, opener string)  // 작업 공간에 탭 열기
Baro_coop_doc_close(db, room, title string)         // 탭 닫기 (손댄 기록은 남는다)
Baro_coop_doc_open_list(db, room string) []Baro_coop_doc_row
Baro_coop_doc_get(db, room, title string) (Baro_coop_doc_row, bool)
Baro_coop_log_add(db, room, kind, user, data, detail string) string
Baro_coop_log_list(db, room, kind string, limit int) []Baro_coop_log_row
Baro_coop_now(db, user string) (Baro_coop_room_row, bool)   // 지금 편집 자리로 든 방
Baro_coop_edit_mark(db, user, doc_name, send string) string // 편집 요약 뒤에 방을 붙인다

// AI 참고 표시 (편집 요약 뒤에 「(AI 참고)」)
const Baro_ai_field = "baro_ai"          // 폼 칸 이름
const Baro_ai_mark = "(AI 참고)"
Baro_ai_on(value string) bool             // 체크했는가
Baro_ai_has(send string) bool             // 이미 붙어 있는가
Baro_ai_send(send string, on bool) string // 요약 뒤에 붙인다 (두 번 붙지 않는다)
Baro_ai_check_ui() string                 // 본체 편집 화면용 체크 칸
Baro_ai_text() string · Baro_ai_help() string

// 사용자 이름표 (누르면 차림표가 펼쳐진다)
Baro_user_tag(db, user, my_ip string) string

// 프로필 사진
Baro_avatar_has(db, user string) bool
Baro_avatar_url(db, user string) string             // "" 면 올린 사진 없음
Baro_avatar_path(db, user string) string
Baro_avatar_save(db, user string, raw []byte) error // 앞머리로 진짜 그림인지 확인한다
Baro_avatar_delete(db, user string)
Baro_user_avatar_url(db, user string, size int) string  // 사진 없으면 그라바타
Baro_user_avatar_ui(db, user string, size int) string

// 요청 제한 / 지표 / 점검
Baro_rate_ok(db, key, who string, limit, window_sec int) bool
Baro_metric_bump(db, key, period string, delta int)
Baro_metric_get(db, key, period string) int
Baro_read_only(db) bool
Baro_read_only_reason(db) string

// 초기 데이터 (읽기 전용으로 참고)
Baro_badge_default() [][]string    // {키,이름,설명,종류,아이콘,조건,정렬}
Baro_policy_default() [][]string   // {키,제목,본문}
```

### 3-3. UI 부품 (route/tool/baro_ui.go)

```go
Baro_page(db, config, title, body string, menu [][]any) string   // ★ 화면의 마지막 반환값
Baro_css() string                                                 // Baro_page 가 자동 포함
Baro_top_nav(db, config) string                                   // Baro_page 가 자동 포함

Baro_hero(title, sub, body string) string
Baro_card(title, body string) string
Baro_card_ex(title, sub, body, foot string) string
Baro_section(title, body string) string
Baro_grid(items []string, cols int) string          // cols 1~4
Baro_stat(label, value, sub string) string
Baro_stat_row(stats [][]string) string              // {{라벨, 값, 부가설명}, ...}
Baro_progress(done, total int, label string) string
Baro_progress_open(done, goal int, label string) string  // 분모 미확정 분야용
Baro_badge(text, kind string) string                // kind: ok warn danger info muted
Baro_alert(kind, text string) string
Baro_note(text string) string
Baro_empty(text string) string
Baro_table(headers []string, rows [][]string, empty string) string  // 각 칸은 이미 HTML
Baro_dl(pairs [][]string) string                    // 값은 이미 HTML
Baro_list(items []string) string
Baro_pills(items []string) string
Baro_button(url, text, kind string) string          // kind: primary danger ""
Baro_submit(text, kind string) string
Baro_form(action, body string) string
Baro_form_file(action, body string) string
Baro_field_row_html(label, control, help string) string
Baro_input(name, label, value, placeholder, help string) string
Baro_input_type(name, label, value, kind, help string) string  // date datetime-local number ...
Baro_textarea(name, label, value string, rows int, placeholder, help string) string
Baro_select(name, label string, options [][]string, selected, help string) string
Baro_checkbox(name, label, value string, checked bool, help string) string
Baro_hidden(name, value string) string
Baro_tabs(items [][]string, active string) string   // {{주소, 이름}, ...}
Baro_crumb(items [][]string) string                 // 주소가 "" 면 현재 위치
Baro_paging(page, count, max_count int, url string) string  // url 안의 {} 가 쪽 번호로 바뀐다
Baro_doc_link(title string) string
Baro_user_link(db, user, my_ip string) string
Baro_time_cell(data string) string
Baro_status_badge(status string) string
Baro_kv_grid(pairs [][]string) string
Baro_code(text string) string
Baro_danger_zone(title, body string) string
```

CSS 클래스를 새로 만들지 말고 위 부품을 조합한다. 꼭 필요하면 `bw_` 접두사를 쓴 인라인 `<style>` 을 자기 화면 안에서만 추가한다.

---

## 4. 모듈 배정

| 키 | 담당 장 | URL 접두사 | 파일 이름 |
| --- | --- | --- | --- |
| `home` | 1-1 랜딩, 19 화면 지도 | `/baro`, `/baro/founding` | `route/view_baro_home*.go`, `routes_baro_home.go` |
| `founding` | 1-2~1-6 창립단·관심분야·초대·공유카드 | `/baro/founding/...`, `/baro/invite`, `/baro/card` | `route/view_baro_founding*.go` |
| `account` | 2 온보딩·첫기여도우미·프로필·연습장 | `/baro/onboarding`, `/baro/profile`, `/baro/draft` | `route/view_baro_account*.go` |
| `project` | 6 분야허브·개척률·프로젝트·과제·작업실 | `/baro/field`, `/baro/project`, `/baro/task` | `route/view_baro_project*.go` |
| `contrib` | 7 기여이벤트·점수·영향력·랭킹·배지·감사·캘린더 | `/baro/contrib`, `/baro/rank`, `/baro/badge`, `/baro/thanks` | `route/view_baro_contrib*.go` |
| `quality` | 9 문서상태·검토큐·안정판·출처품질·중복·편집필터 | `/baro/quality`, `/baro/review`, `/baro/source` | `route/view_baro_quality*.go` |
| `mod` | 10 신고·사건·제재·이의제기·대량조치 | `/baro/report`, `/baro/case`, `/baro/appeal`, `/baro/mass` | `route/view_baro_mod*.go` |
| `gov` | 11 공개로그·정책버전·역할분리·이해충돌·투명성 | `/baro/log`, `/baro/policy`, `/baro/role`, `/baro/transparency` | `route/view_baro_gov*.go` |
| `award` | 12 MVP·올해의위키인·선정절차·Wrapped | `/baro/award`, `/baro/wrapped` | `route/view_baro_award*.go` |
| `imp` | 13 문서가져오기·라이선스기록·URL매핑·봇 | `/baro/import` | `route/view_baro_imp*.go` |
| `data` | 14 내보내기·덤프·백업·공개API | `/baro/export`, `/baro/backup`, `/baro/api` | `route/view_baro_data*.go` |
| `legal` | 15 개인정보·저작권·동의·법적요청 | `/baro/privacy`, `/baro/copyright`, `/baro/legal` | `route/view_baro_legal*.go` |
| `admin` | 16 백오피스 전체 | `/baro/admin/...` | `route/view_baro_admin*.go` |
| `sec` | 17 2FA·악용방지·감사로그·장애대응·성능·접근성 | `/baro/security`, `/baro/status` | `route/view_baro_sec*.go` |
| `metric` | 18 분석 지표 | `/baro/metrics` | `route/view_baro_metric*.go` |
| `find` | 5 검색확장·결과없음·발견·SEO / 3-15 동명이인·별칭 / 4 파일 | `/baro/search`, `/baro/discover`, `/baro/doc`, `/baro/file` | `route/view_baro_find*.go` |
| `req` | 편집 요청 (생성 · 승인 · 거절 · 삭제 기록) | `/baro/req` | `route/view_baro_req.go` |
| `ceremony` | 위키 수상식 (개최 · 출마 · 투표 · 결과) | `/baro/ceremony` | `route/view_baro_ceremony.go` |
| `record` | 통합 기여 기록 (편집 · 토론 · 편집 요청) | `/baro/record` | `route/view_baro_record.go` |
| `mik` | 밐(가상 화폐) · 바로샵 · 밐 순위 · 프로필 사진 · 바로 캡차 | `/baro/mik`, `/baro/shop`, `/baro/avatar`, `/captcha` | `route/view_baro_mik*.go`, `route/view_baro_account_avatar.go`, `route/tool/baro_captcha*.go` |
| `theme` | 사용자 지정 테마 (만들기 · 공개 · 공식 채택) | `/baro/theme` | `route/view_baro_theme*.go`, `route/tool/baro_theme*.go` |
| `coop` | 편집방 (공동 편집 · 참가 · 댓글 · 금칙어) | `/baro/cooperation` | `route/view_baro_coop*.go`, `route/tool/baro_coop*.go` |

---

## 5. 표 정의 (전부 text 컬럼)

모든 표에는 openNAMU 관례상 쓰지 않는 `test` 컬럼이 하나 더 붙어 있다. 무시한다.
정수·불리언도 전부 문자열로 저장한다. (`tool.Str_to_int` / `"on"` / `""`)

#### 설정 / 운영 기반

| 표 | 컬럼 |
| --- | --- |
| `baro_set` | `name` `data` |
| `baro_flag` | `key_name` `name` `scope` `value` `note` `date` |
| `baro_metric` | `id` `key_name` `period` `value` `detail` `date` |
| `baro_rate` | `key_name` `who` `num` `window_start` |
| `baro_incident` | `id` `title` `status` `severity` `detail` `date` `resolve_date` `postmortem` `actor` |

#### 사용자 / 창립 시즌

| 표 | 컬럼 |
| --- | --- |
| `baro_user_set` | `user` `name` `data` |
| `baro_role` | `user` `role` `granted_by` `date` `memo` |
| `baro_interest` | `user` `field` `kind` `date` |
| `baro_expertise` | `user` `field` `score` `is_main` `date` |
| `baro_founder` | `user` `number` `date` `status` `first_event` `field` `apply_date` `note` |
| `baro_invite` | `id` `code` `owner` `target` `date` `used_date` `status` |
| `baro_consent` | `id` `user` `kind` `version` `date` `ip` |
| `baro_onboard` | `user` `step` `status` `date` |
| `baro_2fa` | `user` `secret` `backup` `status` `date` |
| `baro_login_log` | `id` `user` `ip` `ua` `date` `kind` `detail` |
| `baro_share_card` | `id` `user` `kind` `title` `data` `date` `views` |

#### 문서 부가 정보

| 표 | 컬럼 |
| --- | --- |
| `baro_doc_status` | `title` `status` `reason` `actor` `date` |
| `baro_doc_alias` | `id` `title` `alias` `kind` `note` `date` |
| `baro_doc_meta` | `title` `name` `data` |
| `baro_citation` | `id` `title` `rev` `kind` `url` `author` `publisher` `pub_date` `access_date` `archive_url` `quality` `status` `date` |
| `baro_quarantine` | `id` `title` `rev` `kind` `reason` `actor` `date` `status` `backup` `release_date` `case_id` |
| `baro_stable` | `title` `rev` `actor` `date` `note` |
| `baro_review` | `id` `title` `rev` `user` `kind` `status` `reviewer` `review_date` `comment` `risk` `date` |
| `baro_dup` | `id` `title` `target` `kind` `ratio` `date` `status` `detail` |
| `baro_file_meta` | `title` `source` `author` `photographer` `license` `basis` `shot_date` `description` `related` `hash` `exif_removed` `date` `uploader` |

#### 프로젝트 / 분야 개척

| 표 | 컬럼 |
| --- | --- |
| `baro_field` | `key_name` `name` `description` `sort` `target_count` `recruit_count` `recruit_open` `kind` `date` |
| `baro_project` | `id` `name` `field` `goal` `scope` `dataset` `start_date` `end_date` `owner` `status` `target_count` `done_count` `rule` `finish_cond` `date` `summary` `report` |
| `baro_project_member` | `id` `project` `user` `role` `date` `status` |
| `baro_project_target` | `id` `project` `name` `note` `status` `doc` `date` `assignee` |
| `baro_project_task` | `id` `project` `kind` `title` `detail` `difficulty` `est_time` `assignee` `status` `doc` `date` `done_date` `requester` `field` |
| `baro_project_progress` | `id` `project` `date` `done` `total` `note` |
| `baro_project_source` | `id` `project` `title` `url` `note` `user` `date` |
| `baro_workspace` | `id` `project` `title` `data` `user` `date` `status` `note` |

#### 기여 인정

| 표 | 컬럼 |
| --- | --- |
| `baro_contrib_event` | `id` `user` `kind` `title` `rev` `project` `weight` `detail` `date` `status` `field` |
| `baro_contrib_score` | `id` `user` `event` `base` `source` `novelty` `review` `survive` `penalty` `final` `confirmed` `confirm_date` `date` |
| `baro_impact` | `user` `name` `data` `date` |
| `baro_badge` | `key_name` `name` `description` `kind` `icon` `criteria` `sort` |
| `baro_user_badge` | `id` `user` `badge` `date` `detail` |
| `baro_rank` | `id` `period` `kind` `field` `user` `score` `rank_no` `date` |
| `baro_thanks` | `id` `from_user` `to_user` `title` `rev` `reason` `is_public` `date` |
| `baro_calendar` | `user` `date` `num` `detail` |

#### 커뮤니티

| 표 | 컬럼 |
| --- | --- |
| `baro_watch` | `id` `user` `kind` `target` `date` `mode` |
| `baro_notify` | `id` `user` `kind` `title` `url` `data` `readme` `date` |
| `baro_notify_set` | `user` `kind` `mode` |
| `baro_mentor` | `id` `mentor` `mentee` `status` `date` `field` `note` |

#### 신고 / 제재 / 이의제기

| 표 | 컬럼 |
| --- | --- |
| `baro_report` | `id` `kind` `target_kind` `target` `reporter` `detail` `evidence` `status` `case_id` `date` `priority` `note` |
| `baro_case` | `id` `number` `kind` `target_kind` `target` `rule` `actor` `status` `action` `public_summary` `private_note` `date` `close_date` `deadline` `appeal_status` `priority` |
| `baro_case_action` | `id` `case_id` `actor` `action` `detail` `date` `target` |
| `baro_case_evidence` | `id` `case_id` `kind` `data` `actor` `date` `is_private` |
| `baro_sanction` | `id` `user` `kind` `scope` `reason` `case_id` `actor` `start_date` `end_date` `status` `date` |
| `baro_appeal` | `id` `case_id` `user` `reason` `extra` `status` `reviewer` `result` `result_reason` `date` `deadline` `close_date` |
| `baro_mass_op` | `id` `kind` `actor` `target_list` `num` `reason` `status` `approver` `created` `approved_date` `execute_date` `grace_until` `urgent` `undo_status` `detail` |
| `baro_snapshot` | `id` `mass_id` `kind` `title` `data` `meta` `date` |

#### 거버넌스 / 투명성

| 표 | 컬럼 |
| --- | --- |
| `baro_public_log` | `id` `kind` `actor` `target` `reason` `detail` `date` |
| `baro_audit_log` | `id` `actor` `action` `target` `before_data` `after_data` `auth` `reason` `ip` `ua` `date` `sign_data` |
| `baro_policy` | `id` `key_name` `title` `version` `data` `effective_date` `proposer` `reason` `discussion` `date` `status` |
| `baro_coi` | `id` `case_id` `actor` `reason` `date` |
| `baro_admin_stat` | `user` `name` `data` `period` `date` |
| `baro_transparency` | `id` `period` `data` `date` `publisher` |

#### 어워드

| 표 | 컬럼 |
| --- | --- |
| `baro_award` | `id` `season` `category` `name` `description` `status` `date` `result_reason` `winner` |
| `baro_nomination` | `id` `award` `target_kind` `target` `nominator` `kind` `score` `status` `date` `reason` |
| `baro_award_vote` | `id` `award` `nomination` `voter` `weight` `date` |
| `baro_wrapped` | `id` `user` `year_data` `data` `date` |

#### 이전 / 데이터 개방

| 표 | 컬럼 |
| --- | --- |
| `baro_import` | `id` `kind` `source` `file` `status` `actor` `date` `report` `num` `license` `options` |
| `baro_import_item` | `id` `import_id` `title` `status` `note` `orig_author` `orig_url` `license` `date` |
| `baro_url_map` | `id` `old_url` `new_url` `kind` `date` |
| `baro_export` | `id` `user` `kind` `scope` `status` `path` `date` `expire` |
| `baro_backup` | `id` `kind` `path` `size` `status` `date` `note` `checksum` |
| `baro_api_key` | `id` `user` `key_hash` `name` `scope` `rate` `status` `date` `last_used` |

#### 개인정보 / 저작권 / 법무

| 표 | 컬럼 |
| --- | --- |
| `baro_privacy_req` | `id` `user` `kind` `detail` `status` `handler` `date` `close_date` `verify` `result` |
| `baro_copyright_req` | `id` `claimant` `contact` `target_kind` `target` `proof` `status` `handler` `date` `close_date` `result` `counter` |
| `baro_legal_log` | `id` `kind` `num` `period` `date` `note` |
| `baro_deletion_req` | `id` `user` `kind` `reason` `status` `date` `close_date` `handler` `result` |

#### 편집 필터 (바로위키 확장)

| 표 | 컬럼 |
| --- | --- |
| `baro_edit_filter` | `id` `name` `pattern` `kind` `action` `status` `note` `date` `hits` |
| `baro_filter_log` | `id` `filter_id` `user` `title` `action` `detail` `date` |
| `baro_edit_req` | `id` `title` `base_rev` `user` `data` `summary` `status` `handler` `handle_date` `reason` `date` `new_rev` `kind` `target` `locked` |
| `baro_edit_req_log` | `id` `req_id` `title` `actor` `action` `detail` `date` |

#### 고유 번호

| 표 | 컬럼 |
| --- | --- |
| `baro_rev_uuid` | `uuid` `title` `rev` `date` |
| `baro_user_name_log` | `id` `uuid` `user` `old_name` `new_name` `actor` `date` |

#### 밐 (가상 화폐) / 바로샵

| 표 | 컬럼 |
| --- | --- |
| `baro_mik_log` | `id` `user` `delta` `balance` `kind` `reason` `actor` `target` `date` |
| `baro_shop_item` | `id` `name` `description` `kind` `price` `data` `status` `stock` `sort` `actor` `date` |
| `baro_shop_buy` | `id` `item` `user` `name` `kind` `price` `data` `status` `date` `end_date` |
| `baro_captcha` | `id` `code` `answer` `who` `status` `date` |

#### 사용자 지정 테마

| 표 | 컬럼 |
| --- | --- |
| `baro_theme` | `id` `uuid` `title` `description` `maker` `data` `status` `official` `date` `edit_date` `views` `note` |

#### 편집방 (공동 편집)

| 표 | 컬럼 |
| --- | --- |
| `baro_coop_room` | `id` `link` `code` `name` `description` `owner` `open_kind` `max_user` `status` `close_at` `notice` `banned` `date` `close_date` `note` `acl` `seal_by` `seal_reason` `seal_date` |
| `baro_coop_member` | `id` `room` `user` `role` `status` `block` `date` `out_date` `note` |
| `baro_coop_doc` | `id` `room` `title` `num` `date` `status` `opener` `open_date` |
| `baro_coop_log` | `id` `room` `kind` `user` `data` `detail` `date` `status` |

`baro_coop_room.status` : `open` · `closing`(10분 뒤 닫힘) · `closed`(방장이 다시 연다) · `sealed`(영구 폐쇄, 관리자만 푼다).
`baro_coop_member.status` : `in` · `out`(나감) · `invited`(불렸으나 아직 안 들어옴).
`baro_coop_doc.status` : `open` 이면 작업 공간에 탭이 열려 있다. `num` 은 그 방에서 손댄 횟수다.
`baro_coop_log.kind` : `comment` 댓글 · `notice` 공지 · `join` 참가 · `leave` 나감 · `invite` 초대 · `edit` 편집 · `system` 방 자체의 일.
나간 사람은 `baro_coop_member.status = 'out'` 으로 남는다. 지우지 않는다. (2번 원칙)

`data` 는 토큰 이름 → `"밝은쪽|어두운쪽"` 을 담은 JSON 이다.
쓰고 있는 테마는 `baro_user_set` 의 `name = 'theme'` 에 고유 번호로 둔다.

잔액 자체는 표를 따로 두지 않고 `baro_user_set` 에 담는다.
(`mik` 잔액 · `mik_attend` 마지막 접속 보상 날짜 · `mik_attend_streak` 연속 일수 ·
`mik_title` 달고 있는 칭호 · `mik_captcha_until` 캡차 면제 만료 · `mik_captcha_once` 1회 면제권 장수 ·
`mik_max` 최대 보유 · `mik_total` 누적 획득 · `avatar` 프로필 사진 확장자|판)
직접 더하고 빼지 말고 아래 함수만 쓴다.

계정 고유 번호는 표를 따로 두지 않고 `baro_user_set` 의 `name = 'uuid'` 에 담는다.
번호를 직접 만들거나 `baro_rev_uuid` 를 직접 뒤지지 말고 아래 함수만 쓴다. (3-2 참고)
| `baro_ceremony` | `id` `name` `doc` `opener` `status` `run_end` `vote_end` `note` `date` `close_date` `result` |
| `baro_ceremony_cat` | `id` `ceremony` `name` `description` `sort` `winner` |
| `baro_ceremony_run` | `id` `ceremony` `cat` `target_kind` `target` `runner` `reason` `status` `date` |
| `baro_ceremony_vote` | `id` `ceremony` `cat` `run_id` `voter` `date` |


### 자주 쓰는 값 규칙

- 상태 컬럼(`status`) : `open` `pending` `active` `closed` `done` `rejected` `expired` `confirmed` `new` 중에서 고른다.
- 불리언 : `"on"` 이면 참, `""` 이면 거짓.
- 날짜 : `Baro_now()` (초 단위) 또는 `Baro_today()` (날짜만).
- 목록/구조체 : `Baro_json_enc` 로 JSON 문자열로 저장.
- `id` 는 `Baro_next_id(db, "표이름")` 로 만든다. 숫자 문자열이므로 정렬은 `order by id + 0 desc`.

---

## 6. 완료 기준

1. `gofmt -l <내가 만든 파일들>` 이 아무것도 출력하지 않는다.
2. `go build ./...` 가 성공한다.
3. `go vet ./...` 에 내 파일 관련 오류가 없다.
4. 서버를 띄워 내 화면 주소들이 200 또는 302 를 돌려준다. (권한이 필요한 화면은 로그인 없이 접근 시
   권한 안내 화면이 200 으로 떠야 한다 — 500 이 나오면 안 된다)
5. 배정받은 장의 **P0 · P1 · P2 항목을 빠짐없이** 구현했다.
