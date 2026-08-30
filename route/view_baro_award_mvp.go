package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  12-1 월간 MVP — /baro/award/mvp
//
//  부문은 baro_field 에 등록된 분야에서 만들어 낸 분야 부문과
//  신규 기여자 · 문서 검토 · 출처 기여 · 프로젝트 네 가지 고정 부문으로 이루어진다.
//
//  월을 고르면 baro_contrib_event 와 baro_contrib_score 를 집계해
//  부문별 후보를 계산하고, 선정 결과를 baro_award (season=연-월) 에 저장한다.
// ============================================================

// baro_award_month_ok : "2026-08" 모양인지
func baro_award_month_ok(month string) bool {
	if tool.Get_len(month) != 7 {
		return false
	}
	if tool.Get_slice(month, 4, 5) != "-" {
		return false
	}

	year := tool.Str_to_int(tool.Get_slice(month, 0, 4))
	num := tool.Str_to_int(tool.Get_slice(month, 5, 7))

	return year >= 2000 && year <= 2999 && num >= 1 && num <= 12
}

// baro_award_mvp_rank : 이미 모아 둔 집계로 한 부문의 후보를 계산한다
//
//	분야 부문은 baro_field 를 그대로 따라가므로 분야가 늘어나면 부문도 늘어난다.
func baro_award_mvp_rank(
	category string,
	agg map[string]*baro_award_agg,
	first map[string]string,
	review map[string]int,
	month string,
	min_event int,
	limit int,
) ([]baro_award_cand, string) {
	if strings.HasPrefix(category, "mvp_field_") {
		field := strings.TrimPrefix(category, "mvp_field_")

		return baro_award_rank_users(agg, func(a *baro_award_agg) int {
			if a.FieldNum[field] < min_event {
				return 0
			}

			return a.FieldScore[field]
		}, limit), tool.Baro_t("이 달 해당 분야 기여 점수 합계 상위 (분야 기여 ") + tool.Baro_itoa(min_event) + tool.Baro_t("건 이상)")
	}

	switch category {
	case "mvp_newbie":
		return baro_award_rank_users(agg, func(a *baro_award_agg) int {
			if a.Events < min_event {
				return 0
			}
			if !strings.HasPrefix(first[a.User], month) {
				return 0
			}

			return a.Score
		}, limit), tool.Baro_t("최초 기여가 이 달인 사람 중 점수 합계 상위 (기여 ") + tool.Baro_itoa(min_event) + tool.Baro_t("건 이상)")

	case "mvp_review":
		return baro_award_rank_users(agg, func(a *baro_award_agg) int {
			return a.Kinds["review"] + review[a.User]
		}, limit), tool.Baro_t("검토 기여 이벤트 + 검토 큐 처리 건수 상위")

	case "mvp_source":
		return baro_award_rank_users(agg, func(a *baro_award_agg) int {
			return a.Kinds["source"]
		}, limit), tool.Baro_t("출처 추가 기여 건수 상위")

	case "mvp_project":
		return baro_award_rank_users(agg, func(a *baro_award_agg) int {
			return a.ProjectNum
		}, limit), tool.Baro_t("프로젝트에 묶인 기여 건수 상위")
	}

	return baro_award_rank_users(agg, func(a *baro_award_agg) int {
		if a.Events < min_event {
			return 0
		}

		return a.Score
	}, limit), tool.Baro_t("기여 점수 합계 상위")
}

// baro_award_mvp_ensure : 그 달 그 부문의 어워드 줄을 만들어 두고 돌려준다
func baro_award_mvp_ensure(db *sql.DB, month string, category string, name string, description string) baro_award_row {
	if row, found := baro_award_find(db, month, category); found {
		return row
	}

	id := tool.Baro_next_id(db, "baro_award")
	tool.Exec_DB(
		db,
		"insert into baro_award (id, season, category, name, description, status, date, result_reason, winner) values (?, ?, ?, ?, ?, 'open', ?, '', '')",
		id, month, category, name, description, tool.Baro_now(),
	)

	row, _ := baro_award_get(db, id)

	return row
}

// View_baro_award_mvp : /baro/award/mvp — 월간 MVP 집계와 선정
func View_baro_award_mvp(config tool.Config, month string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	month = strings.TrimSpace(month)
	if !baro_award_month_ok(month) {
		month = tool.Get_month()
	}

	can := tool.Baro_can(db, config.IP, "award")
	categories := baro_award_mvp_category_list(db)
	message := ""

	// --- 처리 ---
	if values != nil {
		if !can {
			return tool.Baro_deny(db, config, tool.Baro_t("월간 MVP 선정은 어워드 운영 권한(award)이 있는 사람만 할 수 있습니다."))
		}

		action := values.Get("action")
		post_month := strings.TrimSpace(values.Get("month"))
		if baro_award_month_ok(post_month) {
			month = post_month
		}

		if text := baro_award_readonly_msg(db); text != "" {
			message = tool.Baro_alert("warn", tool.HTML_escape(text))
		} else {
			switch action {
			case "open":
				// 이 달의 모든 부문을 만들어 후보 추천·심사·투표를 열어 둔다.
				added := 0
				for _, category := range categories {
					if _, found := baro_award_find(db, month, category[0]); found {
						continue
					}

					baro_award_mvp_ensure(db, month, category[0], category[1], category[3])
					added++
				}

				tool.Baro_public_log(db, "award_season", config.IP, month, "월간 MVP 부문 개설", "부문 "+tool.Baro_itoa(added)+"개")
				tool.Baro_audit(db, config, "award_mvp_open", "season:"+month, "", "added="+tool.Baro_itoa(added), "월간 MVP 개설")

				return tool.Get_redirect("/baro/award/mvp?month=" + tool.Url_parser(month))

			case "select":
				category := strings.TrimSpace(values.Get("category"))
				winner := strings.TrimSpace(values.Get("winner"))
				reason := strings.TrimSpace(values.Get("reason"))

				name := baro_award_category_name(db, category)

				known := false
				for _, item := range categories {
					if item[0] == category {
						known = true
						break
					}
				}

				switch {
				case !known:
					message = tool.Baro_alert("danger", tool.Baro_t("부문을 다시 골라 주세요."))
				case winner == "":
					message = tool.Baro_alert("danger", tool.Baro_t("선정할 사람을 골라 주세요."))
				case tool.Get_len(reason) < 10:
					message = tool.Baro_alert("danger", tool.Baro_t("선정 근거를 열 글자 이상 적어 주세요."))
				default:
					// 집계로 계산된 후보가 아니면 고를 수 없다.
					// (근거 없는 선정은 친목 시상이 되고, 확정은 되돌릴 수 없다)
					cand_list, cand_rule := baro_award_candidates(db, category, month, 5)

					picked := baro_award_cand{}
					in_list := false

					for _, item := range cand_list {
						if item.Target == winner {
							picked = item
							in_list = true

							break
						}
					}

					if !in_list {
						message = tool.Baro_alert("danger", tool.Baro_t("집계로 계산된 후보 중에서만 고를 수 있습니다. 기여 데이터에 근거가 없는 대상은 선정할 수 없습니다."))
						break
					}

					row := baro_award_mvp_ensure(db, month, category, name, baro_award_category_desc(db, category))
					if baro_award_locked(row) {
						message = tool.Baro_alert("danger", tool.Baro_t("이미 확정된 부문입니다. 결과는 영구 보존됩니다."))
						break
					}

					baro_award_nom_add(
						db, row, "user", winner, "", "mvp",
						tool.Baro_t("월간 집계 선정 : ")+cand_rule+" (계산값 "+tool.Baro_comma(picked.Value)+")",
					)

					tool.Exec_DB(
						db,
						"update baro_award set winner = ?, result_reason = ?, status = 'closed', date = ? where id = ?",
						winner, tool.Baro_trunc(reason, 2000), tool.Baro_now(), row.ID,
					)

					tool.Baro_notify(
						db, winner, "award",
						month+" "+name+tool.Baro_t(" MVP 로 선정됐습니다."),
						"/baro/award/view/"+tool.Url_parser(row.ID), "",
					)
					tool.Baro_impact_bump(db, winner, "award_win", 1)

					tool.Baro_public_log(
						db, "award_mvp", config.IP, month+" "+name, tool.Baro_trunc(reason, 400),
						"수상 : "+winner+" · 자격 규칙 : "+cand_rule+" (계산값 "+tool.Baro_comma(picked.Value)+")",
					)
					tool.Baro_audit(db, config, "award_mvp", "baro_award:"+row.ID, "", "winner="+winner, tool.Baro_trunc(reason, 400))

					return tool.Get_redirect("/baro/award/mvp?month=" + tool.Url_parser(month))
				}
			}
		}
	}

	// --- 집계 (한 번만 모아서 모든 부문이 함께 쓴다) ---
	agg := baro_award_collect(db, month)
	first := baro_award_first_map(db)
	review := baro_award_review_map(db, month)
	min_event := baro_award_min_event(db, true)

	total_events, total_score := 0, 0
	for _, row := range agg {
		total_events += row.Events
		total_score += row.Score
	}

	body := baro_award_tabs("/baro/award/mvp")
	body += tool.Baro_hero(
		tool.Baro_t("월간 MVP"),
		month+tool.Baro_t(" 한 달 동안의 기여를 부문별로 모아 봅니다."),
		tool.Baro_note(tool.Baro_t("후보는 baro_contrib_event 와 baro_contrib_score 집계로 계산합니다. 오타 1,000번보다 좋은 문서 100개를 인정하기 위해 행동 종류별 가중치를 쓴 확정 점수를 씁니다.")),
	)
	body += message

	// 월 고르기
	month_form := tool.Baro_input_type("month", "월", month, "month", tool.Baro_t("고른 달의 기여만 집계합니다."))
	month_form += tool.Baro_submit(tool.Baro_t("이 달 보기"), "primary")

	body += tool.Baro_card_ex(
		tool.Baro_t("월 고르기"),
		tool.Baro_t("기여 집계 기간"),
		`<form class="bw_form" method="get" action="/baro/award/mvp">`+month_form+`</form>`,
		"",
	)

	body += tool.Baro_card(month+tool.Baro_t(" 요약"), tool.Baro_stat_row([][]string{
		{tool.Baro_t("기여자"), tool.Baro_comma(len(agg)), tool.Baro_t("이 달에 기록을 남긴 사람")},
		{tool.Baro_t("기여 건수"), tool.Baro_comma(total_events), tool.Baro_t("행동 종류별로 분류된 이벤트")},
		{tool.Baro_t("기여 점수"), tool.Baro_comma(total_score), tool.Baro_t("확정 점수 합계")},
		{"부문", tool.Baro_comma(len(categories)), tool.Baro_t("분야 부문 + 고정 부문")},
	}))

	if can {
		open_form := tool.Baro_hidden("action", "open") + tool.Baro_hidden("month", month)
		open_form += tool.Baro_note(tool.Baro_t("이 달의 모든 부문을 만들어 후보 추천 · 심사 · 투표를 열어 둡니다."))
		open_form += tool.Baro_submit(month+tool.Baro_t(" 부문 열기"), "")

		body += tool.Baro_card(tool.Baro_t("부문 열기"), tool.Baro_form("/baro/award/mvp", open_form))
	}

	// --- 부문별 후보 ---
	empty_num := 0
	for _, category := range categories {
		key := category[0]
		name := category[1]

		list, rule := baro_award_mvp_rank(key, agg, first, review, month, min_event, 5)
		row, exists := baro_award_find(db, month, key)

		table := [][]string{}
		options := [][]string{{"", tool.Baro_t("고르지 않음")}}

		for index, cand := range list {
			table = append(table, []string{
				tool.Baro_itoa(index + 1),
				baro_award_target_html(db, cand.Kind, cand.Target, config.IP),
				tool.Baro_comma(cand.Value),
				cand.Note,
			})

			options = append(options, []string{cand.Target, cand.Target + " (" + tool.Baro_comma(cand.Value) + ")"})
		}

		inner := tool.Baro_table([]string{tool.Baro_t("순위"), tool.Baro_t("후보"), tool.Baro_t("계산값"), tool.Baro_t("기여 데이터 (근거)")}, table, "")
		if len(list) == 0 {
			empty_num++
			inner = tool.Baro_empty(tool.Baro_t("이 달에는 자격을 만족한 후보가 없습니다."))
		}

		inner += tool.Baro_note(tool.Baro_t("자격 규칙 : ") + rule)

		foot := ""
		if exists {
			foot += tool.Baro_button("/baro/award/view/"+tool.Url_parser(row.ID), "부문 화면", "")

			if row.Winner != "" {
				inner += tool.Baro_alert("ok", tool.Baro_t("<b>선정</b> ")+
					baro_award_target_html(db, "user", row.Winner, config.IP)+
					`<div class="bw_note">`+tool.HTML_escape(tool.Baro_trunc(row.ResultReason, 300))+`</div>`)
			}
			if baro_award_locked(row) {
				foot += " " + tool.Baro_badge(tool.Baro_t("확정 · 잠김"), "ok")
			}
		}

		if can && (!exists || !baro_award_locked(row)) && len(list) > 0 {
			select_form := tool.Baro_hidden("action", "select")
			select_form += tool.Baro_hidden("month", month)
			select_form += tool.Baro_hidden("category", key)
			select_form += tool.Baro_select("winner", tool.Baro_t("선정"), options, "", tool.Baro_t("집계 상위 다섯 명 중에서 고릅니다."))
			select_form += tool.Baro_textarea("reason", tool.Baro_t("선정 근거"), "", 3, tool.Baro_t("왜 이 사람인지 적어 주세요."), tool.Baro_t("열 글자 이상. 결과와 함께 공개됩니다."))
			select_form += tool.Baro_submit(tool.Baro_t("이 부문 선정"), "primary")

			inner += tool.Baro_form("/baro/award/mvp", select_form)
		}

		body += tool.Baro_card_ex(name, category[3], inner, foot)
	}

	if empty_num == len(categories) {
		body += tool.Baro_alert("info", tool.Baro_t("이 달에는 아직 집계할 기여 기록이 없습니다. 다른 달을 골라 보세요."))
	}

	body += tool.Baro_card(tool.Baro_t("점수 공식"), baro_award_formula_html(db))
	body += tool.Baro_card(tool.Baro_t("이 달의 결과"),
		tool.Baro_button("/baro/award/season/"+tool.Url_parser(month), month+" 시즌 보기", "primary")+" "+
			tool.Baro_button("/baro/award", tool.Baro_t("연간 어워드"), ""),
	)

	return tool.Baro_page(db, config, tool.Baro_t("월간 MVP"), body, baro_award_menu())
}
