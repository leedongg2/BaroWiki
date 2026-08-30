package route

import (
	"database/sql"
	"sort"
	"strconv"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  12장 어워드 · 연말 결산 — 공통 계층
//
//  이 파일은 어워드 모듈 안에서만 쓰는 자료 구조와 계산기를 모아 둔다.
//  화면은 view_baro_award_list.go / _mvp.go / _process.go / _wrapped.go 에 있다.
//
//  0장 5번 원칙(근거 공개)에 따라 모든 후보 옆에는 계산 근거가 되는
//  기여 수치를 함께 보여주고, 최종 점수 공식도 화면에 그대로 밝힌다.
// ============================================================

// baro_award_cols : baro_award 표의 컬럼 순서
const baro_award_cols = "id, season, category, name, description, status, date, result_reason, winner"

// baro_award_nom_cols : baro_nomination 표의 컬럼 순서
const baro_award_nom_cols = "id, award, target_kind, target, nominator, kind, score, status, date, reason"

// baro_award_row : 어워드 부문 한 줄
type baro_award_row struct {
	ID           string
	Season       string
	Category     string
	Name         string
	Description  string
	Status       string
	Date         string
	ResultReason string
	Winner       string
}

// baro_award_nom_row : 후보 한 줄
type baro_award_nom_row struct {
	ID         string
	Award      string
	TargetKind string
	Target     string
	Nominator  string
	Kind       string
	Score      string
	Status     string
	Date       string
	Reason     string
}

// baro_award_cand : 계산으로 뽑아낸 후보 (아직 등록 전)
type baro_award_cand struct {
	Kind   string // user / document / project
	Target string
	Value  int
	Note   string // 근거 수치 (이미 HTML)
}

// baro_award_agg : 한 사람의 기간 내 기여 집계
type baro_award_agg struct {
	User       string
	Events     int
	Score      int
	ProjectNum int
	Kinds      map[string]int
	FieldScore map[string]int
	FieldNum   map[string]int
	Docs       map[string]bool
	First      string
}

// ------------------------------------------------------------
//  부문 정의
// ------------------------------------------------------------

// baro_award_year_category_list : 올해의 바로위키인 부문 (키, 이름, 대상 종류, 설명)
func baro_award_year_category_list() [][]string {
	return [][]string{
		{"year_wikian", tool.Baro_t("올해의 위키인"), "user", tool.Baro_t("올해 가장 넓고 깊게 기여한 사람")},
		{"year_rookie", tool.Baro_t("올해의 신인"), "user", tool.Baro_t("올해 처음 기여를 시작해 두각을 나타낸 사람")},
		{"year_history", tool.Baro_t("올해의 역사 기여자"), "user", tool.Baro_t("역사 분야 기록을 가장 크게 넓힌 사람")},
		{"year_science", tool.Baro_t("올해의 과학 기여자"), "user", tool.Baro_t("과학 분야 기록을 가장 크게 넓힌 사람")},
		{"year_photo", tool.Baro_t("올해의 사진 기여자"), "user", tool.Baro_t("직접 올린 파일로 문서를 채운 사람")},
		{"year_reviewer", tool.Baro_t("올해의 검토자"), "user", tool.Baro_t("검토 큐를 묵묵히 처리한 사람")},
		{"year_document", tool.Baro_t("올해의 문서"), "document", tool.Baro_t("올해 새로 만들어지거나 크게 좋아진 문서")},
		{"year_project", tool.Baro_t("올해의 프로젝트"), "project", tool.Baro_t("올해 뚜렷한 성과를 낸 프로젝트")},
		{"year_hidden", tool.Baro_t("올해의 숨은 기록"), "document", tool.Baro_t("주목받지 못했지만 출처가 탄탄한 조용한 기록")},
	}
}

// baro_award_mvp_category_list : 월간 MVP 부문 (키, 이름, 대상 종류, 설명)
//
//	분야 부문은 baro_field 에 등록된 분야에서 그때그때 만들어 낸다.
func baro_award_mvp_category_list(db *sql.DB) [][]string {
	out := [][]string{}

	for _, field := range tool.Baro_field_list(db) {
		out = append(out, []string{
			"mvp_field_" + field.Key,
			field.Name + " 분야",
			"user",
			field.Name + " 분야에 이번 달 가장 크게 기여한 사람",
		})
	}

	out = append(out,
		[]string{"mvp_newbie", tool.Baro_t("신규 기여자"), "user", tool.Baro_t("이번 달 처음 기여를 시작한 사람 중 가장 활발했던 사람")},
		[]string{"mvp_review", tool.Baro_t("문서 검토"), "user", tool.Baro_t("이번 달 검토를 가장 많이 처리한 사람")},
		[]string{"mvp_source", tool.Baro_t("출처 기여"), "user", tool.Baro_t("이번 달 출처를 가장 많이 붙인 사람")},
		[]string{"mvp_project", "프로젝트", "user", tool.Baro_t("이번 달 프로젝트 안에서 가장 많이 움직인 사람")},
	)

	return out
}

// baro_award_category_name : 부문 표시 이름
func baro_award_category_name(db *sql.DB, category string) string {
	if strings.HasPrefix(category, "mvp_field_") {
		return tool.Baro_field_name(db, strings.TrimPrefix(category, "mvp_field_")) + " 분야"
	}

	for _, row := range baro_award_year_category_list() {
		if row[0] == category {
			return row[1]
		}
	}

	for _, row := range baro_award_mvp_category_list(db) {
		if row[0] == category {
			return row[1]
		}
	}

	return category
}

// baro_award_category_desc : 부문 설명
func baro_award_category_desc(db *sql.DB, category string) string {
	if strings.HasPrefix(category, "mvp_field_") {
		return tool.Baro_field_name(db, strings.TrimPrefix(category, "mvp_field_")) + " 분야에 이번 달 가장 크게 기여한 사람"
	}

	for _, row := range baro_award_year_category_list() {
		if row[0] == category {
			return row[3]
		}
	}

	for _, row := range baro_award_mvp_category_list(db) {
		if row[0] == category {
			return row[3]
		}
	}

	return ""
}

// baro_award_category_kind : 부문의 대상 종류 (user / document / project)
func baro_award_category_kind(category string) string {
	switch category {
	case "year_document", "year_hidden":
		return "document"
	case "year_project":
		return "project"
	}

	return "user"
}

// baro_award_is_mvp : 월간 MVP 부문인지
func baro_award_is_mvp(category string) bool {
	return strings.HasPrefix(category, "mvp_")
}

// baro_award_target_kind_name : 대상 종류 표시 이름
func baro_award_target_kind_name(kind string) string {
	switch kind {
	case "user":
		return tool.Baro_t("사용자")
	case "document":
		return "문서"
	case "project":
		return "프로젝트"
	}

	return kind
}

// baro_award_nom_kind_name : 후보 등록 경로 이름
func baro_award_nom_kind_name(kind string) string {
	switch kind {
	case "auto":
		return tool.Baro_t("자동 자격")
	case "user":
		return tool.Baro_t("이용자 추천")
	case "mvp":
		return tool.Baro_t("월간 집계")
	}

	return kind
}

// baro_award_status_name : 시즌 진행 상태 이름
func baro_award_status_name(status string) string {
	switch status {
	case "open":
		return tool.Baro_t("후보 모집 중")
	case "pending":
		return tool.Baro_t("심사·투표 중")
	case "closed":
		return tool.Baro_t("종료 (확정)")
	}

	return status
}

// baro_award_status_badge : 시즌 진행 상태 배지
func baro_award_status_badge(status string) string {
	switch status {
	case "open":
		return tool.Baro_badge(baro_award_status_name(status), "info")
	case "pending":
		return tool.Baro_badge(baro_award_status_name(status), "warn")
	case "closed":
		return tool.Baro_badge(baro_award_status_name(status), "ok")
	}

	return tool.Baro_badge(status, "muted")
}

// baro_award_locked : 결과가 확정되어 더 이상 고칠 수 없는 시즌인가
//
//	0장 5번 원칙 : 확정된 결과는 근거와 함께 영구 보존한다.
func baro_award_locked(row baro_award_row) bool {
	return row.Status == "closed"
}

// baro_award_lock_notice : 잠금 안내
func baro_award_lock_notice() string {
	return tool.Baro_alert("ok", tool.Baro_t("<b>잠김 · 확정된 시즌입니다.</b> 결과는 영구 보존되며 후보 추가·심사·투표·결과 수정이 모두 잠겨 있습니다."))
}

// ------------------------------------------------------------
//  표 읽기
// ------------------------------------------------------------

// baro_award_query : baro_award 여러 줄 읽기 (query 는 코드에서만 만든다)
func baro_award_query(db *sql.DB, query string, args ...any) []baro_award_row {
	out := []baro_award_row{}

	rows := tool.Query_DB(db, query, args...)
	defer rows.Close()

	for rows.Next() {
		row := baro_award_row{}
		if err := rows.Scan(
			&row.ID, &row.Season, &row.Category, &row.Name, &row.Description,
			&row.Status, &row.Date, &row.ResultReason, &row.Winner,
		); err != nil {
			continue
		}

		out = append(out, row)
	}

	return out
}

// baro_award_get : 부문 하나 읽기
func baro_award_get(db *sql.DB, id string) (baro_award_row, bool) {
	row := baro_award_row{}
	found := tool.QueryRow_DB(
		db,
		"select "+baro_award_cols+" from baro_award where id = ?",
		[]any{
			&row.ID, &row.Season, &row.Category, &row.Name, &row.Description,
			&row.Status, &row.Date, &row.ResultReason, &row.Winner,
		},
		id,
	)

	return row, found
}

// baro_award_find : 시즌+부문으로 찾기
func baro_award_find(db *sql.DB, season string, category string) (baro_award_row, bool) {
	row := baro_award_row{}
	found := tool.QueryRow_DB(
		db,
		"select "+baro_award_cols+" from baro_award where season = ? and category = ?",
		[]any{
			&row.ID, &row.Season, &row.Category, &row.Name, &row.Description,
			&row.Status, &row.Date, &row.ResultReason, &row.Winner,
		},
		season, category,
	)

	return row, found
}

// baro_award_season_rows : 한 시즌의 부문 전체
func baro_award_season_rows(db *sql.DB, season string) []baro_award_row {
	return baro_award_query(
		db,
		"select "+baro_award_cols+" from baro_award where season = ? order by id + 0 asc limit 200",
		season,
	)
}

// baro_award_season_list : 시즌 목록 (최근 순)
func baro_award_season_list(db *sql.DB, offset int, limit int) []string {
	out := []string{}

	rows := tool.Query_DB(
		db,
		"select season from baro_award where season != '' group by season order by season desc limit "+
			strconv.Itoa(limit)+" offset "+strconv.Itoa(offset),
	)
	defer rows.Close()

	for rows.Next() {
		season := ""
		if err := rows.Scan(&season); err != nil {
			continue
		}

		out = append(out, season)
	}

	return out
}

// baro_award_nom_query : baro_nomination 여러 줄 읽기
func baro_award_nom_query(db *sql.DB, query string, args ...any) []baro_award_nom_row {
	out := []baro_award_nom_row{}

	rows := tool.Query_DB(db, query, args...)
	defer rows.Close()

	for rows.Next() {
		row := baro_award_nom_row{}
		if err := rows.Scan(
			&row.ID, &row.Award, &row.TargetKind, &row.Target, &row.Nominator,
			&row.Kind, &row.Score, &row.Status, &row.Date, &row.Reason,
		); err != nil {
			continue
		}

		out = append(out, row)
	}

	return out
}

// baro_award_nom_list : 한 부문의 후보 목록 (쪽 나눔)
func baro_award_nom_list(db *sql.DB, award string, offset int, limit int) []baro_award_nom_row {
	return baro_award_nom_query(
		db,
		"select "+baro_award_nom_cols+" from baro_nomination where award = ? order by id + 0 asc limit "+
			strconv.Itoa(limit)+" offset "+strconv.Itoa(offset),
		award,
	)
}

// baro_award_nom_all : 한 부문의 후보 전체 (계산용, 상한 있음)
func baro_award_nom_all(db *sql.DB, award string) []baro_award_nom_row {
	return baro_award_nom_query(
		db,
		"select "+baro_award_nom_cols+" from baro_nomination where award = ? order by id + 0 asc limit 500",
		award,
	)
}

// baro_award_nom_get : 후보 하나
func baro_award_nom_get(db *sql.DB, id string) (baro_award_nom_row, bool) {
	row := baro_award_nom_row{}
	found := tool.QueryRow_DB(
		db,
		"select "+baro_award_nom_cols+" from baro_nomination where id = ?",
		[]any{
			&row.ID, &row.Award, &row.TargetKind, &row.Target, &row.Nominator,
			&row.Kind, &row.Score, &row.Status, &row.Date, &row.Reason,
		},
		id,
	)

	return row, found
}

// baro_award_nom_count : 후보 수
func baro_award_nom_count(db *sql.DB, award string) int {
	count := 0
	tool.QueryRow_DB(db, "select count(*) from baro_nomination where award = ?", []any{&count}, award)

	return count
}

// baro_award_nom_count_kind : 등록 경로별 후보 수
func baro_award_nom_count_kind(db *sql.DB, award string, kind string) int {
	count := 0
	tool.QueryRow_DB(db, "select count(*) from baro_nomination where award = ? and kind = ?", []any{&count}, award, kind)

	return count
}

// baro_award_nom_exists : 같은 대상이 이미 후보로 올라와 있는지
func baro_award_nom_exists(db *sql.DB, award string, kind string, target string) bool {
	found := ""
	tool.QueryRow_DB(
		db,
		"select id from baro_nomination where award = ? and target_kind = ? and target = ?",
		[]any{&found}, award, kind, target,
	)

	return found != ""
}

// baro_award_target_exists : 실제로 있는 대상인지 확인
//
//	없는 계정·문서·프로젝트가 후보로 올라가면 근거 없는 시상이 되므로 미리 막는다.
func baro_award_target_exists(db *sql.DB, kind string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}

	found := ""

	switch kind {
	case "user":
		tool.QueryRow_DB(db, "select id from user_set where name = 'pw' and id = ?", []any{&found}, target)
	case "document":
		tool.QueryRow_DB(db, "select title from data where title = ?", []any{&found}, target)
	case "project":
		tool.QueryRow_DB(db, "select id from baro_project where id = ?", []any{&found}, target)
	default:
		return false
	}

	return found != ""
}

// baro_award_target_miss : 실재하지 않는 대상에 대한 안내 문구
func baro_award_target_miss(kind string) string {
	switch kind {
	case "user":
		return tool.Baro_t("그런 사용자를 찾을 수 없습니다. 가입한 계정 이름을 정확히 적어 주세요.")
	case "document":
		return tool.Baro_t("그런 문서를 찾을 수 없습니다. 문서 제목을 그대로 적어 주세요.")
	case "project":
		return tool.Baro_t("그런 프로젝트를 찾을 수 없습니다. 프로젝트 번호를 다시 확인해 주세요.")
	}

	return tool.Baro_t("그런 대상을 찾을 수 없습니다.")
}

// baro_award_agg_for : 사람 부문일 때만 기간 집계를 미리 모아 둔다
//
//	사람 부문이 아니면 nil 을 돌려주어, 근거 수치를 후보마다 따로 계산하게 한다.
//	(빈 map 을 넘기면 사람 후보의 근거가 "기록 없음" 으로 잘못 보인다)
func baro_award_agg_for(db *sql.DB, category string, period string) map[string]*baro_award_agg {
	if baro_award_category_kind(category) != "user" {
		return nil
	}

	return baro_award_collect(db, period)
}

// baro_award_nom_add : 후보 등록 (이미 있으면 아무 것도 하지 않는다)
//
//	후보로 등록되면 대상에게 알림을 보낸다 (12-3).
func baro_award_nom_add(db *sql.DB, award baro_award_row, kind string, target string, nominator string, source string, reason string) string {
	target = strings.TrimSpace(target)
	if target == "" || baro_award_nom_exists(db, award.ID, kind, target) {
		return ""
	}

	id := tool.Baro_next_id(db, "baro_nomination")
	tool.Exec_DB(
		db,
		"insert into baro_nomination (id, award, target_kind, target, nominator, kind, score, status, date, reason) values (?, ?, ?, ?, ?, ?, '0', 'open', ?, ?)",
		id, award.ID, kind, target, nominator, source, tool.Baro_now(), tool.Baro_trunc(reason, 1000),
	)

	if kind == "user" {
		tool.Baro_notify(
			db, target, "award",
			award.Season+" "+award.Name+tool.Baro_t(" 후보로 등록됐습니다."),
			"/baro/award/view/"+tool.Url_parser(award.ID),
			"",
		)
	}

	return id
}

// ------------------------------------------------------------
//  기여 데이터 집계 (12-3 3단계 : 후보 옆의 근거 수치)
// ------------------------------------------------------------

// baro_award_agg_new : 빈 집계 만들기
func baro_award_agg_new(user string) *baro_award_agg {
	return &baro_award_agg{
		User:       user,
		Kinds:      map[string]int{},
		FieldScore: map[string]int{},
		FieldNum:   map[string]int{},
		Docs:       map[string]bool{},
	}
}

// baro_award_agg_put : 기여 이벤트 한 건을 집계에 더한다
func baro_award_agg_put(agg *baro_award_agg, kind string, field string, title string, project string, date string, weight string, final string) {
	score := tool.Str_to_int(final)
	if strings.TrimSpace(final) == "" {
		score = tool.Str_to_int(weight)
		if score == 0 {
			score = tool.Baro_contrib_kind_weight(kind)
		}
	}

	agg.Events++
	agg.Score += score
	agg.Kinds[kind]++

	if field != "" {
		agg.FieldScore[field] += score
		agg.FieldNum[field]++
	}
	if title != "" {
		agg.Docs[title] = true
	}
	if project != "" {
		agg.ProjectNum++
	}
	if agg.First == "" || (date != "" && date < agg.First) {
		agg.First = date
	}
}

// baro_award_collect : 기간(날짜 앞부분) 안의 모든 기여를 사람별로 집계
//
//	period 는 "2026" 또는 "2026-08" 처럼 날짜 앞부분을 넣는다.
//	0장 4번 원칙에 따라 횟수만 세지 않고 행동 종류와 확정 점수를 함께 모은다.
func baro_award_collect(db *sql.DB, period string) map[string]*baro_award_agg {
	out := map[string]*baro_award_agg{}
	if strings.TrimSpace(period) == "" {
		return out
	}

	rows := tool.Query_DB(
		db,
		"select e.user, e.kind, e.field, e.title, e.project, e.date, e.weight, coalesce(s.final, '') "+
			"from baro_contrib_event e left join baro_contrib_score s on s.event = e.id "+
			"where e.date like ? and e.user != '' and e.status != 'rejected' order by e.id + 0 limit 50000",
		period+"%",
	)
	defer rows.Close()

	for rows.Next() {
		user, kind, field, title, project, date, weight, final := "", "", "", "", "", "", "", ""
		if err := rows.Scan(&user, &kind, &field, &title, &project, &date, &weight, &final); err != nil {
			continue
		}

		agg, ok := out[user]
		if !ok {
			agg = baro_award_agg_new(user)
			out[user] = agg
		}

		baro_award_agg_put(agg, kind, field, title, project, date, weight, final)
	}

	return out
}

// baro_award_user_agg : 한 사람의 기간 집계 (후보 상세용)
func baro_award_user_agg(db *sql.DB, user string, period string) *baro_award_agg {
	agg := baro_award_agg_new(user)
	if user == "" || period == "" {
		return agg
	}

	rows := tool.Query_DB(
		db,
		"select e.kind, e.field, e.title, e.project, e.date, e.weight, coalesce(s.final, '') "+
			"from baro_contrib_event e left join baro_contrib_score s on s.event = e.id "+
			"where e.user = ? and e.date like ? and e.status != 'rejected' order by e.id + 0 limit 20000",
		user, period+"%",
	)
	defer rows.Close()

	for rows.Next() {
		kind, field, title, project, date, weight, final := "", "", "", "", "", "", ""
		if err := rows.Scan(&kind, &field, &title, &project, &date, &weight, &final); err != nil {
			continue
		}

		baro_award_agg_put(agg, kind, field, title, project, date, weight, final)
	}

	return agg
}

// baro_award_first_map : 사람별 최초 기여 시각 (신인 부문 판정용)
func baro_award_first_map(db *sql.DB) map[string]string {
	out := map[string]string{}

	rows := tool.Query_DB(db, "select user, min(date) from baro_contrib_event where user != '' group by user")
	defer rows.Close()

	for rows.Next() {
		user, first := "", ""
		if err := rows.Scan(&user, &first); err != nil {
			continue
		}

		out[user] = first
	}

	return out
}

// baro_award_review_map : 기간 안의 검토 처리 수 (검토자 부문 보조 근거)
func baro_award_review_map(db *sql.DB, period string) map[string]int {
	out := map[string]int{}

	rows := tool.Query_DB(
		db,
		"select reviewer, count(*) from baro_review where reviewer != '' and review_date like ? group by reviewer",
		period+"%",
	)
	defer rows.Close()

	for rows.Next() {
		user, count := "", 0
		if err := rows.Scan(&user, &count); err != nil {
			continue
		}

		out[user] = count
	}

	return out
}

// baro_award_agg_note : 사람 후보의 근거 수치 한 줄 (이미 HTML)
func baro_award_agg_note(agg *baro_award_agg) string {
	if agg == nil || agg.Events == 0 {
		return tool.Baro_t(`<span class="bw_muted">기간 안의 기여 기록 없음</span>`)
	}

	parts := []string{
		tool.Baro_t("기여 ") + tool.Baro_comma(agg.Events) + tool.Baro_t("건"),
		tool.Baro_t("점수 ") + tool.Baro_comma(agg.Score) + tool.Baro_t("점"),
		"문서 " + tool.Baro_comma(len(agg.Docs)) + tool.Baro_t("개"),
	}

	if agg.Kinds["new_document"] > 0 {
		parts = append(parts, tool.Baro_t("새 문서 ")+tool.Baro_comma(agg.Kinds["new_document"])+tool.Baro_t("개"))
	}
	if agg.Kinds["source"] > 0 {
		parts = append(parts, "출처 "+tool.Baro_comma(agg.Kinds["source"])+tool.Baro_t("건"))
	}
	if agg.Kinds["review"] > 0 {
		parts = append(parts, "검토 "+tool.Baro_comma(agg.Kinds["review"])+tool.Baro_t("건"))
	}
	if agg.Kinds["file"] > 0 {
		parts = append(parts, tool.Baro_t("파일 ")+tool.Baro_comma(agg.Kinds["file"])+tool.Baro_t("개"))
	}
	if agg.ProjectNum > 0 {
		parts = append(parts, tool.Baro_t("프로젝트 기여 ")+tool.Baro_comma(agg.ProjectNum)+tool.Baro_t("건"))
	}

	return `<span class="bw_muted">` + tool.HTML_escape(strings.Join(parts, " · ")) + `</span>`
}

// baro_award_doc_note : 문서 후보의 근거 수치
func baro_award_doc_note(db *sql.DB, title string, period string) string {
	events, editors, cites := 0, 0, 0
	tool.QueryRow_DB(db, "select count(*) from baro_contrib_event where title = ? and date like ?", []any{&events}, title, period+"%")
	tool.QueryRow_DB(db, "select count(distinct user) from baro_contrib_event where title = ?", []any{&editors}, title)
	tool.QueryRow_DB(db, "select count(*) from baro_citation where title = ?", []any{&cites}, title)

	text := tool.Baro_t("기간 편집 ") + tool.Baro_comma(events) + tool.Baro_t("건 · 참여자 ") + tool.Baro_comma(editors) + tool.Baro_t("명 · 출처 ") + tool.Baro_comma(cites) + tool.Baro_t("건")
	html := `<span class="bw_muted">` + tool.HTML_escape(text) + `</span>`

	if status := tool.Baro_doc_status_get(db, title); status != "" {
		html += " " + tool.Baro_status_badge(status)
	}

	return html
}

// baro_award_project_note : 프로젝트 후보의 근거 수치
func baro_award_project_note(db *sql.DB, id string) string {
	name, done, target, status, field := "", "0", "0", "", ""
	if !tool.QueryRow_DB(
		db,
		"select name, done_count, target_count, status, field from baro_project where id = ?",
		[]any{&name, &done, &target, &status, &field}, id,
	) {
		return tool.Baro_t(`<span class="bw_muted">사라진 프로젝트</span>`)
	}

	members := 0
	tool.QueryRow_DB(db, "select count(*) from baro_project_member where project = ?", []any{&members}, id)

	text := tool.Baro_t("완료 ") + tool.Baro_comma(tool.Str_to_int(done)) + tool.Baro_t(" / 목표 ") + tool.Baro_comma(tool.Str_to_int(target)) +
		tool.Baro_t(" · 참여 ") + tool.Baro_comma(members) + tool.Baro_t("명 · 분야 ") + tool.Baro_field_name(db, field) + tool.Baro_t(" · 상태 ") + status

	return `<span class="bw_muted">` + tool.HTML_escape(text) + `</span>`
}

// baro_award_target_html : 후보 대상 표시 (이미 HTML)
func baro_award_target_html(db *sql.DB, kind string, target string, my_ip string) string {
	switch kind {
	case "user":
		return tool.Baro_user_link(db, target, my_ip)
	case "document":
		return tool.Baro_doc_link(target)
	case "project":
		name := ""
		tool.QueryRow_DB(db, "select name from baro_project where id = ?", []any{&name}, target)
		if name == "" {
			name = "프로젝트 " + target
		}

		return tool.HTML_escape(name)
	}

	return tool.HTML_escape(target)
}

// baro_award_note_html : 후보 근거 수치 (종류에 맞춰 계산)
func baro_award_note_html(db *sql.DB, kind string, target string, period string, agg map[string]*baro_award_agg) string {
	switch kind {
	case "user":
		if agg != nil {
			return baro_award_agg_note(agg[target])
		}

		return baro_award_agg_note(baro_award_user_agg(db, target, period))
	case "document":
		return baro_award_doc_note(db, target, period)
	case "project":
		return baro_award_project_note(db, target)
	}

	return `<span class="bw_muted">-</span>`
}

// ------------------------------------------------------------
//  자동 자격 후보 계산 (12-3 1단계)
// ------------------------------------------------------------

// baro_award_rank_users : 사람 후보를 값 기준으로 줄 세우기
func baro_award_rank_users(agg map[string]*baro_award_agg, value func(*baro_award_agg) int, limit int) []baro_award_cand {
	out := []baro_award_cand{}

	for user, row := range agg {
		score := value(row)
		if score <= 0 {
			continue
		}

		out = append(out, baro_award_cand{
			Kind:   "user",
			Target: user,
			Value:  score,
			Note:   baro_award_agg_note(row),
		})
	}

	sort.SliceStable(out, func(i int, j int) bool {
		if out[i].Value == out[j].Value {
			return out[i].Target < out[j].Target
		}

		return out[i].Value > out[j].Value
	})

	if len(out) > limit {
		out = out[:limit]
	}

	return out
}

// baro_award_doc_rank : 기간 안 편집이 많은 문서
func baro_award_doc_rank(db *sql.DB, period string, limit int) []baro_award_cand {
	out := []baro_award_cand{}
	pick := [][]string{}

	rows := tool.Query_DB(
		db,
		"select title, count(*) from baro_contrib_event where title != '' and date like ? and status != 'rejected' "+
			"group by title order by count(*) desc limit "+strconv.Itoa(limit),
		period+"%",
	)
	defer rows.Close()

	for rows.Next() {
		title, count := "", 0
		if err := rows.Scan(&title, &count); err != nil {
			continue
		}

		pick = append(pick, []string{title, tool.Baro_itoa(count)})
	}

	for _, row := range pick {
		out = append(out, baro_award_cand{
			Kind:   "document",
			Target: row[0],
			Value:  tool.Str_to_int(row[1]),
			Note:   baro_award_doc_note(db, row[0], period),
		})
	}

	return out
}

// baro_award_hidden_rank : 출처는 탄탄한데 손이 적게 간 조용한 문서
func baro_award_hidden_rank(db *sql.DB, period string, limit int) []baro_award_cand {
	out := []baro_award_cand{}
	pick := [][]string{}

	rows := tool.Query_DB(
		db,
		"select title, count(*) from baro_citation where title != '' and date like ? group by title order by count(*) desc limit 300",
		period+"%",
	)
	defer rows.Close()

	for rows.Next() {
		title, count := "", 0
		if err := rows.Scan(&title, &count); err != nil {
			continue
		}

		pick = append(pick, []string{title, tool.Baro_itoa(count)})
	}

	for _, row := range pick {
		editors := 0
		tool.QueryRow_DB(db, "select count(distinct user) from baro_contrib_event where title = ?", []any{&editors}, row[0])
		if editors > 3 {
			continue
		}

		out = append(out, baro_award_cand{
			Kind:   "document",
			Target: row[0],
			Value:  tool.Str_to_int(row[1]),
			Note:   baro_award_doc_note(db, row[0], period),
		})

		if len(out) >= limit {
			break
		}
	}

	return out
}

// baro_award_project_rank : 기간 안 성과가 큰 프로젝트
func baro_award_project_rank(db *sql.DB, period string, limit int) []baro_award_cand {
	out := []baro_award_cand{}
	pick := [][]string{}

	rows := tool.Query_DB(
		db,
		"select id, done_count from baro_project where date like ? or start_date like ? or end_date like ? limit 300",
		period+"%", period+"%", period+"%",
	)
	defer rows.Close()

	for rows.Next() {
		id, done := "", "0"
		if err := rows.Scan(&id, &done); err != nil {
			continue
		}

		pick = append(pick, []string{id, done})
	}

	for _, row := range pick {
		out = append(out, baro_award_cand{
			Kind:   "project",
			Target: row[0],
			Value:  tool.Str_to_int(row[1]),
			Note:   baro_award_project_note(db, row[0]),
		})
	}

	sort.SliceStable(out, func(i int, j int) bool {
		if out[i].Value == out[j].Value {
			return out[i].Target < out[j].Target
		}

		return out[i].Value > out[j].Value
	})

	if len(out) > limit {
		out = out[:limit]
	}

	return out
}

// baro_award_min_event : 자동 자격의 최소 기여 건수
func baro_award_min_event(db *sql.DB, mvp bool) int {
	if mvp {
		return tool.Baro_get_int(db, "award_mvp_min_event", 3)
	}

	return tool.Baro_get_int(db, "award_year_min_event", 10)
}

// baro_award_candidates : 부문별 자동 자격 후보 계산 (12-3 1단계)
//
//	인기투표만으로 뽑으면 친목 대회가 되므로, 후보의 출발점은 언제나 기여 데이터다.
//	두 번째 반환값은 화면에 그대로 공개하는 자격 규칙 설명이다.
func baro_award_candidates(db *sql.DB, category string, period string, limit int) ([]baro_award_cand, string) {
	if limit < 1 {
		limit = 20
	}

	mvp := baro_award_is_mvp(category)
	min_event := baro_award_min_event(db, mvp)

	// 분야 부문 (월간 MVP 의 분야 부문 · 연간 역사/과학 부문)
	field := ""
	switch {
	case strings.HasPrefix(category, "mvp_field_"):
		field = strings.TrimPrefix(category, "mvp_field_")
	case category == "year_history":
		field = "history"
	case category == "year_science":
		field = "science"
	}

	if field != "" {
		agg := baro_award_collect(db, period)
		rule := tool.Baro_field_name(db, field) + tool.Baro_t(" 분야 기여 점수 합계 상위 (해당 분야 최소 기여 ") + tool.Baro_itoa(min_event) + "건)"

		return baro_award_rank_users(agg, func(a *baro_award_agg) int {
			if a.FieldNum[field] < min_event {
				return 0
			}

			return a.FieldScore[field]
		}, limit), rule
	}

	switch category {
	case "year_wikian":
		agg := baro_award_collect(db, period)

		return baro_award_rank_users(agg, func(a *baro_award_agg) int {
			if a.Events < min_event {
				return 0
			}

			return a.Score
		}, limit), tool.Baro_t("기간 내 기여 점수 합계 상위 (최소 기여 ") + tool.Baro_itoa(min_event) + "건)"

	case "year_rookie", "mvp_newbie":
		agg := baro_award_collect(db, period)
		first := baro_award_first_map(db)

		return baro_award_rank_users(agg, func(a *baro_award_agg) int {
			if a.Events < min_event {
				return 0
			}
			if !strings.HasPrefix(first[a.User], period) {
				return 0
			}

			return a.Score
		}, limit), tool.Baro_t("최초 기여가 ") + period + tool.Baro_t(" 안에 있는 사람 중 점수 합계 상위 (최소 기여 ") + tool.Baro_itoa(min_event) + "건)"

	case "year_photo":
		agg := baro_award_collect(db, period)

		return baro_award_rank_users(agg, func(a *baro_award_agg) int {
			return a.Kinds["file"]
		}, limit), tool.Baro_t("파일 추가 기여 건수 상위")

	case "year_reviewer", "mvp_review":
		agg := baro_award_collect(db, period)
		review := baro_award_review_map(db, period)

		return baro_award_rank_users(agg, func(a *baro_award_agg) int {
			return a.Kinds["review"] + review[a.User]
		}, limit), tool.Baro_t("검토 기여 이벤트 + 검토 큐 처리 건수 상위")

	case "mvp_source":
		agg := baro_award_collect(db, period)

		return baro_award_rank_users(agg, func(a *baro_award_agg) int {
			return a.Kinds["source"]
		}, limit), tool.Baro_t("출처 추가 기여 건수 상위")

	case "mvp_project":
		agg := baro_award_collect(db, period)

		return baro_award_rank_users(agg, func(a *baro_award_agg) int {
			return a.ProjectNum
		}, limit), tool.Baro_t("프로젝트에 묶인 기여 건수 상위")

	case "year_document":
		return baro_award_doc_rank(db, period, limit), tool.Baro_t("기간 내 편집 이벤트가 많은 문서 상위")

	case "year_hidden":
		return baro_award_hidden_rank(db, period, limit), tool.Baro_t("출처가 많지만 참여자가 3명 이하인 조용한 문서")

	case "year_project":
		return baro_award_project_rank(db, period, limit), tool.Baro_t("기간에 걸쳐 진행된 프로젝트 중 완료 수 상위")
	}

	agg := baro_award_collect(db, period)

	return baro_award_rank_users(agg, func(a *baro_award_agg) int {
		if a.Events < min_event {
			return 0
		}

		return a.Score
	}, limit), tool.Baro_t("기여 점수 합계 상위 (최소 기여 ") + tool.Baro_itoa(min_event) + "건)"
}
