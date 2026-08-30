package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  1-1 창립 시즌 랜딩 (/baro/founding)
//
//  "바로위키가 왜 존재하는가" 를 한 화면에서 보여주고
//  창립 기여자 지원 · 출시 알림 신청으로 이어 준다.
// ============================================================

// View_baro_home_founding : 창립 시즌 랜딩
//
//	values 가 nil 이 아니면 POST 처리 (출시 알림 신청)
func View_baro_home_founding(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if values != nil {
		return baro_home_founding_post(db, config, values)
	}

	body := baro_home_notice(query)
	body += baro_home_founding_hero(db, config)
	body += baro_home_founding_purpose(db)
	body += baro_home_founding_principle(db)
	body += baro_home_founding_progress(db)
	body += baro_home_founding_fields(db)
	body += baro_home_founding_projects(db)
	body += baro_home_founding_docs(db, query)
	body += baro_home_founding_join(db, config)

	return tool.Baro_page(db, config, tool.Baro_t("창립 시즌 · ")+baro_home_wiki_name(db), body, baro_home_menu())
}

// ------------------------------------------------------------
//  머리말
// ------------------------------------------------------------

// baro_home_founding_hero : 머리말 + 지원 버튼
func baro_home_founding_hero(db *sql.DB, config tool.Config) string {
	name := baro_home_wiki_name(db)

	confirmed := baro_home_founder_confirmed(db)
	goal := baro_home_founding_goal(db)

	buttons := tool.Baro_button("/baro/founding/apply", tool.Baro_t("창립 기여자 지원하기"), "primary")
	buttons += " " + tool.Baro_button("/baro", tool.Baro_t("바로위키 홈"), "")
	buttons += " " + tool.Baro_button("/baro/about", tool.Baro_t("0장 고정 원칙 보기"), "")

	if tool.Baro_is_user(config.IP) {
		buttons += " " + tool.Baro_button("/baro/invite", tool.Baro_t("친구 초대 링크 받기"), "")
	}

	open := tool.Baro_get_on(db, "founding_open")
	state := tool.Baro_badge(tool.Baro_t("창립 시즌 진행 중"), "ok")
	if !open {
		state = tool.Baro_badge(tool.Baro_t("창립 기여자 모집 마감"), "muted")
	}

	sub := tool.Baro_t("지금 ") + tool.Baro_comma(confirmed) + tool.Baro_t("명이 함께하고 있습니다. 창립 기여자는 ") + tool.Baro_comma(goal) + tool.Baro_t("명까지 모십니다.")

	return tool.Baro_hero(
		name+tool.Baro_t(" 창립 시즌"),
		sub,
		`<p>`+state+`</p>`+
			`<p>`+tool.HTML_escape(tool.Baro_t("기록이 남지 않아 사라지는 것들이 있습니다. ")+name+tool.Baro_t("는 그 기록을 남기려고 시작합니다."))+`</p>`+
			`<div>`+buttons+`</div>`,
	)
}

// baro_home_founding_purpose : 바로위키의 목적
func baro_home_founding_purpose(db *sql.DB) string {
	name := tool.HTML_escape(baro_home_wiki_name(db))
	license := tool.HTML_escape(tool.Baro_get_def(db, "license", "CC BY-SA 4.0"))

	body := `<p>` + name + tool.Baro_t(`는 "빨리 지우는 위키" 가 아니라 "제대로 남기는 위키" 를 목표로 합니다. `) +
		tool.Baro_t(`문서가 마음에 들지 않는다는 이유로 지우는 대신, 출처를 붙이고 보강해서 남깁니다.</p>`)

	body += tool.Baro_grid([]string{
		tool.Baro_card(tool.Baro_t("① 기록이 먼저입니다"),
			tool.Baro_t(`<p>독립운동가, 지역사, 사라진 언론사처럼 아무도 정리해 두지 않은 주제부터 채웁니다. `)+
				tool.Baro_t(`분야마다 "무엇을 얼마나 채웠는지" 를 숫자로 보여 주고, 남은 몫을 나눕니다.</p>`)),
		tool.Baro_card(tool.Baro_t("② 출처가 곧 신뢰입니다"),
			tool.Baro_t(`<p>문장마다 어디서 온 이야기인지 밝힙니다. 출처를 다는 일 자체를 기여로 인정하고, `)+
				tool.Baro_t(`출처 없는 문서는 "출처 필요" 상태로 눈에 띄게 표시합니다.</p>`)),
		tool.Baro_card(tool.Baro_t("③ 운영 근거를 공개합니다"),
			tool.Baro_t(`<p>어떤 문서를 왜 격리했는지, 어떤 제재를 무슨 규정으로 내렸는지 공개 로그에 남깁니다. `)+
				tool.Baro_t(`다만 IP·이메일·신고 원문 같은 개인정보는 공개하지 않습니다.</p>`)),
		tool.Baro_card(tool.Baro_t("④ 기여를 사람에게 돌려줍니다"),
			tool.Baro_t(`<p>편집 횟수를 그대로 점수로 쓰지 않습니다. 새 문서, 출처 추가, 오류 수정, 검토를 나누어 기록하고 `)+
				tool.Baro_t(`기여자 이름과 창립 번호를 함께 남깁니다.</p>`)),
	}, 2)

	body += tool.Baro_note(tool.Baro_t("모든 기여는 ") + license + " 로 공개됩니다.")

	return tool.Baro_card(tool.Baro_t("바로위키를 만드는 이유"), body)
}

// baro_home_founding_principle : 3대 원칙 (baro_policy 의 principle 최신 판)
func baro_home_founding_principle(db *sql.DB) string {
	row := baro_home_policy_get(db, "principle")

	if !row.Found {
		return tool.Baro_card(
			tool.Baro_t("바로위키의 원칙"),
			tool.Baro_empty(tool.Baro_t("원칙 문서가 아직 등록되지 않았습니다. 관리자가 정책을 등록하면 이곳에 표시됩니다.")),
		)
	}

	items := baro_home_policy_items(row.Data)

	head := []string{}
	rest := []string{}
	for i, item := range items {
		if i < 3 {
			head = append(head, item[0])
		} else {
			rest = append(rest, item[0])
		}
	}

	body := ""
	if len(head) > 0 {
		body += baro_home_policy_cards(items[:len(head)], 3)
	} else {
		body += baro_home_text_html(row.Data)
	}

	if len(rest) > 0 {
		lines := []string{}
		for _, title := range rest {
			lines = append(lines, tool.HTML_escape(title))
		}

		body += tool.Baro_section(tool.Baro_t("이어지는 원칙"), tool.Baro_list(lines))
	}

	foot := tool.Baro_button("/baro/about", tool.Baro_t("고정 원칙 전문 보기"), "") + " " +
		tool.Baro_button("/baro/policy", tool.Baro_t("정책 전체 보기"), "")

	return tool.Baro_card_ex(
		tool.Baro_t("바로위키의 원칙 — ")+row.Title,
		tool.Baro_t("제")+row.Version+tool.Baro_t("판 · 시행 ")+row.Date,
		body,
		foot,
	)
}

// ------------------------------------------------------------
//  모집 현황
// ------------------------------------------------------------

// baro_home_founding_progress : 창립 기여자 모집 현황
func baro_home_founding_progress(db *sql.DB) string {
	confirmed := baro_home_founder_confirmed(db)
	pending := baro_home_founder_pending(db)
	goal := baro_home_founding_goal(db)

	body := tool.Baro_t(`<p class="bw_hero_s">창립 기여자 `) + tool.Baro_comma(confirmed) + ` / ` + tool.Baro_comma(goal) + tool.Baro_t(`명</p>`)
	body += tool.Baro_progress(confirmed, goal, tool.Baro_t("창립 기여자 모집"))

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("확정된 창립 기여자"), tool.Baro_comma(confirmed) + tool.Baro_t("명"), tool.Baro_t("첫 유효 기여를 마친 사람")},
		{tool.Baro_t("심사 중인 지원"), tool.Baro_comma(pending) + tool.Baro_t("명"), tool.Baro_t("지원서 검토 대기")},
		{tool.Baro_t("남은 자리"), tool.Baro_comma(baro_home_max(goal-confirmed, 0)) + tool.Baro_t("명"), tool.Baro_t("목표 ") + tool.Baro_comma(goal) + tool.Baro_t("명 기준")},
		{tool.Baro_t("달성률"), tool.Baro_pct_text(confirmed, goal), tool.Baro_t("창립 시즌 목표 대비")},
	})

	recent := baro_home_founder_recent(db, 10)
	rows := [][]string{}
	for _, row := range recent {
		rows = append(rows, []string{
			`<strong>#` + tool.HTML_escape(row[1]) + `</strong>`,
			tool.Baro_user_link(db, row[0], ""),
			tool.HTML_escape(tool.Baro_field_name(db, row[2])),
			tool.Baro_time_cell(row[3]),
		})
	}

	body += tool.Baro_section(
		tool.Baro_t("최근 확정된 창립 기여자"),
		tool.Baro_table(
			[]string{"번호", tool.Baro_t("기여자"), "분야", tool.Baro_t("확정")},
			rows,
			tool.Baro_t("아직 확정된 창립 기여자가 없습니다. 첫 번째 번호가 비어 있습니다."),
		),
	)

	return tool.Baro_card(tool.Baro_t("창립 기여자 모집 현황"), body)
}

// baro_home_max : 두 수 중 큰 값
func baro_home_max(a int, b int) int {
	if a > b {
		return a
	}

	return b
}

// baro_home_founding_fields : 분야별 모집 현황
func baro_home_founding_fields(db *sql.DB) string {
	counts := baro_home_founder_by_field(db)

	cards := []string{}
	total_open := 0

	for _, field := range tool.Baro_field_list(db) {
		if !field.RecruitOpen {
			continue
		}

		total_open++
		count := counts[field.Key]

		inner := `<p>` + tool.HTML_escape(field.Name) + tool.Baro_t(` 분야 <strong>`) + tool.Baro_comma(count) + tool.Baro_t(`명</strong></p>`)
		if field.Description != "" {
			inner += tool.Baro_note(field.Description)
		}

		if field.Recruit > 0 {
			inner += tool.Baro_progress(count, field.Recruit, tool.Baro_t("모집 목표"))
		} else {
			inner += tool.Baro_note(tool.Baro_t("모집 인원을 따로 정하지 않은 분야입니다."))
		}

		if field.Kind == "closed" && field.TargetCount > 0 {
			inner += tool.Baro_note(tool.Baro_t("전체 대상 ") + tool.Baro_comma(field.TargetCount) + tool.Baro_t("건이 확정된 분야라 개척률을 계산할 수 있습니다."))
		}

		inner += tool.Baro_button("/baro/field/view/"+tool.Url_parser(field.Key), "분야 살펴보기", "")

		cards = append(cards, tool.Baro_card(field.Name+" 분야", inner))
	}

	body := ""
	if len(cards) == 0 {
		body = tool.Baro_empty(tool.Baro_t("지금 모집 중인 분야가 없습니다. 잠시 뒤에 다시 확인해 주세요."))
	} else {
		body = tool.Baro_grid(cards, 3)
	}

	body += tool.Baro_note(tool.Baro_t("분야는 ") + tool.Baro_comma(total_open) + tool.Baro_t("개가 열려 있습니다. 관심 분야는 지원할 때 고를 수 있습니다."))

	return tool.Baro_card(tool.Baro_t("분야별 모집 현황"), body)
}

// baro_home_founding_projects : 준비 중인 프로젝트
func baro_home_founding_projects(db *sql.DB) string {
	projects := baro_home_projects(db, 6)

	body := ""
	if len(projects) == 0 {
		body = tool.Baro_empty(tool.Baro_t("아직 공개된 프로젝트가 없습니다. 창립 기여자와 함께 첫 프로젝트를 정합니다."))
	} else {
		cards := []string{}
		for _, row := range projects {
			cards = append(cards, baro_home_project_card(db, row))
		}

		body = tool.Baro_grid(cards, 3)
	}

	body += tool.Baro_button("/baro/project", tool.Baro_t("프로젝트 전체 보기"), "") + " " +
		tool.Baro_button("/baro/task", tool.Baro_t("할 일 목록 보기"), "")

	return tool.Baro_card(tool.Baro_t("현재 준비 중인 프로젝트"), body)
}

// baro_home_founding_docs : 대표 문서 미리보기
func baro_home_founding_docs(db *sql.DB, query url.Values) string {
	page := baro_home_page_num(query, "doc_page")
	size := 6

	// 한 개 더 읽어 다음 쪽이 실제로 있는지 확인한다
	docs := baro_home_recent_docs(db, (page-1)*size, size+1, true)

	more := len(docs) > size
	if more {
		docs = docs[:size]
	}

	body := ""
	if len(docs) == 0 {
		body = tool.Baro_empty(tool.Baro_t("아직 보여 드릴 문서가 없습니다. 창립 기여자의 첫 문서가 이 자리에 놓입니다."))
	} else {
		cards := []string{}
		for _, doc := range docs {
			inner := ""
			if doc[2] != "" {
				inner += `<p>` + tool.HTML_escape(doc[2]) + `</p>`
			} else {
				inner += tool.Baro_note(tool.Baro_t("발췌할 본문이 아직 짧습니다."))
			}

			status := tool.Baro_doc_status_get(db, doc[0])
			if status != "" {
				inner += `<p>` + tool.Baro_status_badge(status) + `</p>`
			}

			inner += tool.Baro_note(tool.Baro_t("만든 시각 ") + doc[1])
			inner += tool.Baro_button("/w/"+tool.Url_parser(doc[0]), "문서 읽기", "")

			cards = append(cards, tool.Baro_card(tool.Baro_trunc(tool.Namespace_display(doc[0]), 40), inner))
		}

		body = tool.Baro_grid(cards, 3)
	}

	count := 0
	if more {
		count = size
	}

	body += tool.Baro_paging(page, count, size, "/baro/founding?doc_page={}")

	return tool.Baro_card(tool.Baro_t("대표 문서 미리보기"), body)
}

// ------------------------------------------------------------
//  참여 (지원 / 알림 / 초대)
// ------------------------------------------------------------

// baro_home_founding_join : 지원 · 출시 알림 · 초대
func baro_home_founding_join(db *sql.DB, config tool.Config) string {
	cards := []string{}

	// 창립 기여자 지원
	apply_body := tool.Baro_t(`<p>문서를 한 편 쓰거나, 출처를 붙이거나, 잘못된 사실을 고치면 창립 기여자가 됩니다. `) +
		tool.Baro_t(`첫 유효 기여가 확인되면 창립 번호가 발급됩니다.</p>`)

	if number := tool.Baro_founder_number(db, config.IP); number != "" {
		apply_body += tool.Baro_alert("ok", tool.Baro_t("이미 창립 기여자입니다. 창립 번호 <strong>#")+tool.HTML_escape(number)+"</strong>")
		apply_body += tool.Baro_button("/baro/profile/"+tool.Url_parser(config.IP), "내 기여 보기", "primary")
	} else if !tool.Baro_get_on(db, "founding_open") {
		apply_body += tool.Baro_alert("warn", tool.Baro_t("창립 기여자 모집이 마감됐습니다. 가입은 계속 열려 있습니다."))
		apply_body += tool.Baro_button("/register", tool.Baro_t("가입하기"), "")
	} else {
		apply_body += tool.Baro_button("/baro/founding/apply", tool.Baro_t("창립 기여자 지원하기"), "primary")
		apply_body += " " + tool.Baro_button("/baro/onboarding", tool.Baro_t("무엇부터 할지 보기"), "")
	}

	cards = append(cards, tool.Baro_card(tool.Baro_t("창립 기여자로 참여하기"), apply_body))

	// 출시 알림 신청
	form := tool.Baro_hidden("do", "notify_mail")
	form += tool.Baro_input_type("mail", tool.Baro_t("이메일"), "", "email", tool.Baro_t("출시 안내 한 번만 보냅니다. 광고에 쓰지 않습니다."))
	form += tool.Baro_submit(tool.Baro_t("출시 알림 받기"), "primary")

	mail_body := tool.Baro_t(`<p>지금 당장 기여하기 어렵다면 이메일만 남겨 두세요. 바로위키가 문을 열 때 알려 드립니다.</p>`)
	mail_body += tool.Baro_form("/baro/founding", form)
	mail_body += tool.Baro_note(tool.Baro_t("현재 ") + tool.Baro_comma(baro_home_mail_count(db)) + tool.Baro_t("명이 알림을 신청했습니다. 로그인하지 않아도 신청할 수 있습니다."))
	mail_body += tool.Baro_note(tool.Baro_t("이메일은 중복 없이 한 번만 저장하며 공개 화면에 표시하지 않습니다."))

	cards = append(cards, tool.Baro_card("출시 알림 신청", mail_body))

	// 친구 초대
	invite_body := ""
	if tool.Baro_is_user(config.IP) {
		invite_body = tool.Baro_t(`<p>초대 링크로 들어온 친구가 첫 기여를 마치면 두 사람 모두 창립 기록에 남습니다.</p>`)
		invite_body += tool.Baro_button("/baro/invite", tool.Baro_t("친구 초대 링크 받기"), "primary")
		invite_body += " " + tool.Baro_button("/baro/card", tool.Baro_t("공유 카드 만들기"), "")
	} else {
		invite_body = tool.Baro_t(`<p>로그인하면 나만의 초대 링크를 만들 수 있습니다.</p>`)
		invite_body += tool.Baro_button("/login", tool.Baro_t("로그인"), "") + " " + tool.Baro_button("/register", tool.Baro_t("가입하기"), "")
	}

	cards = append(cards, tool.Baro_card(tool.Baro_t("친구 초대"), invite_body))

	return tool.Baro_grid(cards, 3)
}

// ------------------------------------------------------------
//  POST 처리
// ------------------------------------------------------------

// baro_home_founding_post : 출시 알림 신청 처리 (비로그인도 가능)
func baro_home_founding_post(db *sql.DB, config tool.Config, values url.Values) string {
	if values.Get("do") != "notify_mail" {
		return tool.Get_redirect("/baro/founding?err=unknown")
	}

	if tool.Baro_read_only(db) {
		return tool.Get_redirect("/baro/founding?err=read_only")
	}

	mail := strings.TrimSpace(values.Get("mail"))
	if !baro_home_mail_valid(mail) {
		// 형식이 틀린 것은 요청 횟수로 세지 않는다 (오타 때문에 막히지 않도록)
		return tool.Get_redirect("/baro/founding?err=mail_form")
	}

	// 같은 곳에서 반복 신청하는 것을 막는다 (1시간에 5번)
	if !tool.Baro_rate_ok(db, "founding_mail", config.IP, 5, 3600) {
		return tool.Get_redirect("/baro/founding?err=mail_rate")
	}

	if baro_home_mail_exists(db, mail) {
		return tool.Get_redirect("/baro/founding?ok=mail_dup")
	}

	baro_home_mail_save(db, mail, config.IP)
	tool.Baro_metric_bump(db, "founding_mail", tool.Baro_today(), 1)

	// 공개 기록에는 개인정보를 그대로 남기지 않는다 (0장 5번 원칙)
	tool.Baro_public_log(db, "founding_mail", "system", "", "출시 알림 신청 접수", "누적 "+tool.Baro_itoa(baro_home_mail_count(db))+"건")
	tool.Baro_audit(db, config, "founding_mail_add", baro_home_mail_mask(mail), "", "", "출시 알림 신청")

	// 로그인 상태라면 확인 알림을 보내 준다
	if tool.Baro_is_user(config.IP) {
		tool.Baro_notify(db, config.IP, "system", "출시 알림을 신청했습니다.", "/baro/founding", "")
	}

	return tool.Get_redirect("/baro/founding?ok=mail")
}
