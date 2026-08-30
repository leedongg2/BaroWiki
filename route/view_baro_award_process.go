package route

import (
	"database/sql"
	"net/url"
	"strconv"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  12-3 선정 절차 화면
//
//    /baro/award/auto/<id>     1) 자동 자격 후보
//    /baro/award/nominate/<id> 2) 이용자 추천
//    /baro/award/judge/<id>    4) 심사위원 평가
//    /baro/award/vote/<id>     5) 커뮤니티 투표
//    /baro/award/close/<id>    최종 선정 근거 기록과 확정
//    /baro/award/manage        시즌 개설 · 단계 이동 · 가중치
//
//  3) 기여 데이터 공개는 모든 화면의 후보 옆에 함께 붙어 있다.
// ============================================================

// baro_award_readonly_msg : 읽기 전용일 때의 안내 (아니면 "")
func baro_award_readonly_msg(db *sql.DB) string {
	if !tool.Baro_read_only(db) {
		return ""
	}

	reason := tool.Baro_read_only_reason(db)
	if strings.TrimSpace(reason) == "" {
		reason = tool.Baro_t("지금은 읽기 전용 상태입니다.")
	}

	return reason
}

// baro_award_score_check : 심사 점수 입력값 검사
//
//	0 ~ max_score 사이의 정수만 받아들인다. 범위를 벗어나거나 숫자가 아니면
//	두 번째 값이 false 로 온다. (몰래 값을 깎아 저장하지 않기 위한 검사)
func baro_award_score_check(raw string, max_score int) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}

	score, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}

	if score < 0 || score > max_score {
		return score, false
	}

	return score, true
}

// baro_award_locked_page : 잠긴 시즌에 쓰기를 시도했을 때의 안내 화면
func baro_award_locked_page(db *sql.DB, config tool.Config, row baro_award_row, title string) string {
	body := baro_award_tabs("/baro/award")
	body += tool.Baro_crumb([][]string{
		{"/baro/award", tool.Baro_t("어워드")},
		{"/baro/award/view/" + tool.Url_parser(row.ID), row.Name},
		{"", title},
	})
	body += baro_award_lock_notice()
	body += tool.Baro_card(row.Season+" "+row.Name,
		tool.Baro_button("/baro/award/view/"+tool.Url_parser(row.ID), "결과 보기", "primary"),
	)

	return tool.Baro_page(db, config, title, body, baro_award_menu())
}

// ------------------------------------------------------------
//  1) 자동 자격 후보
// ------------------------------------------------------------

// View_baro_award_auto : /baro/award/auto/<id> — 기여 데이터로 자격을 만족한 후보 계산
func View_baro_award_auto(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row, found := baro_award_get(db, id)
	if !found {
		return baro_award_not_found(db, config, tool.Baro_t("그런 어워드 부문이 없습니다."))
	}

	limit := tool.Baro_get_int(db, "award_auto_limit", 20)
	if limit < 1 {
		limit = 20
	}

	list, rule := baro_award_candidates(db, row.Category, row.Season, limit)
	can := tool.Baro_can(db, config.IP, "award")

	// --- 처리 ---
	if values != nil {
		if !can {
			return tool.Baro_deny(db, config, tool.Baro_t("자동 자격 후보를 등록하려면 어워드 운영 권한이 필요합니다."))
		}
		if baro_award_locked(row) {
			return baro_award_locked_page(db, config, row, "자동 자격 후보")
		}
		if message := baro_award_readonly_msg(db); message != "" {
			return baro_award_not_found(db, config, message)
		}

		added := 0
		for _, cand := range list {
			if baro_award_nom_add(db, row, cand.Kind, cand.Target, "", "auto",
				"자동 자격 : "+rule+" (계산값 "+tool.Baro_comma(cand.Value)+")") != "" {
				added++
			}
		}

		baro_award_recalc(db, row.ID)

		tool.Baro_public_log(
			db, "award_auto", config.IP, row.Season+" "+row.Name,
			"자동 자격 후보 "+tool.Baro_itoa(added)+"명 등록", rule,
		)
		tool.Baro_audit(
			db, config, "award_auto", "baro_award:"+row.ID, "",
			"added="+tool.Baro_itoa(added), rule,
		)

		return tool.Get_redirect("/baro/award/view/" + tool.Url_parser(row.ID))
	}

	// --- 보여주기 ---
	body := baro_award_tabs("/baro/award")
	body += tool.Baro_crumb([][]string{
		{"/baro/award", tool.Baro_t("어워드")},
		{"/baro/award/view/" + tool.Url_parser(row.ID), row.Name},
		{"", "자동 자격 후보"},
	})

	body += tool.Baro_hero(
		"자동 자격 후보",
		row.Season+" · "+row.Name,
		tool.Baro_alert("info", tool.Baro_t("<b>1단계.</b> 인기가 아니라 기여 데이터로 자격을 판정합니다. 아래 규칙을 만족한 대상은 추천 없이도 후보가 됩니다.")),
	)

	body += tool.Baro_card(tool.Baro_t("적용 규칙"), tool.Baro_kv_grid([][]string{
		{"기간", tool.HTML_escape(row.Season)},
		{tool.Baro_t("대상 종류"), tool.HTML_escape(baro_award_target_kind_name(baro_award_category_kind(row.Category)))},
		{tool.Baro_t("자격 규칙"), tool.HTML_escape(rule)},
		{tool.Baro_t("최대 인원"), tool.Baro_comma(limit) + tool.Baro_t("명")},
	}))

	table := [][]string{}
	for index, cand := range list {
		mark := tool.Baro_badge(tool.Baro_t("새 후보"), "info")
		if baro_award_nom_exists(db, row.ID, cand.Kind, cand.Target) {
			mark = tool.Baro_badge(tool.Baro_t("이미 등록됨"), "ok")
		}

		table = append(table, []string{
			tool.Baro_itoa(index + 1),
			baro_award_target_html(db, cand.Kind, cand.Target, config.IP),
			tool.Baro_comma(cand.Value),
			cand.Note,
			mark,
		})
	}

	list_body := tool.Baro_table([]string{tool.Baro_t("순위"), tool.Baro_t("대상"), tool.Baro_t("계산값"), tool.Baro_t("기여 데이터 (근거)"), "상태"}, table, "")
	if len(list) == 0 {
		list_body = tool.Baro_empty(tool.Baro_t("자격을 만족한 대상이 아직 없습니다. 기간 안의 기여 기록이 쌓이면 여기에 나타납니다."))
	}

	foot := ""
	if baro_award_locked(row) {
		foot = tool.Baro_badge(tool.Baro_t("확정된 시즌이라 등록할 수 없습니다."), "muted")
	} else if can && len(list) > 0 {
		foot = tool.Baro_form("/baro/award/auto/"+tool.Url_parser(row.ID),
			tool.Baro_hidden("confirm", "on")+
				tool.Baro_note(tool.Baro_t("아래 목록을 그대로 후보로 등록합니다. 이미 등록된 대상은 건너뜁니다."))+
				tool.Baro_submit(tool.Baro_t("자동 자격 후보 등록"), "primary"),
		)
	} else if !can {
		foot = tool.Baro_note(tool.Baro_t("등록은 어워드 운영 권한이 있는 사람만 할 수 있습니다. 계산 결과는 누구나 볼 수 있습니다."))
	}

	body += tool.Baro_card_ex(tool.Baro_t("계산 결과"), tool.Baro_t("근거 수치와 함께 공개합니다"), list_body, foot)
	body += tool.Baro_button("/baro/award/view/"+tool.Url_parser(row.ID), "부문으로 돌아가기", "")

	return tool.Baro_page(db, config, "자동 자격 후보", body, baro_award_menu())
}

// ------------------------------------------------------------
//  2) 이용자 추천
// ------------------------------------------------------------

// View_baro_award_nominate : /baro/award/nominate/<id> — 사유를 적어 후보 추천
func View_baro_award_nominate(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row, found := baro_award_get(db, id)
	if !found {
		return baro_award_not_found(db, config, tool.Baro_t("그런 어워드 부문이 없습니다."))
	}

	if wall, need := tool.Baro_login_wall(db, config, "/baro/award/nominate/"+tool.Url_parser(row.ID)); need {
		return wall
	}

	if baro_award_locked(row) {
		return baro_award_locked_page(db, config, row, tool.Baro_t("후보 추천"))
	}

	kind := baro_award_category_kind(row.Category)
	message := ""

	// --- 처리 ---
	if values != nil {
		target := strings.TrimSpace(values.Get("target"))
		reason := strings.TrimSpace(values.Get("reason"))

		switch {
		case baro_award_readonly_msg(db) != "":
			message = tool.Baro_alert("warn", tool.HTML_escape(baro_award_readonly_msg(db)))
		case !tool.Baro_rate_ok(db, "award_nominate", config.IP, 10, 3600):
			message = tool.Baro_alert("danger", tool.Baro_t("추천이 너무 잦습니다. 한 시간에 열 번까지만 추천할 수 있습니다."))
		case target == "":
			message = tool.Baro_alert("danger", tool.Baro_t("추천할 대상을 적어 주세요."))
		case tool.Get_len(reason) < 10:
			message = tool.Baro_alert("danger", tool.Baro_t("추천 사유를 열 글자 이상 적어 주세요. 사유 없는 추천은 친목 투표가 됩니다."))
		case kind == "user" && !tool.Baro_is_user(target):
			message = tool.Baro_alert("danger", tool.Baro_t("사용자 이름을 정확히 적어 주세요. 비로그인 IP 는 후보가 될 수 없습니다."))
		case !baro_award_target_exists(db, kind, target):
			message = tool.Baro_alert("danger", tool.HTML_escape(baro_award_target_miss(kind)))
		case baro_award_nom_exists(db, row.ID, kind, target):
			message = tool.Baro_alert("warn", tool.Baro_t("이미 후보로 올라와 있는 대상입니다."))
		default:
			new_id := baro_award_nom_add(db, row, kind, target, config.IP, "user", reason)
			if new_id == "" {
				message = tool.Baro_alert("danger", tool.Baro_t("후보를 등록하지 못했습니다."))
				break
			}

			baro_award_recalc(db, row.ID)

			tool.Baro_public_log(
				db, "award_nominate", config.IP, row.Season+" "+row.Name,
				"이용자 추천", baro_award_target_kind_name(kind)+" 후보 추가",
			)
			tool.Baro_audit(db, config, "award_nominate", "baro_award:"+row.ID, "", "target="+target, reason)

			return tool.Get_redirect("/baro/award/view/" + tool.Url_parser(row.ID))
		}
	}

	// --- 보여주기 ---
	help := tool.Baro_t("사용자 이름을 그대로 적어 주세요.")
	switch kind {
	case "document":
		help = tool.Baro_t("문서 제목을 그대로 적어 주세요.")
	case "project":
		help = tool.Baro_t("프로젝트 번호를 적어 주세요. (프로젝트 목록에서 확인할 수 있습니다)")
	}

	body := baro_award_tabs("/baro/award")
	body += tool.Baro_crumb([][]string{
		{"/baro/award", tool.Baro_t("어워드")},
		{"/baro/award/view/" + tool.Url_parser(row.ID), row.Name},
		{"", tool.Baro_t("후보 추천")},
	})

	body += tool.Baro_hero(
		tool.Baro_t("후보 추천"),
		row.Season+" · "+row.Name,
		tool.Baro_alert("info", tool.Baro_t("<b>2단계.</b> 자동 자격에서 빠졌더라도 이용자가 사유를 적어 후보로 올릴 수 있습니다. 사유는 그대로 공개됩니다.")),
	)
	body += message

	form := tool.Baro_input("target", baro_award_target_kind_name(kind)+tool.Baro_t(" 이름"), "", "", help)
	form += tool.Baro_textarea("reason", tool.Baro_t("추천 사유"), "", 6, tool.Baro_t("어떤 기여가 왜 인정받아야 하는지 구체적으로 적어 주세요."),
		tool.Baro_t("열 글자 이상. 사유는 후보 목록에 그대로 공개됩니다."))
	form += tool.Baro_note(tool.Baro_t("추천된 대상에게는 어워드 알림이 갑니다."))
	form += tool.Baro_submit(tool.Baro_t("후보로 추천"), "primary")

	body += tool.Baro_card("추천서 작성", tool.Baro_form("/baro/award/nominate/"+tool.Url_parser(row.ID), form))

	// 이미 올라온 이용자 추천 목록
	list := baro_award_nom_query(
		db,
		"select "+baro_award_nom_cols+" from baro_nomination where award = ? and kind = 'user' order by id + 0 desc limit 30",
		row.ID,
	)

	// 추천 목록에도 근거가 되는 기여 수치를 함께 보여 준다 (12-3 3단계)
	nom_agg := baro_award_agg_for(db, row.Category, row.Season)

	table := [][]string{}
	for _, nom := range list {
		table = append(table, []string{
			baro_award_target_html(db, nom.TargetKind, nom.Target, config.IP),
			baro_award_note_html(db, nom.TargetKind, nom.Target, row.Season, nom_agg),
			tool.Baro_user_link(db, nom.Nominator, config.IP),
			tool.HTML_escape(tool.Baro_trunc(nom.Reason, 200)),
			tool.Baro_time_cell(nom.Date),
		})
	}

	list_body := tool.Baro_table([]string{tool.Baro_t("후보"), tool.Baro_t("기여 데이터 (근거)"), tool.Baro_t("추천인"), "사유", "시각"}, table, "")
	if len(table) == 0 {
		list_body = tool.Baro_empty(tool.Baro_t("아직 이용자 추천이 없습니다. 첫 추천을 남겨 주세요."))
	}

	body += tool.Baro_card(tool.Baro_t("이용자 추천 목록"), list_body)

	return tool.Baro_page(db, config, tool.Baro_t("후보 추천"), body, baro_award_menu())
}

// ------------------------------------------------------------
//  4) 심사위원 평가
// ------------------------------------------------------------

// View_baro_award_judge : /baro/award/judge/<id> — 심사위원 점수 매기기
func View_baro_award_judge(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row, found := baro_award_get(db, id)
	if !found {
		return baro_award_not_found(db, config, tool.Baro_t("그런 어워드 부문이 없습니다."))
	}

	if !tool.Baro_can(db, config.IP, "award") {
		return tool.Baro_deny(db, config, tool.Baro_t("심사는 어워드 운영 권한(award)이 있는 사람만 할 수 있습니다."))
	}

	if baro_award_locked(row) {
		return baro_award_locked_page(db, config, row, "심사")
	}

	max_score := baro_award_judge_max(db)
	list := baro_award_nom_all(db, row.ID)
	message := ""

	// 저장이 거절됐을 때 사용자가 친 값을 그대로 다시 보여 주기 위해 들고 있는다.
	typed_score := map[string]string{}
	typed_comment := map[string]string{}

	// --- 처리 ---
	if values != nil {
		if text := baro_award_readonly_msg(db); text != "" {
			message = tool.Baro_alert("warn", tool.HTML_escape(text))
		} else {
			// 1) 먼저 모든 점수를 검사한다. 하나라도 잘못됐으면 아무것도 저장하지 않고
			//    무엇이 왜 잘못됐는지 알려 준다. (몰래 값을 깎지 않는다)
			type baro_award_judge_input struct {
				Nom     baro_award_nom_row
				Score   int
				Comment string
			}

			input_list := []baro_award_judge_input{}
			bad_list := []string{}

			for _, nom := range list {
				raw := strings.TrimSpace(values.Get("score_" + nom.ID))
				comment := strings.TrimSpace(values.Get("comment_" + nom.ID))

				typed_score[nom.ID] = raw
				typed_comment[nom.ID] = comment

				// 점수와 의견을 둘 다 비워 두면 그 후보는 건너뛴다.
				if raw == "" && comment == "" {
					continue
				}

				if raw == "" {
					bad_list = append(bad_list,
						tool.Baro_t("후보 #")+tool.HTML_escape(nom.ID)+tool.Baro_t(" : 심사 의견만 적혀 있고 점수 칸이 비어 있습니다.")+
							tool.Baro_t(" 점수는 0~")+tool.Baro_itoa(max_score)+tool.Baro_t(" 사이로 넣어 주세요."))
					continue
				}

				score, ok := baro_award_score_check(raw, max_score)
				if !ok {
					bad_list = append(bad_list,
						tool.Baro_t("후보 #")+tool.HTML_escape(nom.ID)+tool.Baro_t(" : 넣으신 값은 ")+
							tool.HTML_escape(tool.Baro_trunc(raw, 30))+tool.Baro_t(" 입니다.")+
							tool.Baro_t(" 점수는 0~")+tool.Baro_itoa(max_score)+tool.Baro_t(" 사이 정수로 넣어 주세요."))
					continue
				}

				input_list = append(input_list, baro_award_judge_input{Nom: nom, Score: score, Comment: comment})
			}

			if len(bad_list) > 0 {
				// 한 칸이라도 잘못됐으면 저장하지 않고 사유를 보여 준 뒤
				// 사용자가 친 값 그대로 다시 그려 준다. (아래 보여주기로 이어진다)
				message = tool.Baro_alert("danger",
					tool.Baro_t("<b>점수는 0~")+tool.Baro_itoa(max_score)+tool.Baro_t(" 사이로 넣어 주세요.</b>")+
						tool.Baro_t(" 잘못 넣은 칸이 있어 이번 심사는 한 건도 저장하지 않았습니다. 아래 사유를 보고 고쳐 주세요.")+
						tool.Baro_list(bad_list))
			} else {
				saved, coi_num := 0, 0

				for _, one := range input_list {
					nom := one.Nom

					coi_reason := baro_award_coi_reason(db, config.IP, nom.TargetKind, nom.Target)
					coi := coi_reason != ""
					if coi {
						coi_num++
						baro_award_coi_log(db, row.ID, nom.ID, config.IP, coi_reason)
					}

					baro_award_judge_save(db, row.ID, nom.ID, config.IP, one.Score, one.Comment, coi, coi_reason)
					saved++
				}

				baro_award_recalc(db, row.ID)

				// 심사가 시작되면 시즌 단계를 심사·투표 중으로 옮긴다.
				if row.Status == "open" {
					tool.Exec_DB(db, "update baro_award set status = 'pending' where id = ?", row.ID)
				}

				tool.Baro_public_log(
					db, "award_judge", config.IP, row.Season+" "+row.Name,
					"심사 평가 "+tool.Baro_itoa(saved)+"건 저장",
					"이해충돌로 제외된 심사 "+tool.Baro_itoa(coi_num)+"건",
				)
				tool.Baro_audit(
					db, config, "award_judge", "baro_award:"+row.ID, "",
					"saved="+tool.Baro_itoa(saved)+" coi="+tool.Baro_itoa(coi_num), "심사위원 평가",
				)

				return tool.Get_redirect("/baro/award/view/" + tool.Url_parser(row.ID))
			}
		}
	}

	// --- 보여주기 ---
	body := baro_award_tabs("/baro/award")
	body += tool.Baro_crumb([][]string{
		{"/baro/award", tool.Baro_t("어워드")},
		{"/baro/award/view/" + tool.Url_parser(row.ID), row.Name},
		{"", "심사"},
	})

	body += tool.Baro_hero(
		tool.Baro_t("심사위원 평가"),
		row.Season+" · "+row.Name,
		tool.Baro_alert("info", tool.Baro_t("<b>4단계.</b> 후보마다 0 ~ ")+tool.Baro_itoa(max_score)+
			tool.Baro_t("점을 매깁니다. 점수는 최종 점수의 ")+tool.Baro_itoa(baro_award_judge_weight(db))+tool.Baro_t("%로 반영됩니다.")),
	)
	body += message
	body += baro_award_formula_html(db)

	agg := baro_award_agg_for(db, row.Category, row.Season)

	form := ""
	for _, nom := range list {
		mine := baro_award_judge_one(db, row.ID, nom.ID, config.IP)
		coi_reason := baro_award_coi_reason(db, config.IP, nom.TargetKind, nom.Target)
		coi_note := baro_award_coi_note(db, config.IP, nom.TargetKind, nom.Target)

		head := baro_award_target_html(db, nom.TargetKind, nom.Target, config.IP)
		head += " " + tool.Baro_badge(baro_award_nom_kind_name(nom.Kind), "muted")

		inner := `<div>` + head + `</div>`
		inner += `<div class="bw_note">` + baro_award_note_html(db, nom.TargetKind, nom.Target, row.Season, agg) + `</div>`

		if nom.Reason != "" {
			inner += tool.Baro_note(tool.Baro_t("추천 사유 : ") + tool.Baro_trunc(nom.Reason, 300))
		}

		if coi_reason != "" {
			inner += tool.Baro_alert("danger", tool.Baro_t("<b>이해충돌.</b> ")+tool.HTML_escape(coi_reason)+
				tool.Baro_t(" 이 심사는 공개 기록으로 남고 평균 점수에서 제외됩니다."))
		} else if coi_note != "" {
			inner += tool.Baro_alert("warn", tool.HTML_escape(coi_note)+tool.Baro_t(" (제외 대상은 아니지만 참고해 주세요)"))
		}

		score_value := ""
		if mine.Date != "" {
			score_value = tool.Baro_itoa(mine.Score)
		}

		// 저장이 거절된 경우에는 사용자가 직접 친 값을 그대로 남겨 고쳐 넣을 수 있게 한다.
		if typed, ok := typed_score[nom.ID]; ok {
			score_value = typed
		}

		inner += tool.Baro_input_type("score_"+nom.ID, tool.Baro_t("점수 (0 ~ ")+tool.Baro_itoa(max_score)+")", score_value, "number", "0 ~ "+tool.Baro_itoa(max_score)+tool.Baro_t(" 사이 정수만 받습니다. 비워 두면 이 후보는 건너뜁니다."))
		comment_value := mine.Comment
		if typed, ok := typed_comment[nom.ID]; ok {
			comment_value = typed
		}

		inner += tool.Baro_textarea("comment_"+nom.ID, tool.Baro_t("심사 의견"), comment_value, 3, tool.Baro_t("왜 이 점수인지 적어 주세요."), tool.Baro_t("최종 선정 근거를 쓸 때 쓰입니다."))

		if mine.Date != "" {
			inner += tool.Baro_note(tool.Baro_t("마지막 저장 : ") + mine.Date)
		}

		form += tool.Baro_section(tool.Baro_t("후보 #")+nom.ID, inner)
	}

	if len(list) == 0 {
		body += tool.Baro_card("심사", tool.Baro_empty(tool.Baro_t("아직 후보가 없습니다. 자동 자격 계산이나 이용자 추천이 먼저 필요합니다.")))
	} else {
		form += tool.Baro_submit(tool.Baro_t("심사 점수 저장"), "primary")
		body += tool.Baro_card_ex(
			tool.Baro_t("후보 ")+tool.Baro_comma(len(list))+tool.Baro_t("명 심사"),
			tool.Baro_t("기여 데이터를 함께 보며 점수를 매깁니다"),
			tool.Baro_form("/baro/award/judge/"+tool.Url_parser(row.ID), form),
			tool.HTML_escape(baro_award_formula_text(db)),
		)
	}

	body += tool.Baro_button("/baro/award/view/"+tool.Url_parser(row.ID), "부문으로 돌아가기", "")

	return tool.Baro_page(db, config, "심사", body, baro_award_menu())
}

// ------------------------------------------------------------
//  5) 커뮤니티 투표
// ------------------------------------------------------------

// View_baro_award_vote : /baro/award/vote/<id> — 로그인 사용자 1인 1표
func View_baro_award_vote(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row, found := baro_award_get(db, id)
	if !found {
		return baro_award_not_found(db, config, tool.Baro_t("그런 어워드 부문이 없습니다."))
	}

	if wall, need := tool.Baro_login_wall(db, config, "/baro/award/vote/"+tool.Url_parser(row.ID)); need {
		return wall
	}

	if baro_award_locked(row) {
		return baro_award_locked_page(db, config, row, tool.Baro_t("투표"))
	}

	list, scores := baro_award_sorted_noms(db, row.ID)
	mine := baro_award_vote_of(db, row.ID, config.IP)
	message := ""

	// --- 처리 ---
	if values != nil {
		pick := strings.TrimSpace(values.Get("nomination"))

		ok := false
		for _, nom := range list {
			if nom.ID == pick {
				ok = true
				break
			}
		}

		switch {
		case baro_award_readonly_msg(db) != "":
			message = tool.Baro_alert("warn", tool.HTML_escape(baro_award_readonly_msg(db)))
		case !ok:
			message = tool.Baro_alert("danger", tool.Baro_t("후보를 하나 골라 주세요."))
		case len(tool.Baro_sanction_active(db, config.IP)) > 0:
			message = tool.Baro_alert("danger", tool.Baro_t("제재가 진행 중인 계정은 투표할 수 없습니다."))
		case !tool.Baro_rate_ok(db, "award_vote", config.IP, 30, 3600):
			message = tool.Baro_alert("danger", tool.Baro_t("투표 요청이 너무 잦습니다. 잠시 뒤에 다시 시도해 주세요."))
		default:
			baro_award_vote_put(db, row.ID, pick, config.IP)
			baro_award_recalc(db, row.ID)

			tool.Baro_public_log(db, "award_vote", config.IP, row.Season+" "+row.Name, "커뮤니티 투표", "1인 1표")

			return tool.Get_redirect("/baro/award/vote/" + tool.Url_parser(row.ID))
		}
	}

	// --- 보여주기 ---
	_, total := baro_award_vote_map(db, row.ID)

	body := baro_award_tabs("/baro/award")
	body += tool.Baro_crumb([][]string{
		{"/baro/award", tool.Baro_t("어워드")},
		{"/baro/award/view/" + tool.Url_parser(row.ID), row.Name},
		{"", tool.Baro_t("투표")},
	})

	body += tool.Baro_hero(
		"커뮤니티 투표",
		row.Season+" · "+row.Name,
		tool.Baro_alert("info", tool.Baro_t("<b>5단계.</b> 로그인 사용자 1인 1표입니다. 표는 최종 점수의 ")+
			tool.Baro_itoa(100-baro_award_judge_weight(db))+tool.Baro_t("%만 차지합니다. 다시 던지면 이전 표가 바뀝니다.")),
	)
	body += message
	body += baro_award_formula_html(db)

	if mine != "" {
		if nom, ok := baro_award_nom_get(db, mine); ok {
			body += tool.Baro_alert("ok", tool.Baro_t("현재 내 표 : ")+baro_award_target_html(db, nom.TargetKind, nom.Target, config.IP))
		}
	}

	agg := baro_award_agg_for(db, row.Category, row.Season)

	form := ""
	for _, nom := range list {
		score := baro_award_score_of(scores, nom.ID)

		checked := ""
		if nom.ID == mine {
			checked = ` checked`
		}

		item := `<label class="bw_cb"><input type="radio" name="nomination" value="` + tool.HTML_escape(nom.ID) + `"` + checked + `> <span>` +
			baro_award_target_html(db, nom.TargetKind, nom.Target, config.IP) + `</span></label>`
		item += `<div class="bw_note">` + baro_award_note_html(db, nom.TargetKind, nom.Target, row.Season, agg) + `</div>`

		if nom.Reason != "" {
			item += `<div class="bw_note">` + tool.HTML_escape(tool.Baro_trunc(nom.Reason, 200)) + `</div>`
		}

		item += `<div class="bw_note">` + tool.Baro_comma(score.Votes) + tool.Baro_t(`표 · 득표율 `) +
			tool.HTML_escape(baro_award_num_text(score.VotePct)) + `%</div>`

		form += `<div class="bw_f">` + item + `</div>`
	}

	vote_body := ""
	if len(list) == 0 {
		vote_body = tool.Baro_empty(tool.Baro_t("아직 후보가 없어 투표할 수 없습니다."))
	} else {
		form += tool.Baro_submit(tool.Baro_t("이 후보에 투표"), "primary")
		vote_body = tool.Baro_form("/baro/award/vote/"+tool.Url_parser(row.ID), form)
	}

	body += tool.Baro_card_ex(tool.Baro_t("후보 ")+tool.Baro_comma(len(list))+tool.Baro_t("명"), tool.Baro_t("총 ")+tool.Baro_comma(total)+tool.Baro_t("표"), vote_body, "")
	body += tool.Baro_button("/baro/award/view/"+tool.Url_parser(row.ID), "부문으로 돌아가기", "")

	return tool.Baro_page(db, config, "커뮤니티 투표", body, baro_award_menu())
}

// ------------------------------------------------------------
//  결과 확정 (최종 선정 근거 공개 · 영구 보존)
// ------------------------------------------------------------

// View_baro_award_close : /baro/award/close/<id> — 수상자와 선정 근거를 적어 확정
func View_baro_award_close(config tool.Config, id string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	row, found := baro_award_get(db, id)
	if !found {
		return baro_award_not_found(db, config, tool.Baro_t("그런 어워드 부문이 없습니다."))
	}

	if !tool.Baro_can(db, config.IP, "award") {
		return tool.Baro_deny(db, config, tool.Baro_t("결과 확정은 어워드 운영 권한(award)이 있는 사람만 할 수 있습니다."))
	}

	if baro_award_locked(row) {
		return baro_award_locked_page(db, config, row, "결과 확정")
	}

	list, scores := baro_award_sorted_noms(db, row.ID)
	message := ""

	// --- 처리 ---
	if values != nil {
		winner := strings.TrimSpace(values.Get("winner"))
		reason := strings.TrimSpace(values.Get("result_reason"))
		confirm := values.Get("confirm")

		ok := false
		for _, nom := range list {
			if nom.Target == winner {
				ok = true
				break
			}
		}

		switch {
		case baro_award_readonly_msg(db) != "":
			message = tool.Baro_alert("warn", tool.HTML_escape(baro_award_readonly_msg(db)))
		case !ok:
			message = tool.Baro_alert("danger", tool.Baro_t("후보 중에서 수상자를 골라 주세요."))
		case tool.Get_len(reason) < 20:
			message = tool.Baro_alert("danger", tool.Baro_t("최종 선정 근거를 스무 글자 이상 적어 주세요. 근거 없는 확정은 공개할 수 없습니다."))
		case confirm != "on":
			message = tool.Baro_alert("danger", tool.Baro_t("확정하면 되돌릴 수 없습니다. 확인란에 표시해 주세요."))
		default:
			before := "winner=" + row.Winner + " status=" + row.Status

			tool.Exec_DB(
				db,
				"update baro_award set winner = ?, result_reason = ?, status = 'closed', date = ? where id = ?",
				winner, tool.Baro_trunc(reason, 4000), tool.Baro_now(), row.ID,
			)
			tool.Exec_DB(db, "update baro_nomination set status = 'closed' where award = ?", row.ID)

			kind := baro_award_category_kind(row.Category)
			if kind == "user" {
				tool.Baro_notify(
					db, winner, "award",
					row.Season+" "+row.Name+tool.Baro_t("에 선정됐습니다."),
					"/baro/award/view/"+tool.Url_parser(row.ID), "",
				)
				tool.Baro_impact_bump(db, winner, "award_win", 1)
			}

			tool.Baro_public_log(
				db, "award_close", config.IP, row.Season+" "+row.Name,
				tool.Baro_trunc(reason, 400), "수상 : "+winner,
			)
			tool.Baro_audit(
				db, config, "award_close", "baro_award:"+row.ID, before,
				"winner="+winner+" status=closed", tool.Baro_trunc(reason, 400),
			)

			return tool.Get_redirect("/baro/award/view/" + tool.Url_parser(row.ID))
		}
	}

	// --- 보여주기 ---
	body := baro_award_tabs("/baro/award")
	body += tool.Baro_crumb([][]string{
		{"/baro/award", tool.Baro_t("어워드")},
		{"/baro/award/view/" + tool.Url_parser(row.ID), row.Name},
		{"", "결과 확정"},
	})

	body += tool.Baro_hero("결과 확정", row.Season+" · "+row.Name, "")
	body += message
	body += tool.Baro_alert("warn", tool.Baro_t("<b>확정하면 되돌릴 수 없습니다.</b> 시즌이 잠기고 후보·심사·투표·결과가 그대로 영구 보존됩니다."))

	options := [][]string{{"", tool.Baro_t("고르지 않음")}}
	table := [][]string{}

	// 최종 결정을 내리는 화면이므로 후보 옆에 근거 수치를 반드시 함께 둔다 (12-3 3단계)
	close_agg := baro_award_agg_for(db, row.Category, row.Season)

	for index, nom := range list {
		score := baro_award_score_of(scores, nom.ID)

		label := nom.Target + tool.Baro_t(" (최종 ") + baro_award_num_text(score.Final) + tool.Baro_t("점)")
		options = append(options, []string{nom.Target, label})

		table = append(table, []string{
			tool.Baro_itoa(index + 1),
			baro_award_target_html(db, nom.TargetKind, nom.Target, config.IP),
			baro_award_note_html(db, nom.TargetKind, nom.Target, row.Season, close_agg),
			baro_award_score_cell(score),
			tool.HTML_escape(tool.Baro_trunc(nom.Reason, 160)),
		})
	}

	body += tool.Baro_card(tool.Baro_t("현재 순위"), tool.Baro_table(
		[]string{tool.Baro_t("순위"), tool.Baro_t("후보"), tool.Baro_t("기여 데이터 (근거)"), tool.Baro_t("최종 점수"), tool.Baro_t("추천 사유")}, table, tool.Baro_t("후보가 없습니다."),
	))

	if len(list) == 0 {
		body += tool.Baro_button("/baro/award/view/"+tool.Url_parser(row.ID), "부문으로 돌아가기", "")

		return tool.Baro_page(db, config, "결과 확정", body, baro_award_menu())
	}

	form := tool.Baro_select("winner", tool.Baro_t("수상"), options, row.Winner, tool.Baro_t("최종 점수는 참고 자료입니다. 심사 결과에 따라 다르게 고를 수 있으며, 그 경우 근거에 반드시 이유를 적어야 합니다."))
	form += tool.Baro_textarea("result_reason", tool.Baro_t("최종 선정 근거"), row.ResultReason, 8,
		tool.Baro_t("어떤 기여를 왜 인정했는지, 점수와 다르게 골랐다면 그 이유까지 적어 주세요."),
		tool.Baro_t("스무 글자 이상. 이 글은 결과와 함께 그대로 공개됩니다."))
	form += tool.Baro_checkbox("confirm", tool.Baro_t("확정하면 되돌릴 수 없다는 것을 이해했습니다."), "on", false, "")
	form += tool.Baro_submit(tool.Baro_t("결과 확정하고 공개"), "danger")

	body += tool.Baro_danger_zone("결과 확정", tool.Baro_form("/baro/award/close/"+tool.Url_parser(row.ID), form))
	body += tool.Baro_button("/baro/award/view/"+tool.Url_parser(row.ID), "부문으로 돌아가기", "")

	return tool.Baro_page(db, config, "결과 확정", body, baro_award_menu())
}

// ------------------------------------------------------------
//  시즌 운영
// ------------------------------------------------------------

// View_baro_award_manage : /baro/award/manage — 시즌 개설 · 단계 이동 · 가중치
func View_baro_award_manage(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if !tool.Baro_can(db, config.IP, "award") {
		return tool.Baro_deny(db, config, tool.Baro_t("시즌 운영은 어워드 운영 권한(award)이 있는 사람만 할 수 있습니다."))
	}

	message := ""

	// --- 처리 ---
	if values != nil {
		action := values.Get("action")

		if text := baro_award_readonly_msg(db); text != "" {
			message = tool.Baro_alert("warn", tool.HTML_escape(text))
		} else {
			switch action {
			case "open_year":
				year := strings.TrimSpace(values.Get("year"))
				if tool.Get_len(year) != 4 || tool.Str_to_int(year) < 2000 {
					message = tool.Baro_alert("danger", tool.Baro_t("연도를 네 자리로 적어 주세요. (보기 : 2026)"))
					break
				}

				added := 0
				for _, category := range baro_award_year_category_list() {
					if _, exists := baro_award_find(db, year, category[0]); exists {
						continue
					}

					tool.Exec_DB(
						db,
						"insert into baro_award (id, season, category, name, description, status, date, result_reason, winner) values (?, ?, ?, ?, ?, 'open', ?, '', '')",
						tool.Baro_next_id(db, "baro_award"), year, category[0],
						category[1], category[3], tool.Baro_now(),
					)
					added++
				}

				tool.Baro_public_log(db, "award_season", config.IP, year, "연간 어워드 시즌 개설", "부문 "+tool.Baro_itoa(added)+"개")
				tool.Baro_audit(db, config, "award_season_open", "season:"+year, "", "added="+tool.Baro_itoa(added), "시즌 개설")

				return tool.Get_redirect("/baro/award/season/" + tool.Url_parser(year))

			case "stage":
				id := strings.TrimSpace(values.Get("award"))
				status := values.Get("status")

				row, found := baro_award_get(db, id)
				if !found {
					message = tool.Baro_alert("danger", tool.Baro_t("그런 부문이 없습니다."))
					break
				}
				if baro_award_locked(row) {
					message = tool.Baro_alert("danger", tool.Baro_t("확정된 시즌은 되돌릴 수 없습니다."))
					break
				}
				if status != "open" && status != "pending" {
					message = tool.Baro_alert("danger", tool.Baro_t("확정은 결과 확정 화면에서만 할 수 있습니다."))
					break
				}

				tool.Exec_DB(db, "update baro_award set status = ? where id = ?", status, row.ID)
				tool.Baro_public_log(db, "award_stage", config.IP, row.Season+" "+row.Name, baro_award_status_name(status), "")
				tool.Baro_audit(db, config, "award_stage", "baro_award:"+row.ID, row.Status, status, "진행 단계 변경")

				return tool.Get_redirect("/baro/award/manage")

			case "weight":
				judge := tool.Str_to_int(values.Get("judge_weight"))
				max_score := tool.Str_to_int(values.Get("judge_max"))

				if judge < 0 || judge > 100 {
					message = tool.Baro_alert("danger", tool.Baro_t("심사 반영 비율은 0 ~ 100 사이여야 합니다."))
					break
				}
				if max_score < 1 || max_score > 100 {
					message = tool.Baro_alert("danger", tool.Baro_t("심사 최고 점수는 1 ~ 100 사이여야 합니다."))
					break
				}

				before := baro_award_formula_text(db)
				tool.Baro_put(db, "award_judge_weight", tool.Baro_itoa(judge))
				tool.Baro_put(db, "award_judge_max", tool.Baro_itoa(max_score))

				tool.Baro_public_log(db, "award_setting", config.IP, "어워드 점수 공식", baro_award_formula_text(db), "")
				tool.Baro_audit(db, config, "award_setting", "award_weight", before, baro_award_formula_text(db), "점수 공식 변경")

				return tool.Get_redirect("/baro/award/manage")
			}
		}
	}

	// --- 보여주기 ---
	body := baro_award_tabs("/baro/award/manage")
	body += tool.Baro_hero(tool.Baro_t("시즌 운영"), tool.Baro_t("어워드 운영 권한이 있는 사람만 볼 수 있는 화면입니다."), "")
	body += message

	year := baro_award_this_year()
	open_form := tool.Baro_input("year", tool.Baro_t("연도"), year, "2026", tool.Baro_t("그 해의 아홉 부문을 한꺼번에 만듭니다. 이미 있는 부문은 건너뜁니다."))
	open_form += tool.Baro_hidden("action", "open_year")
	open_form += tool.Baro_submit(tool.Baro_t("연간 시즌 열기"), "primary")

	body += tool.Baro_card(tool.Baro_t("연간 시즌 열기"), tool.Baro_form("/baro/award/manage", open_form))

	weight_form := tool.Baro_input_type("judge_weight", tool.Baro_t("심사 반영 비율 (%)"), tool.Baro_itoa(baro_award_judge_weight(db)), "number",
		tool.Baro_t("나머지는 커뮤니티 투표 비율이 됩니다."))
	weight_form += tool.Baro_input_type("judge_max", tool.Baro_t("심사 최고 점수"), tool.Baro_itoa(baro_award_judge_max(db)), "number",
		tool.Baro_t("심사위원이 후보마다 매길 수 있는 최고 점수입니다."))
	weight_form += tool.Baro_hidden("action", "weight")
	weight_form += tool.Baro_submit(tool.Baro_t("공식 저장"), "")

	body += tool.Baro_card_ex(tool.Baro_t("점수 공식"), baro_award_formula_text(db),
		tool.Baro_form("/baro/award/manage", weight_form), "")

	// 진행 중인 부문의 단계 이동
	rows := baro_award_query(
		db,
		"select "+baro_award_cols+" from baro_award where status != 'closed' order by id + 0 desc limit 60",
	)

	table := [][]string{}
	for _, row := range rows {
		stage := tool.Baro_form("/baro/award/manage",
			tool.Baro_hidden("action", "stage")+
				tool.Baro_hidden("award", row.ID)+
				tool.Baro_select("status", "", [][]string{
					{"open", tool.Baro_t("후보 모집 중")},
					{"pending", tool.Baro_t("심사·투표 중")},
				}, row.Status, "")+
				tool.Baro_submit(tool.Baro_t("변경"), ""),
		)

		table = append(table, []string{
			tool.HTML_escape(row.Season),
			`<a href="/baro/award/view/` + tool.Url_parser(row.ID) + `">` + tool.HTML_escape(row.Name) + `</a>`,
			baro_award_status_badge(row.Status),
			tool.Baro_comma(baro_award_nom_count(db, row.ID)) + tool.Baro_t("명"),
			stage,
			tool.Baro_button("/baro/award/close/"+tool.Url_parser(row.ID), "결과 확정", "danger"),
		})
	}

	stage_body := tool.Baro_table([]string{tool.Baro_t("시즌"), "부문", "상태", tool.Baro_t("후보"), tool.Baro_t("단계 이동"), tool.Baro_t("확정")}, table, "")
	if len(table) == 0 {
		stage_body = tool.Baro_empty(tool.Baro_t("진행 중인 부문이 없습니다. 시즌을 먼저 열어 주세요."))
	}

	body += tool.Baro_card(tool.Baro_t("진행 중인 부문"), stage_body)
	body += tool.Baro_card(tool.Baro_t("월간 MVP"), tool.Baro_button("/baro/award/mvp", tool.Baro_t("월간 MVP 집계로 이동"), "primary"))

	return tool.Baro_page(db, config, tool.Baro_t("어워드 시즌 운영"), body, baro_award_menu())
}
