package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  2-1 계정 시스템 보강
// ============================================================

// View_baro_account_me : 내 계정 화면 (/baro/account)
func View_baro_account_me(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/account"); need {
		return wall
	}

	user := config.IP

	// --- 쓰기 처리 (약관 재동의) ---
	if values != nil {
		if tool.Baro_read_only(db) {
			return tool.Baro_page(db, config, tool.Baro_t("내 계정"),
				tool.Baro_alert("warn", tool.Baro_t("지금은 읽기 전용 상태입니다. ")+tool.HTML_escape(tool.Baro_read_only_reason(db))),
				baro_account_menu())
		}

		kind := strings.TrimSpace(values.Get("kind"))

		// 정해진 정책 종류만 받는다
		known := false
		for _, row := range baro_account_policy_kind_list() {
			if row[0] == kind {
				known = true

				break
			}
		}

		if values.Get("action") == "consent" && known {
			if values.Get("agree") != "on" {
				body := tool.Baro_alert("warn", tool.Baro_t("동의 확인란을 체크해야 기록할 수 있습니다."))
				body += tool.Baro_button("/baro/account", "돌아가기", "primary")

				return tool.Baro_page(db, config, tool.Baro_t("내 계정"), body, baro_account_menu())
			}

			policy := baro_account_policy(db, kind)
			baro_account_consent_add(db, config, kind, policy[0])
			tool.Baro_audit(db, config, "consent_agree", kind, "", policy[0], "약관 재동의")
		}

		return tool.Get_redirect("/baro/account")
	}

	// --- 계정 개요 ---
	join_date := baro_account_join_date(db, user)
	verified, masked_email := baro_account_email_verified(db, user)
	founder := tool.Baro_founder_number(db, user)
	last_active := baro_account_last_active(db, user)
	recent := baro_account_recent_count(db, user, 30)
	total := baro_account_contrib_count(db, user)

	email_html := tool.Baro_badge(tool.Baro_t("미등록"), "warn") +
		tool.Baro_t(` <span class="bw_muted">이메일이 없으면 비밀번호를 되찾을 수 없습니다.</span> `) +
		tool.Baro_button("/change/email", tool.Baro_t("이메일 등록"), "")
	if verified {
		email_html = tool.Baro_badge(tool.Baro_t("인증됨"), "ok") + " " + tool.HTML_escape(masked_email)
	}

	founder_html := tool.Baro_t(`<span class="bw_muted">창립 기여자가 아닙니다.</span> `) + tool.Baro_button("/baro/founding", tool.Baro_t("창립 시즌 보기"), "")
	if founder != "" {
		founder_html = tool.Baro_badge("창립 기여자 #"+founder, "ok")
	}

	active_html := tool.Baro_badge(tool.Baro_t("휴면"), "muted")
	switch {
	case recent >= 10:
		active_html = tool.Baro_badge(tool.Baro_t("활동 중"), "ok")
	case recent >= 1:
		active_html = tool.Baro_badge(tool.Baro_t("가끔 활동"), "info")
	}
	active_html += tool.Baro_t(` <span class="bw_muted">최근 30일 `) + tool.Baro_comma(recent) + tool.Baro_t(`건</span>`)

	info_html := tool.Baro_dl([][]string{
		{tool.Baro_t("사용자 이름"), tool.HTML_escape(user)},
		{tool.Baro_t("가입일"), func() string {
			if join_date == "" {
				return tool.Baro_t(`<span class="bw_muted">기록이 없습니다.</span>`)
			}

			return tool.HTML_escape(join_date) + ` <span class="bw_muted">(` + tool.HTML_escape(tool.Baro_time_ago(join_date)) + `)</span>`
		}()},
		{tool.Baro_t("이메일 인증"), email_html},
		{tool.Baro_t("창립 번호"), founder_html},
		{tool.Baro_t("활동 상태"), active_html},
		{tool.Baro_t("마지막 활동"), func() string {
			if last_active == "" {
				return tool.Baro_t(`<span class="bw_muted">아직 기여 기록이 없습니다.</span>`)
			}

			return tool.Baro_time_cell(last_active)
		}()},
		{tool.Baro_t("누적 유효 기여"), tool.Baro_comma(total) + tool.Baro_t("건")},
		{tool.Baro_t("소개 문서"), baro_account_user_doc_link(user)},
	})

	body := tool.Baro_hero(tool.Baro_t("내 계정"), user+tool.Baro_t(" 님의 계정 상태와 동의 기록입니다."),
		tool.Baro_button("/baro/profile/"+tool.Url_parser(user), "내 프로필 보기", "primary")+" "+
			tool.Baro_button("/baro/profile/edit", tool.Baro_t("프로필 설정"), "")+" "+
			tool.Baro_button("/baro/onboarding", tool.Baro_t("첫걸음 이어하기"), ""))

	body += baro_account_tabs("/baro/account")
	body += tool.Baro_card(tool.Baro_t("계정 정보"), info_html)

	// --- 제재 상태 ---
	body += tool.Baro_card(tool.Baro_t("제재 상태"), baro_account_sanction_html(db, user))

	// --- 약관 · 라이선스 동의 ---
	body += baro_account_consent_card(db, config, page)

	// --- 편집 자격 방침 ---
	body += tool.Baro_card(tool.Baro_t("편집 자격 방침"),
		tool.Baro_alert("info", tool.Baro_t("바로위키는 초기 단계에서 <b>로그인한 사용자만</b> 문서를 편집할 수 있습니다."))+
			tool.Baro_list([]string{
				tool.Baro_t("기여 이력을 사람 단위로 남겨 감사·복구·이의제기가 가능하도록 하기 위한 조치입니다."),
				tool.Baro_t("익명(IP) 편집은 생태계가 안정된 뒤에 별도 논의를 거쳐 검토합니다. 지금은 열지 않습니다."),
				tool.Baro_t("읽기는 누구나 로그인 없이 할 수 있습니다."),
			})+
			tool.Baro_note(tool.Baro_t("이 방침이 바뀔 때는 정책 판을 새로 올리고 공개 로그에 남깁니다."))+
			tool.Baro_button("/baro/policy", tool.Baro_t("정책 문서 보기"), ""))

	// --- 개인정보 ---
	body += tool.Baro_card(tool.Baro_t("내 정보 관리"),
		tool.Baro_list([]string{
			tool.Baro_t(`<a href="/baro/account/download">내 정보 내려받기 안내</a> — 어떤 정보가 담기는지 먼저 확인하세요.`),
			tool.Baro_t(`<a href="/baro/privacy">개인정보 요청 센터</a> — 열람 · 정정 · 내려받기 · 삭제를 신청합니다.`),
			tool.Baro_t(`<a href="/change">계정 설정(비밀번호 · 이메일 변경)</a> — 위키 본체 설정 화면입니다.`),
			tool.Baro_t(`<a href="/baro/notify/set">알림 설정</a> — 종류마다 즉시 / 일일 요약 / 끄기를 고릅니다.`),
		}))

	return tool.Baro_page(db, config, tool.Baro_t("내 계정"), body, baro_account_menu())
}

// baro_account_consent_card : 약관·라이선스 동의 기록 카드 (2-1)
func baro_account_consent_card(db *sql.DB, config tool.Config, page string) string {
	user := config.IP

	// 현재 시행 중인 정책 판과 내가 동의한 판을 비교한다
	need_html := ""
	state_rows := [][]string{}

	for _, kind_row := range baro_account_policy_kind_list() {
		kind := kind_row[0]
		policy := baro_account_policy(db, kind)
		mine := baro_account_consent_version(db, user, kind)

		state := tool.Baro_badge(tool.Baro_t("최신 판 동의"), "ok")
		if policy[0] == "" {
			state = tool.Baro_badge(tool.Baro_t("정책 없음"), "muted")
		} else if mine == "" {
			state = tool.Baro_badge(tool.Baro_t("동의 기록 없음"), "warn")
		} else if mine != policy[0] {
			state = tool.Baro_badge(tool.Baro_t("재동의 필요"), "danger")
		}

		mine_text := `<span class="bw_muted">-</span>`
		if mine != "" {
			mine_text = tool.HTML_escape(mine) + tool.Baro_t("판")
		}

		now_text := `<span class="bw_muted">-</span>`
		if policy[0] != "" {
			now_text = `<a href="/baro/policy">` + tool.HTML_escape(policy[1]) + `</a> ` + tool.HTML_escape(policy[0]) + tool.Baro_t("판")
		}

		state_rows = append(state_rows, []string{
			tool.HTML_escape(kind_row[1]),
			now_text,
			mine_text,
			state,
		})

		if policy[0] != "" && mine != policy[0] {
			preview := tool.Baro_trunc(policy[2], 600)

			need_html += tool.Baro_card_ex(
				kind_row[1]+" "+policy[0]+tool.Baro_t("판에 동의가 필요합니다"),
				tool.Baro_t("시행 중인 판과 동의한 판이 다릅니다. 내용을 확인하고 동의해 주세요."),
				tool.Baro_code(preview),
				tool.Baro_form("/baro/account",
					tool.Baro_hidden("action", "consent")+
						tool.Baro_hidden("kind", kind)+
						tool.Baro_checkbox("agree", kind_row[1]+" "+policy[0]+tool.Baro_t("판 내용을 읽고 동의합니다."), "on", false, "")+
						tool.Baro_submit(policy[0]+tool.Baro_t("판에 동의"), "primary"),
				)+tool.Baro_t(` <a class="bw_btn" href="/baro/policy">전문 보기</a>`),
			)
		}
	}

	html := tool.Baro_table([]string{"정책", tool.Baro_t("시행 중인 판"), tool.Baro_t("내가 동의한 판"), "상태"}, state_rows, tool.Baro_t("등록된 정책이 없습니다."))
	html += need_html

	if need_html == "" {
		html += tool.Baro_alert("ok", tool.Baro_t("시행 중인 정책 판에 모두 동의한 상태입니다."))
	}

	// --- 내 동의 기록 목록 ---
	page_int := baro_account_page_int(page)
	max_count := tool.Baro_page_size

	rows := tool.Query_DB(
		db,
		"select kind, version, date, ip from baro_consent where user = ? order by id + 0 desc limit ?, ?",
		user, (page_int-1)*max_count, max_count,
	)
	defer rows.Close()

	list_rows := [][]string{}
	for rows.Next() {
		kind, version, date, ip := "", "", "", ""
		if err := rows.Scan(&kind, &version, &date, &ip); err != nil {
			continue
		}

		who := tool.Baro_t(`<span class="bw_muted">기록 없음</span>`)
		switch {
		case strings.TrimSpace(ip) == "":
		case ip == user:
			who = tool.Baro_t(`<span class="bw_muted">로그인 계정으로 기록</span>`)
		default:
			who = tool.HTML_escape(ip) + tool.Baro_t(` <span class="bw_muted">(일부 가림)</span>`)
		}

		list_rows = append(list_rows, []string{
			tool.HTML_escape(baro_account_policy_name(kind)),
			tool.HTML_escape(version) + tool.Baro_t("판"),
			tool.HTML_escape(date),
			who,
		})
	}

	list_html := ""
	if len(list_rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("아직 동의 기록이 없습니다. 문서를 처음 게시할 때 라이선스 동의를 받습니다."))
	} else {
		list_html = tool.Baro_table([]string{"정책", "판", tool.Baro_t("동의한 때"), tool.Baro_t("남긴 기록(가림)")}, list_rows, tool.Baro_t("아직 동의 기록이 없습니다."))
	}

	list_html += tool.Baro_paging(page_int, len(list_rows), max_count, "/baro/account?page={}")
	list_html += tool.Baro_note(tool.Baro_t("접속 기록은 부분만 저장합니다. 전체 IP 는 남기지 않습니다. 로그인 상태에서 동의하면 계정 이름만 남습니다."))

	return tool.Baro_card(tool.Baro_t("약관 · 라이선스 동의"), html) +
		tool.Baro_card(tool.Baro_t("내 동의 기록"), list_html)
}

// View_baro_account_download : 개인정보 내려받기 안내 (/baro/account/download)
func View_baro_account_download(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/account/download"); need {
		return wall
	}

	user := config.IP
	_ = page
	_ = values

	draft_count := 0
	tool.QueryRow_DB(db, "select count(*) from baro_workspace where user = ?", []any{&draft_count}, user)

	notify_count := 0
	tool.QueryRow_DB(db, "select count(*) from baro_notify where user = ?", []any{&notify_count}, user)

	consent_count := 0
	tool.QueryRow_DB(db, "select count(*) from baro_consent where user = ?", []any{&consent_count}, user)

	body := tool.Baro_hero(tool.Baro_t("내 정보 내려받기"), tool.Baro_t("어떤 정보가 담기는지 먼저 확인하세요."), "")
	body += baro_account_tabs("/baro/account")

	body += tool.Baro_card(tool.Baro_t("담기는 정보"),
		tool.Baro_table([]string{tool.Baro_t("항목"), tool.Baro_t("내용"), tool.Baro_t("현재 보유")}, [][]string{
			{"계정", tool.Baro_t("사용자 이름, 가입일, 이메일(있는 경우)"), tool.Baro_t("1건")},
			{tool.Baro_t("동의 기록"), tool.Baro_t("약관·라이선스 동의 종류와 판 번호"), tool.Baro_comma(consent_count) + tool.Baro_t("건")},
			{"기여 기록", tool.Baro_t("기여 이벤트 종류·문서·시각"), tool.Baro_comma(baro_account_contrib_count(db, user)) + tool.Baro_t("건")},
			{"초안", tool.Baro_t("연습장을 포함한 작업 중인 초안 본문"), tool.Baro_comma(draft_count) + tool.Baro_t("건")},
			{tool.Baro_t("알림"), tool.Baro_t("받은 알림 제목과 시각"), tool.Baro_comma(notify_count) + tool.Baro_t("건")},
			{tool.Baro_t("프로필 설정"), tool.Baro_t("한 줄 소개, 대표 문서, 공개 항목 설정"), tool.Baro_t("1건")},
		}, "")+
			tool.Baro_note(tool.Baro_t("공개된 문서 편집 역사는 위키의 공개 기록이라 내려받기 결과와 별개로 유지됩니다.")))

	body += tool.Baro_card(tool.Baro_t("신청 방법"),
		tool.Baro_alert("info", tool.Baro_t("실제 내려받기 처리는 개인정보 요청 센터에서 신청합니다. 본인 확인 뒤 파일이 만들어집니다."))+
			tool.Baro_list([]string{
				tool.Baro_t("요청 센터에서 <b>내려받기</b> 종류로 신청합니다."),
				tool.Baro_t("담당자가 본인 여부를 확인합니다."),
				tool.Baro_t("준비되면 알림으로 알려드리고, 받은 링크는 일정 기간 뒤 만료됩니다."),
			})+
			tool.Baro_button("/baro/privacy", tool.Baro_t("개인정보 요청 센터로 가기"), "primary")+" "+
			tool.Baro_button("/baro/export", tool.Baro_t("공개 데이터 내보내기"), "")+" "+
			tool.Baro_button("/baro/account", tool.Baro_t("내 계정으로 돌아가기"), ""))

	body += tool.Baro_card(tool.Baro_t("삭제를 원하신다면"),
		tool.Baro_note(tool.Baro_t("계정 삭제를 신청해도 이미 공개된 정상 기여는 라이선스에 따라 자동으로 지워지지 않습니다. 자세한 내용은 요청 센터의 안내를 확인하세요."))+
			tool.Baro_button("/baro/privacy", tool.Baro_t("삭제 신청 안내"), ""))

	return tool.Baro_page(db, config, tool.Baro_t("내 정보 내려받기"), body, baro_account_menu())
}
