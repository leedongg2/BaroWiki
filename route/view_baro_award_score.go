package route

import (
	"database/sql"
	"sort"
	"strconv"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  12-3 선정 절차 — 심사 · 투표 · 최종 점수
//
//  절차는 다섯 단계다.
//    1) 자동 자격 후보 (기여 데이터로 자동 등록)
//    2) 이용자 추천    (사유를 적어 후보 등록)
//    3) 기여 데이터    (후보 옆에 근거 수치 공개)
//    4) 심사위원 평가  (권한 award 인 사람이 점수)
//    5) 커뮤니티 투표  (로그인 1인 1표, 일부만 반영)
//
//  심사 점수는 baro_set 의 이름-값 저장소에 담는다.
//  (새 표를 만들지 않는다는 계약 6번 규칙을 지키기 위한 방법)
//    이름 : award_judge:<부문ID>:<후보ID>:<심사위원>
//    값   : {"score":"8","comment":"...","coi":"on","date":"..."}
// ============================================================

// baro_award_judge_row : 심사위원 평가 한 건
type baro_award_judge_row struct {
	Nomination string
	Judge      string
	Score      int
	Comment    string
	COI        bool
	COIReason  string
	Date       string
}

// baro_award_score_row : 후보 한 명의 최종 점수 계산 결과
type baro_award_score_row struct {
	Nomination string
	JudgeSum   int
	JudgeNum   int
	CoiNum     int
	Votes      int
	JudgePct   float64
	VotePct    float64
	Final      float64
}

// baro_award_judge_max : 심사위원이 줄 수 있는 최고 점수
func baro_award_judge_max(db *sql.DB) int {
	max_score := tool.Baro_get_int(db, "award_judge_max", 10)
	if max_score < 1 {
		max_score = 10
	}

	return max_score
}

// baro_award_judge_weight : 최종 점수에서 심사 점수가 차지하는 비율 (%)
func baro_award_judge_weight(db *sql.DB) int {
	weight := tool.Baro_get_int(db, "award_judge_weight", 70)
	if weight < 0 {
		weight = 0
	}
	if weight > 100 {
		weight = 100
	}

	return weight
}

// baro_award_formula_text : 화면에 공개하는 최종 점수 공식
func baro_award_formula_text(db *sql.DB) string {
	judge := baro_award_judge_weight(db)
	vote := 100 - judge
	max_score := baro_award_judge_max(db)

	return tool.Baro_t("최종 점수 = (심사 평균 ÷ ") + tool.Baro_itoa(max_score) + " × 100) × " +
		tool.Baro_itoa(judge) + tool.Baro_t("% + (해당 부문 득표율 %) × ") + tool.Baro_itoa(vote) + "%"
}

// baro_award_formula_html : 공식 안내 상자
func baro_award_formula_html(db *sql.DB) string {
	html := `<p><b>` + tool.HTML_escape(baro_award_formula_text(db)) + `</b></p>`
	html += tool.Baro_t(`<p>심사 점수는 `) + tool.Baro_itoa(baro_award_judge_weight(db)) + tool.Baro_t(`%, 커뮤니티 투표는 `) +
		tool.Baro_itoa(100-baro_award_judge_weight(db)) + tool.Baro_t(`%만 반영합니다. `) +
		tool.Baro_t(`인기투표만으로 뽑으면 친목 대회가 되기 때문에, 투표는 참고 지표로만 씁니다.</p>`)
	html += tool.Baro_t(`<p>이해충돌이 있는 심사(본인·본인이 참여한 프로젝트)는 표시된 뒤 평균에서 제외됩니다.</p>`)

	return tool.Baro_alert("info", html)
}

// baro_award_judge_key : 심사 저장 키
func baro_award_judge_key(award string, nomination string, judge string) string {
	return "award_judge:" + award + ":" + nomination + ":" + judge
}

// baro_award_judge_parse : 저장 키에서 후보 ID 와 심사위원 이름 꺼내기
func baro_award_judge_parse(name string) (string, string, string, bool) {
	parts := strings.SplitN(name, ":", 4)
	if len(parts) != 4 || parts[0] != "award_judge" {
		return "", "", "", false
	}

	return parts[1], parts[2], parts[3], true
}

// baro_award_judge_all : 한 부문의 심사 평가 전체
func baro_award_judge_all(db *sql.DB, award string) []baro_award_judge_row {
	out := []baro_award_judge_row{}

	rows := tool.Query_DB(db, "select name, data from baro_set where name like ? limit 5000", "award_judge:"+award+":%")
	defer rows.Close()

	for rows.Next() {
		name, data := "", ""
		if err := rows.Scan(&name, &data); err != nil {
			continue
		}

		award_id, nomination, judge, ok := baro_award_judge_parse(name)
		if !ok || award_id != award {
			continue
		}

		value := tool.Baro_json_dec_str(data)
		out = append(out, baro_award_judge_row{
			Nomination: nomination,
			Judge:      judge,
			Score:      tool.Str_to_int(value["score"]),
			Comment:    value["comment"],
			COI:        value["coi"] == "on",
			COIReason:  value["coi_reason"],
			Date:       value["date"],
		})
	}

	sort.SliceStable(out, func(i int, j int) bool {
		if out[i].Nomination == out[j].Nomination {
			return out[i].Judge < out[j].Judge
		}

		return tool.Str_to_int(out[i].Nomination) < tool.Str_to_int(out[j].Nomination)
	})

	return out
}

// baro_award_judge_one : 심사위원 한 사람의 한 후보 평가
func baro_award_judge_one(db *sql.DB, award string, nomination string, judge string) baro_award_judge_row {
	value := tool.Baro_json_dec_str(tool.Baro_get(db, baro_award_judge_key(award, nomination, judge)))

	return baro_award_judge_row{
		Nomination: nomination,
		Judge:      judge,
		Score:      tool.Str_to_int(value["score"]),
		Comment:    value["comment"],
		COI:        value["coi"] == "on",
		COIReason:  value["coi_reason"],
		Date:       value["date"],
	}
}

// baro_award_judge_save : 심사 평가 저장
func baro_award_judge_save(db *sql.DB, award string, nomination string, judge string, score int, comment string, coi bool, coi_reason string) {
	value := map[string]string{
		"score":      tool.Baro_itoa(score),
		"comment":    tool.Baro_trunc(comment, 1000),
		"coi":        "",
		"coi_reason": coi_reason,
		"date":       tool.Baro_now(),
	}
	if coi {
		value["coi"] = "on"
	}

	tool.Baro_put(db, baro_award_judge_key(award, nomination, judge), tool.Baro_json_enc(value))
}

// baro_award_judge_count : 이 부문을 평가한 심사위원 수
func baro_award_judge_count(db *sql.DB, award string) int {
	seen := map[string]bool{}
	for _, row := range baro_award_judge_all(db, award) {
		seen[row.Judge] = true
	}

	return len(seen)
}

// ------------------------------------------------------------
//  이해충돌 (12-3)
// ------------------------------------------------------------

// baro_award_coi_reason : 심사위원과 후보 사이의 이해충돌 사유 (없으면 "")
func baro_award_coi_reason(db *sql.DB, judge string, kind string, target string) string {
	if judge == "" || target == "" {
		return ""
	}

	if kind == "user" && judge == target {
		return tool.Baro_t("후보가 심사위원 본인입니다.")
	}

	if kind == "project" {
		owner := ""
		tool.QueryRow_DB(db, "select owner from baro_project where id = ?", []any{&owner}, target)
		if owner != "" && owner == judge {
			return tool.Baro_t("심사위원이 이 프로젝트의 운영자입니다.")
		}

		found := ""
		tool.QueryRow_DB(
			db,
			"select id from baro_project_member where project = ? and user = ?",
			[]any{&found}, target, judge,
		)
		if found != "" {
			return tool.Baro_t("심사위원이 이 프로젝트에 참여하고 있습니다.")
		}
	}

	return ""
}

// baro_award_coi_note : 이해충돌은 아니지만 참고로 알릴 관계 (없으면 "")
func baro_award_coi_note(db *sql.DB, judge string, kind string, target string) string {
	if judge == "" || target == "" || kind != "document" {
		return ""
	}

	count := 0
	tool.QueryRow_DB(db, "select count(*) from baro_contrib_event where title = ? and user = ?", []any{&count}, target, judge)
	if count > 0 {
		return tool.Baro_t("심사위원이 이 문서에 ") + tool.Baro_comma(count) + tool.Baro_t("건 기여한 이력이 있습니다.")
	}

	return ""
}

// baro_award_coi_log : 이해충돌 공개 기록 (baro_coi)
func baro_award_coi_log(db *sql.DB, award string, nomination string, judge string, reason string) {
	case_id := "award:" + award + ":" + nomination

	found := ""
	tool.QueryRow_DB(db, "select id from baro_coi where case_id = ? and actor = ?", []any{&found}, case_id, judge)
	if found != "" {
		return
	}

	tool.Exec_DB(
		db,
		"insert into baro_coi (id, case_id, actor, reason, date) values (?, ?, ?, ?, ?)",
		tool.Baro_next_id(db, "baro_coi"), case_id, judge, tool.Baro_trunc(reason, 500), tool.Baro_now(),
	)
}

// baro_award_coi_list : 한 부문에 공개된 이해충돌 목록
func baro_award_coi_list(db *sql.DB, award string) [][]string {
	out := [][]string{}

	rows := tool.Query_DB(
		db,
		"select case_id, actor, reason, date from baro_coi where case_id like ? order by id + 0 desc limit 200",
		"award:"+award+":%",
	)
	defer rows.Close()

	for rows.Next() {
		case_id, actor, reason, date := "", "", "", ""
		if err := rows.Scan(&case_id, &actor, &reason, &date); err != nil {
			continue
		}

		nomination := ""
		if parts := strings.SplitN(case_id, ":", 3); len(parts) == 3 {
			nomination = parts[2]
		}

		out = append(out, []string{nomination, actor, reason, date})
	}

	return out
}

// ------------------------------------------------------------
//  커뮤니티 투표 (12-3 5단계)
// ------------------------------------------------------------

// baro_award_vote_map : 후보별 득표 수와 총 득표 수
func baro_award_vote_map(db *sql.DB, award string) (map[string]int, int) {
	out := map[string]int{}
	total := 0

	rows := tool.Query_DB(db, "select nomination, count(*) from baro_award_vote where award = ? group by nomination", award)
	defer rows.Close()

	for rows.Next() {
		nomination, count := "", 0
		if err := rows.Scan(&nomination, &count); err != nil {
			continue
		}

		out[nomination] = count
		total += count
	}

	return out, total
}

// baro_award_vote_of : 이 사람이 이 부문에서 이미 던진 표 (없으면 "")
func baro_award_vote_of(db *sql.DB, award string, voter string) string {
	if !tool.Baro_is_user(voter) {
		return ""
	}

	nomination := ""
	tool.QueryRow_DB(db, "select nomination from baro_award_vote where award = ? and voter = ?", []any{&nomination}, award, voter)

	return nomination
}

// baro_award_vote_put : 1인 1표 (다시 던지면 이전 표를 바꾼다)
func baro_award_vote_put(db *sql.DB, award string, nomination string, voter string) {
	tool.Exec_DB(db, "delete from baro_award_vote where award = ? and voter = ?", award, voter)
	tool.Exec_DB(
		db,
		"insert into baro_award_vote (id, award, nomination, voter, weight, date) values (?, ?, ?, ?, '1', ?)",
		tool.Baro_next_id(db, "baro_award_vote"), award, nomination, voter, tool.Baro_now(),
	)
}

// ------------------------------------------------------------
//  최종 점수
// ------------------------------------------------------------

// baro_award_score_map : 후보별 최종 점수 계산
func baro_award_score_map(db *sql.DB, award string) map[string]*baro_award_score_row {
	out := map[string]*baro_award_score_row{}

	max_score := baro_award_judge_max(db)
	judge_weight := float64(baro_award_judge_weight(db)) / 100
	vote_weight := 1 - judge_weight

	for _, row := range baro_award_judge_all(db, award) {
		score, ok := out[row.Nomination]
		if !ok {
			score = &baro_award_score_row{Nomination: row.Nomination}
			out[row.Nomination] = score
		}

		// 이해충돌이 있는 심사는 표시만 하고 평균에서 뺀다.
		if row.COI {
			score.CoiNum++
			continue
		}

		score.JudgeSum += row.Score
		score.JudgeNum++
	}

	votes, total := baro_award_vote_map(db, award)
	for nomination, count := range votes {
		score, ok := out[nomination]
		if !ok {
			score = &baro_award_score_row{Nomination: nomination}
			out[nomination] = score
		}

		score.Votes = count
	}

	for _, score := range out {
		if score.JudgeNum > 0 && max_score > 0 {
			score.JudgePct = float64(score.JudgeSum) / float64(score.JudgeNum) / float64(max_score) * 100
		}
		if total > 0 {
			score.VotePct = float64(score.Votes) / float64(total) * 100
		}

		score.Final = score.JudgePct*judge_weight + score.VotePct*vote_weight
	}

	return out
}

// baro_award_score_of : 후보 하나의 점수 (없으면 빈 값)
func baro_award_score_of(scores map[string]*baro_award_score_row, nomination string) *baro_award_score_row {
	if score, ok := scores[nomination]; ok {
		return score
	}

	return &baro_award_score_row{Nomination: nomination}
}

// baro_award_num_text : 소수 첫째 자리까지
func baro_award_num_text(value float64) string {
	return strconv.FormatFloat(value, 'f', 1, 64)
}

// baro_award_score_cell : 표에 넣을 점수 칸 (이미 HTML)
func baro_award_score_cell(score *baro_award_score_row) string {
	html := `<b>` + tool.HTML_escape(baro_award_num_text(score.Final)) + `</b>`
	html += tool.Baro_t(`<div class="bw_note">심사 `) + tool.HTML_escape(baro_award_num_text(score.JudgePct)) + ` (` +
		tool.Baro_itoa(score.JudgeNum) + tool.Baro_t(`명) · 득표 `) + tool.Baro_itoa(score.Votes) + tool.Baro_t(`표 (`) +
		tool.HTML_escape(baro_award_num_text(score.VotePct)) + `%)`

	if score.CoiNum > 0 {
		html += tool.Baro_t(` · 이해충돌 제외 `) + tool.Baro_itoa(score.CoiNum) + `건`
	}

	return html + `</div>`
}

// baro_award_recalc : 계산한 최종 점수를 후보 표에 적어 둔다
func baro_award_recalc(db *sql.DB, award string) {
	scores := baro_award_score_map(db, award)

	for _, row := range baro_award_nom_all(db, award) {
		score := baro_award_score_of(scores, row.ID)
		tool.Exec_DB(db, "update baro_nomination set score = ? where id = ?", baro_award_num_text(score.Final), row.ID)
	}
}

// baro_award_sorted_noms : 최종 점수가 높은 순으로 정렬한 후보
func baro_award_sorted_noms(db *sql.DB, award string) ([]baro_award_nom_row, map[string]*baro_award_score_row) {
	list := baro_award_nom_all(db, award)
	scores := baro_award_score_map(db, award)

	sort.SliceStable(list, func(i int, j int) bool {
		left := baro_award_score_of(scores, list[i].ID).Final
		right := baro_award_score_of(scores, list[j].ID).Final
		if left == right {
			return tool.Str_to_int(list[i].ID) < tool.Str_to_int(list[j].ID)
		}

		return left > right
	})

	return list, scores
}

// ------------------------------------------------------------
//  진행 단계 표시
// ------------------------------------------------------------

// baro_award_steps_html : 5단계 선정 절차의 현재 상태 (12-3)
func baro_award_steps_html(db *sql.DB, row baro_award_row) string {
	auto_num := baro_award_nom_count_kind(db, row.ID, "auto")
	user_num := baro_award_nom_count_kind(db, row.ID, "user")
	all_num := baro_award_nom_count(db, row.ID)
	judge_num := baro_award_judge_count(db, row.ID)
	judge_all := len(baro_award_judge_all(db, row.ID))
	_, vote_total := baro_award_vote_map(db, row.ID)

	id_url := tool.Url_parser(row.ID)
	locked := baro_award_locked(row)

	link := func(url string, text string) string {
		if locked {
			return tool.Baro_badge(tool.Baro_t("잠김"), "muted")
		}

		return tool.Baro_button(url, text, "")
	}

	rows := [][]string{
		{
			"1",
			"자동 자격 후보",
			tool.Baro_t("기여 데이터로 자격을 만족한 대상을 자동으로 후보에 올립니다."),
			tool.Baro_comma(auto_num) + tool.Baro_t("명 등록"),
			tool.Baro_button("/baro/award/auto/"+id_url, tool.Baro_t("자격 계산 보기"), ""),
		},
		{
			"2",
			tool.Baro_t("이용자 추천"),
			tool.Baro_t("누구나 사유를 적어 후보를 추천할 수 있습니다."),
			tool.Baro_comma(user_num) + tool.Baro_t("건 추천"),
			link("/baro/award/nominate/"+id_url, tool.Baro_t("추천하기")),
		},
		{
			"3",
			tool.Baro_t("기여 데이터 공개"),
			tool.Baro_t("후보 옆에 근거가 되는 수치를 함께 보여 줍니다."),
			tool.Baro_comma(all_num) + tool.Baro_t("명 후보"),
			tool.Baro_button("/baro/award/view/"+id_url, tool.Baro_t("후보 보기"), ""),
		},
		{
			"4",
			tool.Baro_t("심사위원 평가"),
			tool.Baro_t("권한이 award 인 심사위원이 후보마다 점수를 매깁니다."),
			tool.Baro_comma(judge_num) + tool.Baro_t("명 · ") + tool.Baro_comma(judge_all) + tool.Baro_t("건 평가"),
			link("/baro/award/judge/"+id_url, "심사하기"),
		},
		{
			"5",
			"커뮤니티 투표",
			tool.Baro_t("로그인 사용자 1인 1표. 최종 점수에 ") + tool.Baro_itoa(100-baro_award_judge_weight(db)) + tool.Baro_t("%만 반영합니다."),
			tool.Baro_comma(vote_total) + tool.Baro_t("표"),
			link("/baro/award/vote/"+id_url, "투표하기"),
		},
	}

	out := [][]string{}
	for _, step := range rows {
		out = append(out, []string{
			tool.HTML_escape(step[0]),
			`<b>` + tool.HTML_escape(step[1]) + `</b><div class="bw_note">` + tool.HTML_escape(step[2]) + `</div>`,
			tool.HTML_escape(step[3]),
			step[4],
		})
	}

	return tool.Baro_table([]string{tool.Baro_t("단계"), tool.Baro_t("내용"), tool.Baro_t("현황"), "이동"}, out, "")
}
