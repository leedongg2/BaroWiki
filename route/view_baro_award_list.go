package route

import (
	"database/sql"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  12-2 올해의 바로위키인 — 시즌 목록 · 부문 상세
// ============================================================

// baro_award_menu : 어워드 화면 공통 메뉴
func baro_award_menu() [][]any {
	return [][]any{
		{"baro/award", tool.Baro_t("어워드")},
		{"baro/award/mvp", tool.Baro_t("월간 MVP")},
		{"baro/wrapped", tool.Baro_t("연말 결산")},
		{"baro", "바로위키"},
	}
}

// baro_award_tabs : 어워드 상단 탭
func baro_award_tabs(active string) string {
	return tool.Baro_tabs([][]string{
		{"/baro/award", tool.Baro_t("올해의 바로위키인")},
		{"/baro/award/mvp", tool.Baro_t("월간 MVP")},
		{"/baro/award/manage", tool.Baro_t("시즌 운영")},
		{"/baro/wrapped", tool.Baro_t("내 연말 결산")},
		{"/baro/wrapped/site", tool.Baro_t("사이트 결산")},
	}, active)
}

// baro_award_this_year : 올해 (연도 네 자리)
func baro_award_this_year() string {
	return tool.Get_slice(tool.Baro_today(), 0, 4)
}

// baro_award_season_kind : 시즌이 월간인지 연간인지
func baro_award_season_kind(season string) string {
	if strings.Contains(season, "-") {
		return tool.Baro_t("월간 MVP")
	}

	return tool.Baro_t("연간 어워드")
}

// baro_award_not_found : 존재하지 않는 부문 안내 (500 대신 안내 화면)
func baro_award_not_found(db *sql.DB, config tool.Config, message string) string {
	body := baro_award_tabs("/baro/award")
	body += tool.Baro_alert("warn", tool.HTML_escape(message))
	body += tool.Baro_card(tool.Baro_t("어워드로 돌아가기"),
		tool.Baro_button("/baro/award", tool.Baro_t("시즌 목록"), "primary")+" "+
			tool.Baro_button("/baro/award/mvp", tool.Baro_t("월간 MVP"), "")+" "+
			tool.Baro_button("/baro/wrapped", tool.Baro_t("연말 결산"), ""),
	)

	return tool.Baro_page(db, config, tool.Baro_t("어워드"), body, baro_award_menu())
}

// baro_award_season_summary : 한 시즌의 요약 줄 (이미 HTML)
func baro_award_season_summary(db *sql.DB, season string, my_ip string) string {
	rows := baro_award_season_rows(db, season)

	closed, open_num := 0, 0
	items := []string{}

	for _, row := range rows {
		if row.Status == "closed" {
			closed++
		} else {
			open_num++
		}

		text := `<a href="/baro/award/view/` + tool.Url_parser(row.ID) + `">` + tool.HTML_escape(row.Name) + `</a> `
		if row.Winner != "" {
			text += baro_award_target_html(db, baro_award_category_kind(row.Category), row.Winner, my_ip)
		} else {
			text += baro_award_status_badge(row.Status)
		}

		items = append(items, text)
	}

	body := tool.Baro_stat_row([][]string{
		{"부문", tool.Baro_comma(len(rows)), baro_award_season_kind(season)},
		{tool.Baro_t("확정"), tool.Baro_comma(closed), tool.Baro_t("결과 공개됨")},
		{tool.Baro_t("진행 중"), tool.Baro_comma(open_num), tool.Baro_t("후보·심사·투표")},
	})

	if len(items) == 0 {
		body += tool.Baro_empty(tool.Baro_t("이 시즌에는 아직 부문이 없습니다."))
	} else {
		body += tool.Baro_list(items)
	}

	body += tool.Baro_button("/baro/award/season/"+tool.Url_parser(season), "시즌 전체 보기", "")

	return body
}

// View_baro_award_list : /baro/award — 시즌 목록과 현재 시즌 진행 상황
func View_baro_award_list(config tool.Config, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	page_int := tool.Str_to_int(page)
	if page_int < 1 {
		page_int = 1
	}

	page_size := 10
	seasons := baro_award_season_list(db, (page_int-1)*page_size, page_size)

	body := baro_award_tabs("/baro/award")
	body += tool.Baro_hero(
		tool.Baro_t("바로위키 어워드"),
		tool.Baro_t("한 해 동안의 기록을 남긴 사람과 문서를 함께 기억하기 위한 시상입니다."),
		tool.Baro_note(tool.Baro_t("후보는 인기가 아니라 기여 데이터에서 출발합니다. 자동 자격 → 이용자 추천 → 기여 데이터 공개 → 심사위원 평가 → 커뮤니티 투표의 다섯 단계를 모두 거칩니다.")),
	)

	// --- 현재 시즌 진행 상황 ---
	year := baro_award_this_year()
	current := baro_award_season_rows(db, year)

	current_body := ""
	if len(current) == 0 {
		current_body = tool.Baro_empty(year + tool.Baro_t("년 시즌이 아직 열리지 않았습니다."))
		if tool.Baro_can(db, config.IP, "award") {
			current_body += tool.Baro_button("/baro/award/manage", tool.Baro_t("시즌 열기"), "primary")
		}
	} else {
		rows := [][]string{}
		for _, row := range current {
			winner := tool.Baro_t(`<span class="bw_muted">미정</span>`)
			if row.Winner != "" {
				winner = baro_award_target_html(db, baro_award_category_kind(row.Category), row.Winner, config.IP)
			}

			rows = append(rows, []string{
				`<a href="/baro/award/view/` + tool.Url_parser(row.ID) + `">` + tool.HTML_escape(row.Name) + `</a>` +
					`<div class="bw_note">` + tool.HTML_escape(row.Description) + `</div>`,
				baro_award_status_badge(row.Status),
				tool.Baro_comma(baro_award_nom_count(db, row.ID)) + tool.Baro_t("명"),
				winner,
			})
		}

		current_body = tool.Baro_table([]string{"부문", tool.Baro_t("진행 상태"), tool.Baro_t("후보"), tool.Baro_t("수상")}, rows, tool.Baro_t("부문이 없습니다."))
		current_body += tool.Baro_note(tool.Baro_t("부문 이름을 누르면 후보와 근거 수치, 진행 단계를 볼 수 있습니다."))
	}

	body += tool.Baro_card_ex(
		year+tool.Baro_t("년 시즌"),
		tool.Baro_t("올해의 바로위키인 진행 상황"),
		current_body,
		tool.Baro_button("/baro/award/season/"+tool.Url_parser(year), "올해 시즌 전체 보기", "")+" "+
			tool.Baro_button("/baro/wrapped/"+tool.Url_parser(year), year+"년 내 결산", ""),
	)

	// --- 부문 안내 ---
	category_items := []string{}
	for _, row := range baro_award_year_category_list() {
		category_items = append(category_items,
			`<b>`+tool.HTML_escape(row[1])+`</b> `+tool.Baro_badge(baro_award_target_kind_name(row[2]), "muted")+
				`<div class="bw_note">`+tool.HTML_escape(row[3])+`</div>`,
		)
	}
	body += tool.Baro_card(tool.Baro_t("연간 부문 아홉 가지"), tool.Baro_grid(category_items, 3))

	// --- 지난 시즌 목록 ---
	season_items := []string{}
	for _, season := range seasons {
		season_items = append(season_items, tool.Baro_card_ex(
			season,
			baro_award_season_kind(season),
			baro_award_season_summary(db, season, config.IP),
			"",
		))
	}

	season_body := ""
	if len(season_items) == 0 {
		season_body = tool.Baro_empty(tool.Baro_t("아직 열린 시즌이 없습니다. 시즌을 열면 이곳에 쌓입니다."))
	} else {
		season_body = tool.Baro_grid(season_items, 2)
	}

	season_body += tool.Baro_paging(page_int, len(seasons), page_size, "/baro/award/page/{}")
	body += tool.Baro_card(tool.Baro_t("시즌 목록"), season_body)

	body += tool.Baro_card(tool.Baro_t("선정 절차와 점수 공식"), baro_award_formula_html(db))

	return tool.Baro_page(db, config, tool.Baro_t("바로위키 어워드"), body, baro_award_menu())
}

// View_baro_award_season : /baro/award/season/<시즌> — 한 시즌의 모든 부문
func View_baro_award_season(config tool.Config, season string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	season = strings.TrimSpace(season)
	if season == "" {
		return baro_award_not_found(db, config, tool.Baro_t("시즌을 지정하지 않았습니다."))
	}

	rows := baro_award_season_rows(db, season)

	body := baro_award_tabs("/baro/award")
	body += tool.Baro_crumb([][]string{
		{"/baro/award", tool.Baro_t("어워드")},
		{"", season},
	})
	body += tool.Baro_hero(season+tool.Baro_t(" 시즌"), baro_award_season_kind(season), "")

	if len(rows) == 0 {
		body += tool.Baro_empty(tool.Baro_t("이 시즌에는 등록된 부문이 없습니다."))
		body += tool.Baro_button("/baro/award", tool.Baro_t("시즌 목록으로"), "primary")

		return tool.Baro_page(db, config, season+tool.Baro_t(" 시즌"), body, baro_award_menu())
	}

	table := [][]string{}
	for _, row := range rows {
		winner := tool.Baro_t(`<span class="bw_muted">미정</span>`)
		if row.Winner != "" {
			winner = baro_award_target_html(db, baro_award_category_kind(row.Category), row.Winner, config.IP)
		}

		reason := `<span class="bw_muted">-</span>`
		if row.ResultReason != "" {
			reason = tool.HTML_escape(tool.Baro_trunc(row.ResultReason, 160))
		}

		table = append(table, []string{
			`<a href="/baro/award/view/` + tool.Url_parser(row.ID) + `">` + tool.HTML_escape(row.Name) + `</a>`,
			baro_award_status_badge(row.Status),
			tool.Baro_comma(baro_award_nom_count(db, row.ID)) + tool.Baro_t("명"),
			winner,
			reason,
		})
	}

	body += tool.Baro_card("부문", tool.Baro_table(
		[]string{"부문", tool.Baro_t("진행 상태"), tool.Baro_t("후보"), tool.Baro_t("수상"), tool.Baro_t("선정 근거")}, table, tool.Baro_t("부문이 없습니다."),
	))

	body += tool.Baro_card(tool.Baro_t("점수 공식"), baro_award_formula_html(db))

	return tool.Baro_page(db, config, season+tool.Baro_t(" 시즌"), body, baro_award_menu())
}

// View_baro_award_view : /baro/award/view/<id> — 부문 상세 (후보 · 진행 단계 · 결과)
func View_baro_award_view(config tool.Config, id string, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row, found := baro_award_get(db, id)
	if !found {
		return baro_award_not_found(db, config, tool.Baro_t("그런 어워드 부문이 없습니다. 주소를 다시 확인해 주세요."))
	}

	page_int := tool.Str_to_int(page)
	if page_int < 1 {
		page_int = 1
	}

	page_size := tool.Baro_page_size
	period := row.Season
	locked := baro_award_locked(row)

	body := baro_award_tabs("/baro/award")
	body += tool.Baro_crumb([][]string{
		{"/baro/award", tool.Baro_t("어워드")},
		{"/baro/award/season/" + tool.Url_parser(row.Season), row.Season},
		{"", row.Name},
	})

	body += tool.Baro_hero(
		row.Name,
		row.Season+" · "+baro_award_season_kind(row.Season),
		baro_award_status_badge(row.Status)+" "+tool.Baro_badge(baro_award_target_kind_name(baro_award_category_kind(row.Category)), "muted")+
			`<p class="bw_note">`+tool.HTML_escape(row.Description)+`</p>`,
	)

	if locked {
		body += baro_award_lock_notice()
	}

	// --- 결과 ---
	if row.Winner != "" || row.ResultReason != "" {
		result := ""
		if row.Winner != "" {
			result += tool.Baro_kv_grid([][]string{
				{tool.Baro_t("수상"), baro_award_target_html(db, baro_award_category_kind(row.Category), row.Winner, config.IP)},
				{tool.Baro_t("확정 시각"), tool.Baro_time_cell(row.Date)},
			})
		}

		reason := row.ResultReason
		if strings.TrimSpace(reason) == "" {
			reason = tool.Baro_t("선정 근거가 아직 기록되지 않았습니다.")
		}
		result += tool.Baro_section(tool.Baro_t("최종 선정 근거"), `<div class="bw_share"><div class="bw_share_t">`+
			tool.HTML_escape(row.Name)+`</div>`+baro_award_text_html(reason)+`</div>`)

		body += tool.Baro_card(tool.Baro_t("결과"), result)
	}

	// --- 진행 단계 ---
	body += tool.Baro_card_ex(
		tool.Baro_t("선정 절차"),
		tool.Baro_t("다섯 단계를 모두 거칩니다"),
		baro_award_steps_html(db, row),
		tool.HTML_escape(baro_award_formula_text(db)),
	)

	// --- 후보 목록 ---
	total := baro_award_nom_count(db, row.ID)
	sorted, scores := baro_award_sorted_noms(db, row.ID)

	start := (page_int - 1) * page_size
	end := start + page_size
	if start > len(sorted) {
		start = len(sorted)
	}
	if end > len(sorted) {
		end = len(sorted)
	}
	view := sorted[start:end]

	agg := baro_award_agg_for(db, row.Category, period)

	table := [][]string{}
	for index, nom := range view {
		score := baro_award_score_of(scores, nom.ID)

		reason := `<span class="bw_muted">-</span>`
		if nom.Reason != "" {
			reason = tool.HTML_escape(tool.Baro_trunc(nom.Reason, 200))
		}

		nominator := tool.Baro_t(`<span class="bw_muted">자동</span>`)
		if nom.Nominator != "" {
			nominator = tool.Baro_user_link(db, nom.Nominator, config.IP)
		}

		table = append(table, []string{
			tool.Baro_itoa(start + index + 1),
			baro_award_target_html(db, nom.TargetKind, nom.Target, config.IP),
			tool.Baro_badge(baro_award_nom_kind_name(nom.Kind), "info") + `<div class="bw_note">` + nominator + `</div>`,
			baro_award_note_html(db, nom.TargetKind, nom.Target, period, agg),
			reason,
			baro_award_score_cell(score),
		})
	}

	nom_body := tool.Baro_table(
		[]string{tool.Baro_t("순위"), tool.Baro_t("후보"), tool.Baro_t("등록 경로"), tool.Baro_t("기여 데이터 (근거)"), tool.Baro_t("추천 사유"), tool.Baro_t("최종 점수")},
		table,
		"",
	)

	if total == 0 {
		nom_body = tool.Baro_empty(tool.Baro_t("아직 후보가 없습니다. 자동 자격 계산을 돌리거나 이용자 추천을 받아 보세요."))
	}

	nom_body += tool.Baro_paging(page_int, len(view), page_size, "/baro/award/view/"+tool.Url_parser(row.ID)+"?page={}")

	buttons := tool.Baro_button("/baro/award/auto/"+tool.Url_parser(row.ID), "자동 자격 후보", "")
	if !locked {
		buttons += " " + tool.Baro_button("/baro/award/nominate/"+tool.Url_parser(row.ID), "후보 추천하기", "primary")
		buttons += " " + tool.Baro_button("/baro/award/vote/"+tool.Url_parser(row.ID), "투표하기", "")
	}
	if !locked && tool.Baro_can(db, config.IP, "award") {
		buttons += " " + tool.Baro_button("/baro/award/judge/"+tool.Url_parser(row.ID), "심사하기", "")
		buttons += " " + tool.Baro_button("/baro/award/close/"+tool.Url_parser(row.ID), "결과 확정", "")
	}
	if locked {
		buttons += " " + tool.Baro_badge(tool.Baro_t("확정 · 잠김"), "ok")
	}

	body += tool.Baro_card_ex(tool.Baro_t("후보 ")+tool.Baro_comma(total)+tool.Baro_t("명"), tool.Baro_t("근거 수치와 함께 공개합니다"), nom_body, buttons)

	// --- 이해충돌 공개 ---
	coi := baro_award_coi_list(db, row.ID)
	coi_rows := [][]string{}
	for _, item := range coi {
		target := item[0]
		if nom, ok := baro_award_nom_get(db, item[0]); ok {
			target = ""
			target = baro_award_target_html(db, nom.TargetKind, nom.Target, config.IP)
		} else {
			target = tool.HTML_escape(target)
		}

		coi_rows = append(coi_rows, []string{
			tool.Baro_user_link(db, item[1], config.IP),
			target,
			tool.HTML_escape(item[2]),
			tool.Baro_time_cell(item[3]),
		})
	}

	coi_body := tool.Baro_table([]string{tool.Baro_t("심사위원"), tool.Baro_t("후보"), "사유", tool.Baro_t("기록 시각")}, coi_rows, "")
	if len(coi_rows) == 0 {
		coi_body = tool.Baro_empty(tool.Baro_t("공개된 이해충돌이 없습니다."))
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("이해충돌 공개"),
		tool.Baro_t("심사위원이 자기 자신이나 자기가 참여한 프로젝트를 심사하면 여기에 남기고 점수에서 제외합니다"),
		coi_body,
		"",
	)

	// --- 심사위원 명단 ---
	judge_count := map[string]int{}
	judge_coi := map[string]int{}
	for _, item := range baro_award_judge_all(db, row.ID) {
		judge_count[item.Judge]++
		if item.COI {
			judge_coi[item.Judge]++
		}
	}

	judge_rows := [][]string{}
	for judge, count := range judge_count {
		judge_rows = append(judge_rows, []string{
			tool.Baro_user_link(db, judge, config.IP),
			tool.Baro_comma(count) + tool.Baro_t("건"),
			tool.Baro_comma(judge_coi[judge]) + tool.Baro_t("건"),
		})
	}

	judge_body := tool.Baro_table([]string{tool.Baro_t("심사위원"), tool.Baro_t("평가"), tool.Baro_t("이해충돌 제외")}, judge_rows, "")
	if len(judge_rows) == 0 {
		judge_body = tool.Baro_empty(tool.Baro_t("아직 심사가 시작되지 않았습니다."))
	}

	body += tool.Baro_card(tool.Baro_t("심사위원 참여 현황"), judge_body)
	body += tool.Baro_card(tool.Baro_t("점수 공식"), baro_award_formula_html(db))

	return tool.Baro_page(db, config, row.Season+" "+row.Name, body, baro_award_menu())
}

// baro_award_text_html : 줄바꿈을 살린 여러 줄 문장 (사용자 입력이므로 반드시 escape)
func baro_award_text_html(text string) string {
	lines := tool.Baro_split_lines(text)

	out := ""
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		out += `<p>` + tool.HTML_escape(line) + `</p>`
	}

	if out == "" {
		return `<p class="bw_muted">-</p>`
	}

	return out
}
