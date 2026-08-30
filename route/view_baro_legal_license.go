package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  15-4 기여 라이선스 동의
//
//  URL : /baro/legal/license , /baro/legal/consent
//
//  게시 버튼을 누르기 전에 반드시 알려야 하는 네 가지를 화면으로 만든 것이다.
// ============================================================

// baro_legal_license_points : 게시 전에 반드시 알려야 하는 내용
//
//	{제목, 설명}
func baro_legal_license_points(license_name string) [][]string {
	return [][]string{
		{
			tool.Baro_t("어떤 라이선스로 공개되는가"),
			tool.Baro_t("여기에 쓴 글은 ") + license_name + tool.Baro_t(" 라이선스로 공개됩니다. 위키 전체가 같은 라이선스를 씁니다."),
		},
		{
			tool.Baro_t("다른 사람이 고치고 다시 쓸 수 있다"),
			tool.Baro_t("내가 쓴 문장을 다른 사람이 고치고, 잘라 내고, 다른 문서에 옮겨 쓸 수 있습니다. 위키 밖에서도 같은 조건으로 다시 쓸 수 있습니다."),
		},
		{
			tool.Baro_t("탈퇴해도 정상 기여는 자동으로 지워지지 않는다"),
			tool.Baro_t("계정을 닫아도 이미 공개된 정상 기여와 편집 이력은 그대로 남습니다. 다른 사람이 그 위에 이어서 편집했을 수 있고, 출처 표기 의무가 있기 때문입니다."),
		},
		{
			tool.Baro_t("출처를 표기해야 한다"),
			tool.Baro_t("다른 곳의 글을 옮겨 올 때는 반드시 출처를 밝혀야 합니다. 출처를 밝히더라도 저작권이 있는 글을 그대로 옮기는 것은 안 됩니다. 직접 쓴 문장으로 정리해 주세요."),
		},
	}
}

// baro_legal_license_consent_row : 동의 기록 한 건
type baro_legal_license_consent_row struct {
	ID      string
	Kind    string
	Version string
	Date    string
	IP      string
}

// baro_legal_license_kind_name : 동의 종류 이름
func baro_legal_license_kind_name(kind string) string {
	switch kind {
	case "license":
		return tool.Baro_t("기여 라이선스")
	case "privacy":
		return tool.Baro_t("개인정보 처리 방침")
	case "deletion_notice":
		return tool.Baro_t("계정 탈퇴 안내")
	case "rule":
		return tool.Baro_t("운영 규칙")
	}

	return kind
}

// baro_legal_license_consented : 현재 판에 동의했는지
func baro_legal_license_consented(db *sql.DB, user string, version string) bool {
	if !tool.Baro_is_user(user) {
		return false
	}

	found := ""
	tool.QueryRow_DB(
		db,
		"select id from baro_consent where user = ? and kind = 'license' and version = ? limit 1",
		[]any{&found}, user, version,
	)

	return found != ""
}

// ------------------------------------------------------------
//  15-4 라이선스 안내와 동의 : /baro/legal/license
// ------------------------------------------------------------

// View_baro_legal_license : 기여 라이선스 안내와 동의
func View_baro_legal_license(config tool.Config, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	policy := baro_legal_policy_get(db, "license")
	license_name := tool.Baro_get_def(db, "license", "CC BY-SA 4.0")

	version := policy.Version
	if version == "" {
		version = "0"
	}

	message := ""

	if values != nil {
		switch {
		case !tool.Baro_is_user(config.IP):
			message = tool.Baro_alert("danger", tool.Baro_t("동의 기록은 로그인한 계정에만 남길 수 있습니다."))
		case tool.Baro_read_only(db):
			message = tool.Baro_alert("danger", tool.Baro_t("지금은 읽기 전용 상태라 동의를 기록할 수 없습니다."))
		case !baro_legal_form_on(values, "agree"):
			message = tool.Baro_alert("danger", tool.Baro_t("안내를 읽고 동의에 표시해 주세요."))
		default:
			already := ""
			tool.QueryRow_DB(
				db,
				"select id from baro_consent where user = ? and kind = 'license' and version = ? limit 1",
				[]any{&already}, config.IP, version,
			)

			if already == "" {
				tool.Exec_DB(
					db,
					"insert into baro_consent (id, user, kind, version, date, ip) values (?, ?, 'license', ?, ?, ?)",
					tool.Baro_next_id(db, "baro_consent"), config.IP, version, tool.Baro_now(), tool.Baro_mask_ip(config.IP),
				)

				tool.Baro_public_log(db, "license_consent", "system", "", "기여 라이선스 동의", version+"판")
				tool.Baro_audit(db, config, "license_consent", "baro_consent", "", "license "+version+"판", "기여 라이선스 동의")
				tool.Baro_metric_bump(db, "license_consent", tool.Baro_today(), 1)

				tool.Baro_notify(
					db, config.IP, "system",
					tool.Baro_t("기여 라이선스 ")+version+tool.Baro_t("판에 동의했습니다"),
					"/baro/legal/consent", "",
				)
			}

			return tool.Get_redirect("/baro/legal/consent")
		}
	}

	consented := baro_legal_license_consented(db, config.IP, version)

	body := tool.Baro_crumb([][]string{
		{"/baro/legal", tool.Baro_t("법적 요청")},
		{"", tool.Baro_t("기여 라이선스")},
	})

	body += message

	head := tool.Baro_stat_row([][]string{
		{"라이선스", license_name, tool.Baro_t("위키 전체 공통")},
		{tool.Baro_t("정책 판"), version + tool.Baro_t("판"), tool.Baro_t("현재 시행 중")},
		{tool.Baro_t("내 동의"), func() string {
			if consented {
				return tool.Baro_t("완료")
			}

			return tool.Baro_t("아직")
		}(), tool.Baro_t("현재 판 기준")},
	})

	body += tool.Baro_hero(
		tool.Baro_t("기여 라이선스 안내"),
		tool.Baro_t("게시 버튼을 누르기 전에 이 네 가지를 꼭 알아 두세요."),
		head,
	)

	// 네 가지 필수 안내
	cards := []string{}
	for index, point := range baro_legal_license_points(license_name) {
		cards = append(cards, tool.Baro_card_ex(
			tool.Baro_itoa(index+1)+". "+point[0],
			"",
			`<p>`+tool.HTML_escape(point[1])+`</p>`,
			"",
		))
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("게시 전에 알아야 할 것"),
		tool.Baro_t("이 안내는 편집 화면의 게시 버튼 옆에서도 같은 내용으로 보여야 합니다."),
		tool.Baro_grid(cards, 2),
		"",
	)

	body += tool.Baro_card_ex(
		tool.Baro_t("게시 버튼 옆에 붙는 짧은 안내"),
		tool.Baro_t("편집 화면에서 게시 버튼을 누르기 직전에 보이는 안내입니다."),
		baro_legal_license_short_notice(license_name),
		"",
	)

	body += tool.Baro_alert(
		"danger",
		tool.Baro_t(`<strong>다른 곳의 글을 그대로 옮기지 마세요.</strong> 출처를 밝혀도 저작권이 있는 문장을 그대로 옮기면 저작권 침해입니다. `)+
			tool.Baro_t(`침해가 신고되면 해당 문서는 임시로 격리되고, 원 작성자는 `)+
			tool.Baro_t(`<a href="/baro/copyright">저작권 요청 센터</a>에서 소명할 수 있습니다.`),
	)

	// 정책 본문
	body += tool.Baro_card_ex(
		tool.Baro_t("현재 시행 중인 라이선스 정책 본문"),
		tool.Baro_t("baro_policy 의 license 최신 판을 그대로 보여 줍니다."),
		baro_legal_policy_head(policy)+baro_legal_text_html(policy.Data),
		tool.Baro_button("/baro/policy", tool.Baro_t("정책 판 이력 보기"), ""),
	)

	// 동의
	if !tool.Baro_is_user(config.IP) {
		body += tool.Baro_alert("info", tool.Baro_t(`동의 기록은 로그인한 계정에만 남습니다. `)+tool.Baro_button("/login", tool.Baro_t("로그인"), "primary"))

		return baro_legal_page(db, config, tool.Baro_t("기여 라이선스"), "/baro/legal/license", body)
	}

	if consented {
		body += tool.Baro_card_ex(
			tool.Baro_t("동의 완료"),
			tool.Baro_t("현재 판(")+version+tool.Baro_t("판)에 이미 동의했습니다."),
			tool.Baro_alert("ok", tool.Baro_t("기여 라이선스 ")+tool.HTML_escape(version)+tool.Baro_t("판에 동의한 기록이 남아 있습니다. 정책이 새 판으로 바뀌면 다시 동의를 받습니다.")),
			tool.Baro_button("/baro/legal/consent", tool.Baro_t("내 동의 이력 보기"), "primary"),
		)

		return baro_legal_page(db, config, tool.Baro_t("기여 라이선스"), "/baro/legal/license", body)
	}

	form := tool.Baro_checkbox(
		"agree",
		tool.Baro_t("위 네 가지 안내를 읽었고, 내 기여가 ")+license_name+tool.Baro_t(" 로 공개되는 데 동의합니다."),
		"on", false,
		tool.Baro_t("동의 기록에는 계정 이름 · 정책 판 · 시각 · 가려진 IP 만 남습니다."),
	)
	form += tool.Baro_submit(tool.Baro_t("기여 라이선스 ")+version+tool.Baro_t("판에 동의"), "primary")

	body += tool.Baro_card_ex(
		tool.Baro_t("동의하기"),
		tool.Baro_t("동의는 baro_consent 에 kind=license, version=")+version+tool.Baro_t(" 으로 기록됩니다."),
		tool.Baro_form("/baro/legal/license", form),
		"",
	)

	return baro_legal_page(db, config, tool.Baro_t("기여 라이선스"), "/baro/legal/license", body)
}

// ------------------------------------------------------------
//  15-4 동의 이력 : /baro/legal/consent
// ------------------------------------------------------------

// View_baro_legal_consent : 내 동의 이력
func View_baro_legal_consent(config tool.Config, page string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/legal/consent"); need {
		return wall
	}

	page_num := baro_legal_page_num(page)
	size := tool.Baro_page_size

	list := []baro_legal_license_consent_row{}

	rows := tool.Query_DB(
		db,
		"select id, kind, version, date, ip from baro_consent where user = ? order by id + 0 desc limit "+tool.Baro_itoa(size)+" offset "+tool.Baro_itoa((page_num-1)*size),
		config.IP,
	)
	defer rows.Close()

	for rows.Next() {
		row := baro_legal_license_consent_row{}
		if err := rows.Scan(&row.ID, &row.Kind, &row.Version, &row.Date, &row.IP); err != nil {
			continue
		}

		list = append(list, row)
	}

	policy := baro_legal_policy_get(db, "license")
	version := policy.Version
	if version == "" {
		version = "0"
	}

	table_rows := [][]string{}
	for _, row := range list {
		current := ""
		if row.Kind == "license" && row.Version == version {
			current = " " + tool.Baro_badge(tool.Baro_t("현재 판"), "ok")
		}

		table_rows = append(table_rows, []string{
			tool.HTML_escape(row.ID),
			tool.HTML_escape(baro_legal_license_kind_name(row.Kind)) + current,
			tool.HTML_escape(row.Version) + tool.Baro_t("판"),
			tool.Baro_time_cell(row.Date),
			tool.HTML_escape(row.IP),
		})
	}

	list_html := tool.Baro_table(
		[]string{"번호", tool.Baro_t("동의 종류"), tool.Baro_t("정책 판"), tool.Baro_t("동의 시각"), tool.Baro_t("접속 위치(가림)")},
		table_rows,
		tool.Baro_t("동의 기록이 없습니다."),
	)

	if len(table_rows) == 0 {
		list_html = tool.Baro_empty(tool.Baro_t("아직 남은 동의 기록이 없습니다. 기여 라이선스 안내를 읽고 동의하면 여기에 남습니다."))
	}

	list_html += tool.Baro_paging(page_num, len(list), size, "/baro/legal/consent/{}")

	consented := baro_legal_license_consented(db, config.IP, version)

	body := tool.Baro_crumb([][]string{
		{"/baro/legal", tool.Baro_t("법적 요청")},
		{"/baro/legal/license", tool.Baro_t("기여 라이선스")},
		{"", tool.Baro_t("동의 이력")},
	})

	total := baro_legal_count(db, "select count(*) from baro_consent where user = ?", config.IP)
	license_total := baro_legal_count(db, "select count(*) from baro_consent where user = ? and kind = 'license'", config.IP)

	body += tool.Baro_hero(
		tool.Baro_t("내 동의 이력"),
		tool.Baro_t("어떤 정책의 몇 판에 언제 동의했는지 남습니다."),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("전체 동의"), tool.Baro_comma(total), ""},
			{tool.Baro_t("라이선스 동의"), tool.Baro_comma(license_total), ""},
			{tool.Baro_t("현재 판 동의"), func() string {
				if consented {
					return tool.Baro_t("완료")
				}

				return tool.Baro_t("아직")
			}(), version + tool.Baro_t("판 기준")},
		}),
	)

	if !consented {
		body += tool.Baro_alert("warn", tool.Baro_t(`현재 시행 중인 기여 라이선스 `)+tool.HTML_escape(version)+tool.Baro_t(`판에 아직 동의하지 않았습니다. `)+tool.Baro_button("/baro/legal/license", tool.Baro_t("안내 읽고 동의하기"), "primary"))
	}

	body += tool.Baro_card_ex(
		tool.Baro_t("동의 기록"),
		tool.Baro_t("기록에는 접속 위치를 가려서 남깁니다. (원칙 5)"),
		list_html,
		tool.Baro_note(tool.Baro_t("정책이 새 판으로 바뀌면 예전 판의 동의 기록은 그대로 두고 새 동의를 다시 받습니다.")),
	)

	return baro_legal_page(db, config, tool.Baro_t("내 동의 이력"), "/baro/legal/license", body)
}

// baro_legal_license_short_notice : 다른 화면에서도 쓸 수 있는 짧은 게시 전 안내
func baro_legal_license_short_notice(license_name string) string {
	points := []string{}
	for _, point := range baro_legal_license_points(license_name) {
		points = append(points, `<strong>`+tool.HTML_escape(point[0])+`</strong> — `+tool.HTML_escape(point[1]))
	}

	return tool.Baro_alert("info", strings.Join([]string{
		tool.Baro_t(`게시하면 이 내용은 `) + tool.HTML_escape(license_name) + ` 로 공개됩니다.`,
		tool.Baro_list(points),
		tool.Baro_button("/baro/legal/license", tool.Baro_t("자세한 안내와 동의"), ""),
	}, ""))
}
