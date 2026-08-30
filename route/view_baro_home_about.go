package route

import (
	"database/sql"
	"net/url"

	"opennamu/route/tool"
)

// ============================================================
//  바로위키 소개 (/baro/about)
//
//  0장 고정 원칙 5가지 전문을 baro_policy 에서 읽어 보여준다.
//  (코드에 원칙 문구를 박아 넣지 않는다)
// ============================================================

// View_baro_home_about : 바로위키 소개 + 0장 고정 원칙 전문
func View_baro_home_about(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = values

	body := baro_home_notice(query)
	body += baro_home_about_hero(db)
	body += baro_home_about_what(db)
	body += baro_home_about_principle(db)
	body += baro_home_about_policy_list(db)
	body += baro_home_about_promise(db)

	return tool.Baro_page(db, config, tool.Baro_t("바로위키 소개"), body, baro_home_menu())
}

// baro_home_about_hero : 소개 머리말
func baro_home_about_hero(db *sql.DB) string {
	name := baro_home_wiki_name(db)

	doc_total := baro_home_doc_total(db)
	citation_total := baro_home_count(db, "select count(*) from baro_citation")
	founder := baro_home_founder_confirmed(db)

	buttons := tool.Baro_button("/baro/founding", tool.Baro_t("창립 시즌"), "primary") + " " +
		tool.Baro_button("/baro/policy", tool.Baro_t("정책 전체"), "") + " " +
		tool.Baro_button("/baro/transparency", tool.Baro_t("투명성 보고서"), "") + " " +
		tool.Baro_button("/baro/help", tool.Baro_t("도움말"), "")

	return tool.Baro_hero(
		name+tool.Baro_t(" 소개"),
		tool.Baro_t("지우기보다 남기기. 주장보다 출처. 결정보다 근거 공개."),
		tool.Baro_stat_row([][]string{
			{"문서", tool.Baro_comma(doc_total) + tool.Baro_t("개"), tool.Baro_t("일반 이름공간")},
			{"출처", tool.Baro_comma(citation_total) + tool.Baro_t("개"), tool.Baro_t("문서에 달린 근거")},
			{tool.Baro_t("창립 기여자"), tool.Baro_comma(founder) + tool.Baro_t("명"), tool.Baro_t("창립 시즌")},
			{tool.Baro_t("확장 버전"), tool.Baro_ver, tool.Baro_t("바로위키 기능 판")},
		})+`<div>`+buttons+`</div>`,
	)
}

// baro_home_about_what : 바로위키가 무엇인지
func baro_home_about_what(db *sql.DB) string {
	name := tool.HTML_escape(baro_home_wiki_name(db))
	license := tool.HTML_escape(tool.Baro_get_def(db, "license", "CC BY-SA 4.0"))

	body := `<p>` + name + tool.Baro_t(`는 한국어로 된 주제를 출처와 함께 정리하는 열린 위키입니다. `) +
		tool.Baro_t(`누구나 읽을 수 있고, 로그인한 사람은 누구나 고칠 수 있습니다.</p>`)

	body += tool.Baro_grid([]string{
		tool.Baro_card(tool.Baro_t("무엇을 다루나요"),
			tool.Baro_t(`<p>아직 아무도 정리하지 않은 주제를 먼저 채웁니다. 독립운동가 기록, 지역사, 언론사, 철도, `)+
				tool.Baro_t(`야구 기록처럼 흩어져 있는 자료를 한곳에 모읍니다.</p>`)+
				tool.Baro_button("/baro/field", tool.Baro_t("분야별 현황 보기"), "")),
		tool.Baro_card(tool.Baro_t("어떻게 다른가요"),
			tool.Baro_t(`<p>문서를 지우는 대신 격리하고, 격리한 이유를 공개합니다. 제재는 사람에게만 적용하고 `)+
				tool.Baro_t(`그 사람의 정상적인 문서는 건드리지 않습니다.</p>`)+
				tool.Baro_button("/baro/log", tool.Baro_t("공개 운영 로그"), "")),
		tool.Baro_card(tool.Baro_t("기여는 어떻게 인정되나요"),
			tool.Baro_t(`<p>편집 횟수를 그대로 점수로 쓰지 않습니다. 새 문서 · 출처 추가 · 오류 수정 · 검토를 나누어 기록하고, `)+
				tool.Baro_t(`살아남은 기여를 뒤늦게 확정합니다.</p>`)+
				tool.Baro_button("/baro/rank", tool.Baro_t("기여 인정 방식"), "")),
		tool.Baro_card(tool.Baro_t("저작권은 어떻게 되나요"),
			tool.Baro_t(`<p>모든 기여는 `)+license+tool.Baro_t(` 로 공개됩니다. 출처를 밝히면 누구나 다시 쓸 수 있습니다. `)+
				tool.Baro_t(`저작권을 침해하는 내용은 게시할 수 없습니다.</p>`)+
				tool.Baro_button("/baro/copyright", tool.Baro_t("저작권 안내"), "")),
	}, 2)

	return tool.Baro_card(tool.Baro_t("바로위키란"), body)
}

// baro_home_about_principle : 0장 고정 원칙 전문
func baro_home_about_principle(db *sql.DB) string {
	row := baro_home_policy_get(db, "principle")

	if !row.Found {
		return tool.Baro_card(
			tool.Baro_t("0장 고정 원칙"),
			tool.Baro_empty(tool.Baro_t("원칙 문서를 아직 찾을 수 없습니다. 관리자가 정책을 등록하면 이곳에 전문이 표시됩니다.")),
		)
	}

	items := baro_home_policy_items(row.Data)

	body := tool.Baro_alert("info",
		tool.Baro_t("아래 원칙은 바로위키의 바탕입니다. 개정하려면 정책 개정 절차를 거쳐야 하며, 개정 이력은 모두 공개됩니다."))

	if len(items) > 0 {
		body += baro_home_policy_cards(items, 1)
	}

	body += tool.Baro_section(tool.Baro_t("전문"), baro_home_text_html(row.Data))

	foot := tool.Baro_kv_grid([][]string{
		{tool.Baro_t("판 번호"), tool.Baro_t("제") + tool.HTML_escape(row.Version) + tool.Baro_t("판")},
		{"시행일", tool.HTML_escape(row.Date)},
		{tool.Baro_t("제안자"), tool.HTML_escape(row.Proposer)},
	})
	foot += tool.Baro_button("/baro/policy", tool.Baro_t("개정 이력 보기"), "")

	return tool.Baro_card_ex(
		tool.Baro_t("0장 고정 원칙 — ")+row.Title,
		tool.Baro_t("바로위키가 코드로도 지키는 다섯 가지"),
		body,
		foot,
	)
}

// baro_home_about_policy_list : 등록된 정책 목록
func baro_home_about_policy_list(db *sql.DB) string {
	rows := [][]string{}

	list_rows := tool.Query_DB(
		db,
		"select key_name, title, version, effective_date, status from baro_policy order by key_name, version + 0 desc",
	)
	defer list_rows.Close()

	seen := map[string]bool{}
	for list_rows.Next() {
		key_name, title, version, date, status := "", "", "", "", ""
		if err := list_rows.Scan(&key_name, &title, &version, &date, &status); err != nil {
			continue
		}

		if seen[key_name] {
			continue
		}
		seen[key_name] = true

		kind := "muted"
		state := status
		switch status {
		case "active":
			kind, state = "ok", tool.Baro_t("시행 중")
		case "draft", "pending":
			kind, state = "warn", tool.Baro_t("제안")
		case "closed", "expired":
			kind, state = "muted", tool.Baro_t("폐지")
		}

		rows = append(rows, []string{
			`<a href="/baro/policy/view/` + tool.Url_parser(key_name) + `">` + tool.HTML_escape(title) + `</a>`,
			tool.Baro_t(`제`) + tool.HTML_escape(version) + tool.Baro_t(`판`),
			tool.HTML_escape(date),
			tool.Baro_badge(state, kind),
		})
	}

	body := tool.Baro_table(
		[]string{"정책", "판", "시행일", "상태"},
		rows,
		tool.Baro_t("등록된 정책이 없습니다."),
	)

	body += tool.Baro_note(tool.Baro_t("각 정책은 개정 이력이 판 번호로 남고, 바뀐 내용은 정책 화면에서 비교할 수 있습니다."))
	body += tool.Baro_button("/baro/policy", tool.Baro_t("정책 전체 보기"), "")

	return tool.Baro_card(tool.Baro_t("정책 목록"), body)
}

// baro_home_about_promise : 운영 약속과 연락처
func baro_home_about_promise(db *sql.DB) string {
	threshold := tool.Baro_mass_threshold(db)
	grace := tool.Baro_mass_grace_hours(db)
	retention := tool.Baro_get_int(db, "log_retention_days", 90)
	appeal := tool.Baro_get_int(db, "appeal_deadline_days", 14)

	body := tool.Baro_dl([][]string{
		{tool.Baro_t("대량 작업 기준"), tool.HTML_escape(tool.Baro_itoa(threshold)) + tool.Baro_t("개를 넘는 작업은 2인 승인과 스냅샷을 거칩니다.")},
		{tool.Baro_t("대량 작업 유예"), tool.Baro_t("기본 ") + tool.HTML_escape(tool.Baro_itoa(grace)) + tool.Baro_t("시간 뒤에 실행합니다. 그 사이에 누구나 이의를 낼 수 있습니다.")},
		{tool.Baro_t("이의제기 기한"), tool.Baro_t("조치일로부터 ") + tool.HTML_escape(tool.Baro_itoa(appeal)) + tool.Baro_t("일 안에 신청할 수 있고, 원조치 담당자는 심사에서 빠집니다.")},
		{tool.Baro_t("운영 기록 보관"), tool.Baro_t("접속 기록 등 운영 데이터는 ") + tool.HTML_escape(tool.Baro_itoa(retention)) + tool.Baro_t("일 뒤 삭제합니다.")},
	})

	body += tool.Baro_section(tool.Baro_t("도움이 필요할 때"), tool.Baro_list([]string{
		tool.Baro_button("/baro/report", tool.Baro_t("문제 신고하기"), "") + tool.Baro_t(" 문서·이용자 문제를 알려 주세요."),
		tool.Baro_button("/baro/appeal", tool.Baro_t("이의제기"), "") + tool.Baro_t(" 받은 조치가 부당하다고 생각되면 신청하세요."),
		tool.Baro_button("/baro/privacy", tool.Baro_t("개인정보 요청"), "") + tool.Baro_t(" 열람·수정·삭제를 신청할 수 있습니다."),
		tool.Baro_button("/baro/copyright", tool.Baro_t("저작권 신고"), "") + tool.Baro_t(" 권리 침해를 신고할 수 있습니다."),
		tool.Baro_button("/baro/status", tool.Baro_t("서비스 상태"), "") + tool.Baro_t(" 장애와 점검 안내를 확인할 수 있습니다."),
	}))

	return tool.Baro_card(tool.Baro_t("운영 약속"), body)
}
