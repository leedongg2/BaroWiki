package route

import (
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  1-3 관심 분야 신청 (/baro/interest)
//  1-4 분야별 창립 모집 (/baro/founding/field)
// ============================================================

// View_baro_founding_interest : 관심 분야 선택 (1-3)
func View_baro_founding_interest(config tool.Config, page string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/interest"); need {
		return wall
	}

	field_list := tool.Baro_field_list(db)

	if values != nil {
		if tool.Baro_read_only(db) {
			return tool.Get_redirect("/baro/interest?done=read_only")
		}

		if !tool.Baro_rate_ok(db, "interest_save", config.IP, 30, 600) {
			return tool.Get_redirect("/baro/interest?done=rate")
		}

		picked := []string{}
		for _, key := range values["field"] {
			key = strings.TrimSpace(key)
			if !baro_founding_field_ok(db, key) {
				continue
			}

			already := false
			for _, old := range picked {
				if old == key {
					already = true
					break
				}
			}

			if !already {
				picked = append(picked, key)
			}
		}

		// 관심 분야는 개인 설정이므로 통째로 다시 저장한다.
		tool.Exec_DB(db, "delete from baro_interest where user = ?", config.IP)
		for _, key := range picked {
			tool.Exec_DB(
				db,
				"insert into baro_interest (user, field, kind, date) values (?, ?, 'pick', ?)",
				config.IP, key, tool.Baro_now(),
			)
		}

		tool.Baro_metric_bump(db, "interest_save", tool.Baro_today(), 1)

		if len(picked) == 0 {
			return tool.Get_redirect("/baro/interest?done=empty")
		}

		return tool.Get_redirect("/baro/interest?done=saved")
	}

	mine := tool.Baro_user_interest(db, config.IP)
	mine_map := map[string]bool{}
	for _, key := range mine {
		mine_map[key] = true
	}

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/founding", tool.Baro_t("창립 시즌")},
		{"", tool.Baro_t("관심 분야")},
	})
	body += tool.Baro_hero(
		tool.Baro_t("관심 분야 신청"),
		tool.Baro_t("관심 있는 분야를 고르면 첫 화면과 추천 과제가 그 분야로 맞춰집니다."),
		"",
	)
	body += baro_founding_tabs("/baro/interest")

	if query != nil {
		switch query.Get("done") {
		case "saved":
			body += tool.Baro_alert("ok",
				tool.Baro_t("관심 분야를 저장했습니다. <b>첫 화면과 추천 과제가 고른 분야로 개인화</b>됩니다. ")+
					tool.Baro_t(`나중에 언제든 이 화면에서 다시 바꿀 수 있습니다.`))
		case "empty":
			body += tool.Baro_alert("warn", tool.Baro_t("고른 분야가 없어 관심 분야를 모두 비웠습니다. 개인화는 꺼집니다."))
		case "rate":
			body += tool.Baro_alert("danger", tool.Baro_t("짧은 시간에 너무 많이 저장했습니다. 잠시 뒤에 다시 시도해 주세요."))
		case "read_only":
			body += tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 모드라 저장할 수 없습니다."))
		}
	}

	form := ""
	for _, field := range field_list {
		help := field.Description
		if field.RecruitOpen && field.Recruit > 0 {
			help += tool.Baro_t(" · 창립 모집 ") + tool.Baro_itoa(field.Recruit) + tool.Baro_t("명")
		}

		applied := baro_founding_field_count(db, field.Key, "")
		help += tool.Baro_t(" · 지원 ") + tool.Baro_itoa(applied) + tool.Baro_t("명")

		form += tool.Baro_checkbox("field", field.Name, field.Key, mine_map[field.Key], help)
	}

	form += tool.Baro_submit(tool.Baro_t("관심 분야 저장"), "primary")

	body += tool.Baro_card_ex(
		tool.Baro_t("관심 분야 고르기 (복수 선택)"),
		tool.Baro_t("여러 개를 골라도 됩니다."),
		tool.Baro_form("/baro/interest", form),
		tool.Baro_note(tool.Baro_t("관심 분야는 공개 프로필에 강제로 표시되지 않으며 언제든 바꿀 수 있습니다.")),
	)

	if len(mine) == 0 {
		body += tool.Baro_card(tool.Baro_t("지금 내 관심 분야"), tool.Baro_empty(tool.Baro_t("아직 고른 분야가 없습니다. 위에서 골라 주세요.")))
	} else {
		names := []string{}
		links := []string{}
		for _, key := range mine {
			names = append(names, tool.Baro_field_name(db, key))
			links = append(links, tool.Baro_button("/baro/founding/field", tool.Baro_field_name(db, key)+tool.Baro_t(" 모집 보기"), ""))
		}

		body += tool.Baro_card_ex(
			tool.Baro_t("지금 내 관심 분야"),
			tool.Baro_itoa(len(mine))+tool.Baro_t("개 분야를 고르셨습니다."),
			tool.Baro_pills(names),
			strings.Join(links, " "),
		)
	}

	body += tool.Baro_card(
		tool.Baro_t("관심 분야를 고르면 달라지는 것"),
		tool.Baro_list([]string{
			tool.Baro_t("첫 화면에 <b>내 분야의 할 일과 프로젝트</b>가 먼저 보입니다."),
			tool.Baro_t("추천 과제가 내 분야 위주로 정렬됩니다."),
			tool.Baro_t("분야별 창립 모집 소식을 먼저 확인할 수 있습니다."),
			tool.Baro_t("창립 기여자 지원 시 고른 분야가 기본값으로 채워집니다."),
		})+
			tool.Baro_note(tool.Baro_t("고른 분야가 곧 그 분야에 대한 권한이나 소유권을 뜻하지는 않습니다.")),
	)

	body += tool.Baro_card(
		tool.Baro_t("다음 단계"),
		tool.Baro_button("/baro/founding/field", tool.Baro_t("분야별 창립 모집 보기"), "primary")+" "+
			tool.Baro_button("/baro/founding/apply", tool.Baro_t("창립 기여자로 지원하기"), "")+" "+
			tool.Baro_button("/baro/task", tool.Baro_t("추천 할 일 보기"), ""),
	)

	return tool.Baro_page(db, config, tool.Baro_t("관심 분야"), body, baro_founding_menu())
}

// View_baro_founding_field : 분야별 창립 모집 (1-4)
func View_baro_founding_field(config tool.Config, page string, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if values != nil {
		if wall, need := tool.Baro_login_wall(db, config, "/baro/founding/field"); need {
			return wall
		}

		message := baro_founding_apply_do(db, config, values.Get("field"), values.Get("note"))
		if message == "" {
			return tool.Get_redirect("/baro/founding/check")
		}

		return tool.Get_redirect("/baro/founding/field?fail=" + tool.Url_parser(message))
	}

	field_list := tool.Baro_field_list(db)
	person, joined := baro_founding_find(db, config.IP)
	logged_in := tool.Baro_is_user(config.IP)

	body := tool.Baro_crumb([][]string{
		{"/baro", "바로위키"},
		{"/baro/founding", tool.Baro_t("창립 시즌")},
		{"", tool.Baro_t("분야별 모집")},
	})
	body += tool.Baro_hero(
		tool.Baro_t("분야별 창립 모집"),
		tool.Baro_t("한 분야를 처음부터 만들 사람을 분야마다 따로 모집합니다."),
		"",
	)
	body += baro_founding_tabs("/baro/founding/field")

	if query != nil && query.Get("fail") != "" {
		body += tool.Baro_alert("danger", tool.HTML_escape(query.Get("fail")))
	}

	body += baro_founding_power_html()

	season_open := baro_founding_open(db)
	if !season_open {
		body += tool.Baro_alert("warn", tool.Baro_t("지금은 창립 기여자 지원을 받지 않습니다. 분야별 모집 현황만 볼 수 있습니다."))
	}

	body += baro_founding_seat_html(db)

	if joined {
		body += tool.Baro_alert("info",
			tool.Baro_t("이미 <b>")+tool.HTML_escape(tool.Baro_field_name(db, person.Field))+tool.Baro_t("</b> 분야로 지원했습니다. (현재 상태 : ")+
				baro_founding_state_html(person)+") "+tool.Baro_button("/baro/founding/check", tool.Baro_t("내 지원 상태"), ""))
	}

	cards := []string{}
	for _, field := range field_list {
		applied := baro_founding_field_count(db, field.Key, "")
		confirmed := baro_founding_field_count(db, field.Key, "confirmed")

		recruit := field.Recruit
		left := recruit - applied
		if left < 0 {
			left = 0
		}

		headline := `<p class="bw_share_t">` + tool.HTML_escape(field.Name) + tool.Baro_t(` 분야를 처음부터 만들 `)
		if recruit > 0 {
			headline += tool.Baro_itoa(recruit) + tool.Baro_t(`명을 모집합니다.</p>`)
		} else {
			headline += tool.Baro_t(`사람을 찾습니다.</p>`)
		}

		inner := headline
		if field.Description != "" {
			inner += tool.Baro_note(field.Description)
		}

		inner += tool.Baro_kv_grid([][]string{
			{tool.Baro_t("모집 인원"), func() string {
				if recruit > 0 {
					return tool.Baro_itoa(recruit) + tool.Baro_t("명")
				}

				return tool.Baro_t(`<span class="bw_muted">상시 모집</span>`)
			}()},
			{tool.Baro_t("지금 지원자"), tool.Baro_itoa(applied) + tool.Baro_t("명")},
			{tool.Baro_t("번호 확정"), tool.Baro_itoa(confirmed) + tool.Baro_t("명")},
			{tool.Baro_t("남은 자리"), func() string {
				if recruit > 0 {
					return tool.Baro_itoa(left) + tool.Baro_t("명")
				}

				return tool.Baro_t(`<span class="bw_muted">제한 없음</span>`)
			}()},
		})

		if recruit > 0 {
			inner += tool.Baro_progress(applied, recruit, field.Name+tool.Baro_t(" 모집"))
		} else {
			inner += tool.Baro_progress_open(applied, 0, field.Name+tool.Baro_t(" 모집"))
		}

		if field.TargetCount > 0 {
			inner += tool.Baro_note(tool.Baro_t("이 분야는 전체 목록이 확정된 분야입니다. 목표 문서 수 ") + tool.Baro_comma(field.TargetCount) + tool.Baro_t("개"))
		}

		foot := ""
		switch {
		case !season_open:
			foot = tool.Baro_badge(tool.Baro_t("창립 지원 중단"), "muted")
		case !field.RecruitOpen:
			foot = tool.Baro_badge(tool.Baro_t("모집 마감"), "muted")
		case joined:
			foot = tool.Baro_button("/baro/founding/check", tool.Baro_t("내 지원 상태 보기"), "")
		case !logged_in:
			foot = tool.Baro_button("/login", tool.Baro_t("로그인하고 지원하기"), "primary") + " " + tool.Baro_button("/register", tool.Baro_t("가입하기"), "")
		default:
			apply_form := tool.Baro_hidden("field", field.Key)
			apply_form += tool.Baro_hidden("agree", "on")
			apply_form += tool.Baro_t(`<input class="bw_in" name="note" placeholder="함께 만들고 싶은 것 (선택)">`)
			apply_form += tool.Baro_submit(field.Name+tool.Baro_t(" 분야로 지원하기"), "primary")
			foot = tool.Baro_form("/baro/founding/field", apply_form)
		}

		cards = append(cards, tool.Baro_card_ex(field.Name, tool.Baro_t("분야 개척 모집"), inner, foot))
	}

	if len(cards) == 0 {
		body += tool.Baro_card(tool.Baro_t("모집 중인 분야"), tool.Baro_empty(tool.Baro_t("등록된 분야가 없습니다.")))
	} else {
		body += tool.Baro_grid(cards, 2)
	}

	body += tool.Baro_card(
		tool.Baro_t("분야 개척자가 하는 일"),
		tool.Baro_list([]string{
			tool.Baro_t("그 분야의 <b>기본 문서 틀</b>을 세웁니다. (어떤 항목을 어떤 순서로 적을지)"),
			tool.Baro_t("믿을 만한 <b>출처 목록</b>을 모아 다른 사람이 따라 쓸 수 있게 합니다."),
			tool.Baro_t("<b>할 일 목록</b>을 만들어 새로 온 사람이 무엇부터 할지 알 수 있게 합니다."),
			tool.Baro_t("먼저 온 사람으로서 <b>질문에 답하고 검토를 돕습니다</b>."),
		})+
			tool.Baro_note(tool.Baro_t("여기에 문서 잠금·다른 사람 편집 차단·분야 운영권은 포함되지 않습니다.")),
	)

	body += tool.Baro_card(
		tool.Baro_t("함께 보기"),
		tool.Baro_button("/baro/interest", tool.Baro_t("관심 분야 고르기"), "")+" "+
			tool.Baro_button("/baro/founding/apply", tool.Baro_t("창립 기여자로 지원하기"), "primary")+" "+
			tool.Baro_button("/baro/founding/list", tool.Baro_t("창립단 명단"), ""),
	)

	return tool.Baro_page(db, config, tool.Baro_t("분야별 창립 모집"), body, baro_founding_menu())
}
