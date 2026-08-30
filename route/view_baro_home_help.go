package route

import (
	"database/sql"
	"net/url"

	"opennamu/route/tool"
)

// ============================================================
//  도움말 (/baro/help)
//
//  30초 편집 안내 · 문법 요약표 · 출처 다는 법 · 자주 묻는 질문.
//  도움말 문서는 "바로위키" 이름공간(baro:)에 모은다.
// ============================================================

// View_baro_home_help : 도움말
func View_baro_home_help(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = values

	body := baro_home_notice(query)
	body += baro_home_help_hero(db, config)
	body += baro_home_help_quick(db, config)
	body += baro_home_help_syntax()
	body += baro_home_help_source(db)
	body += baro_home_help_namespace(db, query)
	body += baro_home_help_faq(db)

	return tool.Baro_page(db, config, tool.Baro_t("도움말"), body, baro_home_menu())
}

// baro_home_help_hero : 도움말 머리말
func baro_home_help_hero(db *sql.DB, config tool.Config) string {
	buttons := ""
	if tool.Baro_is_user(config.IP) {
		buttons = tool.Baro_button("/baro/draft", tool.Baro_t("연습장에서 해보기"), "primary") + " " +
			tool.Baro_button("/baro/task", tool.Baro_t("쉬운 할 일 찾기"), "")
	} else {
		buttons = tool.Baro_button("/register", tool.Baro_t("가입하고 시작하기"), "primary") + " " +
			tool.Baro_button("/login", tool.Baro_t("로그인"), "")
	}

	buttons += " " + tool.Baro_button("/baro/onboarding", tool.Baro_t("첫 기여 도우미"), "")

	return tool.Baro_hero(
		tool.Baro_t("도움말"),
		tool.Baro_t("처음이어도 괜찮습니다. 30초면 첫 편집을 끝낼 수 있습니다."),
		`<p>`+tool.HTML_escape(tool.Baro_t("고칠 곳을 찾고, 고치고, 어디서 봤는지 적으면 끝입니다."))+`</p><div>`+buttons+`</div>`,
	)
}

// baro_home_help_quick : 30초 편집 안내
func baro_home_help_quick(db *sql.DB, config tool.Config) string {
	steps := []string{
		tool.Baro_card(tool.Baro_t("1. 고칠 곳을 찾습니다"),
			tool.Baro_t(`<p>문서를 읽다가 틀린 내용, 빠진 내용, 출처 없는 문장을 발견하면 그곳이 시작점입니다.</p>`)+
				tool.Baro_note(tool.Baro_t("무엇을 고칠지 모르겠다면 할 일 목록에서 쉬운 과제를 고르세요."))+
				tool.Baro_button("/baro/task", tool.Baro_t("할 일 목록"), "")),
		tool.Baro_card(tool.Baro_t("2. 편집을 누릅니다"),
			tool.Baro_t(`<p>문서 위쪽의 <strong>편집</strong> 을 누르면 본문이 그대로 보입니다. `)+
				tool.Baro_t(`전체를 다시 쓸 필요 없이 그 한 줄만 고치면 됩니다.</p>`)+
				tool.Baro_note(tool.Baro_t("한 번에 한 가지만 고치는 편집이 검토도 빠르고 되돌려질 위험도 적습니다."))),
		tool.Baro_card(tool.Baro_t("3. 어디서 봤는지 적습니다"),
			tool.Baro_t(`<p>고친 내용의 근거가 되는 책·기사·공공기관 자료 주소를 함께 적습니다. `)+
				tool.Baro_t(`출처가 붙은 문장은 웬만해선 지워지지 않습니다.</p>`)+
				tool.Baro_note(tool.Baro_t("출처를 다는 일 자체가 기여로 기록됩니다."))),
		tool.Baro_card(tool.Baro_t("4. 요약을 쓰고 저장합니다"),
			tool.Baro_t(`<p>"연도 오류 수정", "출처 추가" 처럼 한 줄만 남기면 충분합니다. `)+
				tool.Baro_t(`저장하면 기여 기록에 남고, 며칠 뒤 살아남은 기여로 확정됩니다.</p>`)+
				tool.Baro_note(tool.Baro_t("저장한 편집은 언제든 되돌릴 수 있으니 겁내지 않아도 됩니다."))),
	}

	body := tool.Baro_grid(steps, 2)

	if tool.Baro_is_user(config.IP) {
		body += tool.Baro_alert("ok", tool.Baro_t("로그인 상태입니다. 지금 바로 편집할 수 있습니다. ")+
			tool.Baro_button("/baro/draft", tool.Baro_t("연습장 열기"), ""))
	} else {
		body += tool.Baro_alert("info", tool.Baro_t("바로위키는 로그인한 사용자만 편집할 수 있습니다. 기여를 사람 단위로 남기기 위해서입니다. ")+
			tool.Baro_button("/register", tool.Baro_t("가입하기"), "primary"))
	}

	_ = db

	return tool.Baro_card(tool.Baro_t("30초 편집 안내"), body)
}

// baro_home_help_syntax : 문법 요약표
func baro_home_help_syntax() string {
	rows := [][]string{}

	for _, row := range [][]string{
		{tool.Baro_t("문단 제목"), tool.Baro_t("= 제목 ="), tool.Baro_t("= 큰 제목 = 부터 ====== 작은 제목 ====== 까지 여섯 단계")},
		{tool.Baro_t("굵게"), tool.Baro_t("'''굵은 글씨'''"), tool.Baro_t("강조할 때만 씁니다")},
		{tool.Baro_t("기울임"), tool.Baro_t("''기울인 글씨''"), tool.Baro_t("책 제목 등")},
		{tool.Baro_t("취소선"), tool.Baro_t("~~지운 글씨~~"), tool.Baro_t("옛 정보를 남길 때")},
		{tool.Baro_t("밑줄"), tool.Baro_t("__밑줄__"), tool.Baro_t("되도록 적게 씁니다")},
		{tool.Baro_t("문서 링크"), tool.Baro_t("[[문서 이름]]"), tool.Baro_t("없는 문서는 붉게 표시됩니다")},
		{tool.Baro_t("이름 바꾼 링크"), tool.Baro_t("[[문서 이름|보일 글자]]"), tool.Baro_t("문장에 자연스럽게 넣을 때")},
		{tool.Baro_t("바깥 링크"), tool.Baro_t("[[https://example.com|설명]]"), tool.Baro_t("출처 주소를 적을 때")},
		{tool.Baro_t("목록"), tool.Baro_t("* 항목"), tool.Baro_t("* 를 두 칸 들여쓰면 하위 항목")},
		{tool.Baro_t("번호 목록"), tool.Baro_t("1. 항목"), tool.Baro_t("순서가 있는 내용")},
		{"각주", tool.Baro_t("[* 각주 내용]"), tool.Baro_t("출처를 문장 끝에 다는 기본 방법")},
		{tool.Baro_t("표"), tool.Baro_t("|| 칸 || 칸 ||"), tool.Baro_t("줄마다 || 로 칸을 나눕니다")},
		{"분류", tool.Baro_t("[[분류:한국사]]"), tool.Baro_t("문서 맨 아래에 답니다")},
		{tool.Baro_t("파일"), tool.Baro_t("[[파일:이름.jpg]]"), tool.Baro_t("저작권이 확인된 파일만")},
		{tool.Baro_t("줄바꿈"), "[br]", tool.Baro_t("문단 안에서 줄을 바꿀 때")},
		{tool.Baro_t("그대로 보이기"), tool.Baro_t("{{{문법을 적용하지 않음}}}"), tool.Baro_t("코드나 기호를 그대로 보일 때")},
		{tool.Baro_t("주석"), tool.Baro_t("##설명"), tool.Baro_t("본문에는 보이지 않습니다")},
		{tool.Baro_t("넘겨주기"), tool.Baro_t("#redirect 대상 문서"), tool.Baro_t("문서 첫 줄에만 씁니다")},
	} {
		rows = append(rows, []string{
			tool.HTML_escape(row[0]),
			tool.Baro_code(row[1]),
			tool.HTML_escape(row[2]),
		})
	}

	body := tool.Baro_table([]string{tool.Baro_t("쓰임"), tool.Baro_t("문법"), tool.Baro_t("설명")}, rows, "")
	body += tool.Baro_note(tool.Baro_t("문법이 헷갈리면 그냥 줄글로 써도 됩니다. 모양은 나중에 다른 기여자가 다듬어 줍니다."))

	return tool.Baro_card(tool.Baro_t("문법 요약표"), body)
}

// baro_home_help_source : 출처 다는 법
func baro_home_help_source(db *sql.DB) string {
	body := tool.Baro_t(`<p>바로위키에서 가장 중요한 습관은 <strong>어디서 봤는지 적는 것</strong>입니다. `) +
		tool.Baro_t(`출처가 있는 문장은 논쟁이 생겨도 지워지지 않습니다.</p>`)

	body += tool.Baro_section(tool.Baro_t("기본 모양"), tool.Baro_code(
		tool.Baro_t("1919년 3월 1일 만세운동이 일어났다.[* 국사편찬위원회, 《한국사》 제47권, 2001, 120쪽.]\n")+
			tool.Baro_t("2023년 이용자 수는 12만 명이다.[* [[https://kostat.go.kr/example|통계청 보도자료]], 2024-01-10 확인.]")))

	body += tool.Baro_section(tool.Baro_t("출처에 꼭 넣을 것"), tool.Baro_table(
		[]string{tool.Baro_t("항목"), tool.Baro_t("예"), tool.Baro_t("왜 필요한가")},
		[][]string{
			{tool.HTML_escape(tool.Baro_t("글쓴이·기관")), tool.HTML_escape(tool.Baro_t("국사편찬위원회")), tool.HTML_escape(tool.Baro_t("누가 한 말인지 알 수 있어야 합니다"))},
			{tool.HTML_escape(tool.Baro_t("제목")), tool.HTML_escape(tool.Baro_t("《한국사》 제47권")), tool.HTML_escape(tool.Baro_t("같은 기관의 다른 자료와 구분됩니다"))},
			{tool.HTML_escape(tool.Baro_t("펴낸 곳·매체")), tool.HTML_escape(tool.Baro_t("국사편찬위원회 / 한겨레")), tool.HTML_escape(tool.Baro_t("자료의 성격을 판단할 수 있습니다"))},
			{tool.HTML_escape(tool.Baro_t("날짜")), tool.HTML_escape("2001 / 2024-01-10"), tool.HTML_escape(tool.Baro_t("언제 기준의 사실인지 알 수 있습니다"))},
			{tool.HTML_escape(tool.Baro_t("쪽·주소")), tool.HTML_escape(tool.Baro_t("120쪽 / https://…")), tool.HTML_escape(tool.Baro_t("다른 사람이 직접 확인할 수 있습니다"))},
			{tool.HTML_escape(tool.Baro_t("확인한 날")), tool.HTML_escape(tool.Baro_t("2024-01-10 확인")), tool.HTML_escape(tool.Baro_t("웹 문서가 사라져도 시점을 알 수 있습니다"))},
		},
		"",
	))

	body += tool.Baro_section(tool.Baro_t("좋은 출처와 약한 출처"), tool.Baro_grid([]string{
		tool.Baro_card(tool.Baro_t("좋은 출처"), tool.Baro_list([]string{
			tool.Baro_t("공공기관 · 학술기관이 낸 자료"),
			tool.Baro_t("단행본 · 학술 논문 (쪽수까지)"),
			tool.Baro_t("기자 이름이 있는 언론 기사"),
			tool.Baro_t("당사자 · 기관의 공식 발표문"),
			tool.Baro_t("사라질 수 있는 웹 문서는 보존 주소를 함께"),
		})),
		tool.Baro_card(tool.Baro_t("약한 출처"), tool.Baro_list([]string{
			tool.Baro_t("출처 없이 옮겨 적은 다른 위키"),
			tool.Baro_t("익명 커뮤니티 게시글 · 댓글"),
			tool.Baro_t("광고성 보도자료를 그대로 옮긴 기사"),
			tool.Baro_t("\"알려져 있다\" 같은 주어 없는 문장"),
			tool.Baro_t("개인 기억 · 직접 관찰 (검증할 수 없음)"),
		})),
	}, 2))

	total := baro_home_count(db, "select count(*) from baro_citation")
	week := baro_home_count(db, "select count(*) from baro_citation where date >= ?", baro_home_week_ago())

	body += tool.Baro_stat_row([][]string{
		{tool.Baro_t("등록된 출처"), tool.Baro_comma(total) + tool.Baro_t("개"), tool.Baro_t("누적")},
		{tool.Baro_t("이번 주 추가"), tool.Baro_comma(week) + tool.Baro_t("개"), tool.Baro_t("최근 7일")},
	})

	body += tool.Baro_button("/baro/source", tool.Baro_t("출처 품질 화면"), "") + " " +
		tool.Baro_button("/baro/quality", tool.Baro_t("문서 품질 현황"), "")

	return tool.Baro_card(tool.Baro_t("출처 다는 법"), body)
}

// baro_home_help_ns_note : 이름공간마다 무엇을 담고 누가 편집하는지
//
//	이름공간 목록 자체는 기반 계층(tool.Namespace_list)에서 읽는다.
func baro_home_help_ns_note(prefix string) []string {
	switch prefix {
	case "":
		return []string{tool.Baro_t("일반 문서. 접두사 없이 그냥 씁니다"), tool.Baro_t("로그인 사용자")}
	case "help:":
		return []string{tool.Baro_t("편집 방법 · 문법 · 출처 안내 문서"), tool.Baro_t("로그인 사용자")}
	case "baro:":
		return []string{tool.Baro_t("바로위키 운영 안내 · 정책 해설 · 창립 시즌 기록"), tool.Baro_t("로그인 사용자")}
	case "project:":
		return []string{tool.Baro_t("함께 채우는 공동 작업 문서"), tool.Baro_t("로그인 사용자")}
	case "category:":
		return []string{tool.Baro_t("문서를 묶는 분류"), tool.Baro_t("로그인 사용자")}
	case "template:":
		return []string{tool.Baro_t("문서에 끼워 쓰는 틀"), tool.Baro_t("로그인 사용자")}
	case "file:":
		return []string{tool.Baro_t("올린 파일과 저작권 정보"), tool.Baro_t("로그인 사용자")}
	case "discussion:":
		return []string{tool.Baro_t("문서 바깥에서 이어 가는 논의"), tool.Baro_t("로그인 사용자")}
	case "user:":
		return []string{tool.Baro_t("기여자 개인 문서"), tool.Baro_t("본인 위주")}
	case "ipuser:":
		return []string{tool.Baro_t("비로그인 기여자 기록"), tool.Baro_t("관리자")}
	case "manage:":
		return []string{tool.Baro_t("운영 공지"), tool.Baro_t("관리자")}
	case "trash:":
		return []string{tool.Baro_t("삭제된 문서 보관"), tool.Baro_t("관리자")}
	case "filetrash:":
		return []string{tool.Baro_t("삭제된 파일 보관"), tool.Baro_t("관리자")}
	}

	return []string{tool.Baro_t("부속 문서"), tool.Baro_t("로그인 사용자")}
}

// baro_home_help_ns_docs : 도움말 계열 이름공간(도움말: · 바로위키:)에 있는 문서
func baro_home_help_ns_docs(db *sql.DB, offset int, limit int) [][]string {
	out := [][]string{}

	rows := tool.Query_DB(
		db,
		"select title from data where title like ? or title like ? order by title limit ?, ?",
		"help:%", "baro:%", offset, limit,
	)
	defer rows.Close()

	titles := []string{}
	for rows.Next() {
		title := ""
		if err := rows.Scan(&title); err != nil {
			continue
		}

		titles = append(titles, title)
	}

	for _, title := range titles {
		out = append(out, []string{title, baro_home_doc_excerpt(db, title, 70)})
	}

	return out
}

// baro_home_help_namespace : 도움말 이름공간 안내
func baro_home_help_namespace(db *sql.DB, query url.Values) string {
	page := baro_home_page_num(query, "help_page")
	size := 15

	docs := baro_home_help_ns_docs(db, (page-1)*size, size+1)

	more := len(docs) > size
	if more {
		docs = docs[:size]
	}

	rows := [][]string{}
	for _, doc := range docs {
		excerpt := doc[1]
		if excerpt == "" {
			excerpt = "-"
		}

		rows = append(rows, []string{
			tool.Baro_doc_link(doc[0]),
			tool.HTML_escape(excerpt),
		})
	}

	body := tool.Baro_t(`<p>편집 방법과 문법을 설명하는 글은 <strong>도움말</strong> 이름공간에, `) +
		tool.Baro_t(`운영 안내와 정책 해설은 <strong>바로위키</strong> 이름공간에 모읍니다. `) +
		tool.Baro_t(`문서 이름 앞에 <code>도움말:</code> 또는 <code>바로위키:</code> 를 붙이면 그 이름공간에 들어갑니다.</p>`)

	ns_rows := [][]string{}
	for _, namespace := range tool.Namespace_list() {
		note := baro_home_help_ns_note(namespace.Prefix)

		name := tool.Namespace_name_now(namespace) + ":"
		if namespace.Prefix == "" {
			name = tool.Baro_t("문서:")
		}

		ns_rows = append(ns_rows, []string{
			tool.Baro_code(name),
			tool.Baro_code(namespace.Prefix),
			tool.HTML_escape(note[0]),
			tool.HTML_escape(note[1]),
		})
	}

	body += tool.Baro_table([]string{tool.Baro_t("이름공간"), tool.Baro_t("내부 접두사"), tool.Baro_t("쓰임"), tool.Baro_t("누가 편집하나")}, ns_rows, "")
	body += tool.Baro_note(tool.Baro_t("표시 이름과 내부 접두사는 어느 쪽으로 써도 같은 이름공간으로 들어갑니다. (예: 도움말:문법 = help:문법)"))

	body += tool.Baro_section(tool.Baro_t("도움말 · 바로위키 이름공간 문서"), tool.Baro_table(
		[]string{"문서", tool.Baro_t("앞부분")},
		rows,
		tool.Baro_t("아직 등록된 도움말 문서가 없습니다. 도움말:문법 처럼 만들면 이곳에 모입니다."),
	))

	count := 0
	if more {
		count = size
	}

	body += tool.Baro_paging(page, count, size, "/baro/help?help_page={}")

	return tool.Baro_card(tool.Baro_t("도움말 이름공간 안내"), body)
}

// baro_home_help_faq : 자주 묻는 질문
func baro_home_help_faq(db *sql.DB) string {
	license := tool.HTML_escape(tool.Baro_get_def(db, "license", "CC BY-SA 4.0"))
	appeal := tool.Baro_itoa(tool.Baro_get_int(db, "appeal_deadline_days", 14))
	threshold := tool.Baro_itoa(tool.Baro_mass_threshold(db))
	confirm := tool.Baro_itoa(tool.Baro_get_int(db, "score_confirm_days", 7))

	faq := [][]string{
		{
			tool.Baro_t("편집하려면 꼭 로그인해야 하나요?"),
			tool.Baro_t(`<p>네. 바로위키는 기여를 사람 단위로 남기기 위해 로그인한 사용자만 편집할 수 있습니다. `) +
				tool.Baro_t(`가입할 때 실명·전화번호·생년월일은 요구하지 않습니다.</p>`) +
				tool.Baro_button("/register", tool.Baro_t("가입하기"), ""),
		},
		{
			tool.Baro_t("잘못 저장하면 어떻게 되나요?"),
			tool.Baro_t(`<p>모든 판이 그대로 남습니다. 문서 역사에서 이전 판으로 되돌리면 됩니다. `) +
				tool.Baro_t(`실수로 지운 내용도 사라지지 않습니다.</p>`),
		},
		{
			tool.Baro_t("내 편집이 되돌려졌습니다. 왜인가요?"),
			tool.Baro_t(`<p>대개 출처가 없거나 다른 자료와 어긋나기 때문입니다. 되돌린 사람은 사유를 남기게 되어 있고, `) +
				tool.Baro_t(`알림으로 그 사유가 전달됩니다. 납득이 되지 않으면 문서 토론에서 근거를 들어 이야기하세요.</p>`) +
				tool.Baro_button("/baro/notify", tool.Baro_t("내 알림 보기"), ""),
		},
		{
			tool.Baro_t("문서가 삭제되나요?"),
			tool.Baro_t(`<p>기본 경로는 삭제가 아니라 임시 격리입니다. 격리된 문서는 보이지 않을 뿐 내용은 보존되며, `) +
				tool.Baro_t(`검토 뒤 수정·복구·삭제 중 하나로 처리합니다. 조치 사유는 공개 로그에 남습니다.</p>`) +
				tool.Baro_button("/baro/log", tool.Baro_t("공개 운영 로그"), ""),
		},
		{
			tool.Baro_t("한 번에 여러 문서를 지우는 일도 있나요?"),
			`<p>` + threshold + tool.Baro_t(`개를 넘는 작업은 대량 작업으로 분류합니다. 대상 목록 공개, 운영자 2인 승인, `) +
				tool.Baro_t(`복구 스냅샷, 유예 시간을 모두 거쳐야 실행됩니다. 실행 뒤에도 일괄 복구할 수 있습니다.</p>`) +
				tool.Baro_button("/baro/mass", tool.Baro_t("대량 작업 현황"), ""),
		},
		{
			tool.Baro_t("기여 점수는 언제 반영되나요?"),
			tool.Baro_t(`<p>저장 즉시 점수가 오르지 않습니다. 약 `) + confirm + tool.Baro_t(`일 뒤 그 기여가 살아남았는지 확인한 다음 확정합니다. `) +
				tool.Baro_t(`오타 수정 1,000번이 좋은 문서 100편과 같아지지 않도록 종류별로 다르게 셉니다.</p>`) +
				tool.Baro_button("/baro/rank", tool.Baro_t("기여 인정 방식"), ""),
		},
		{
			tool.Baro_t("제재를 받으면 내 문서도 지워지나요?"),
			tool.Baro_t(`<p>아닙니다. 제재는 사람에게만 적용합니다. 이용자가 차단됐다는 이유만으로 정상적인 기여를 지우지 않습니다. `) +
				tool.Baro_t(`다만 저작권 침해나 개인정보 노출처럼 콘텐츠 자체에 문제가 있으면 따로 처리합니다.</p>`),
		},
		{
			tool.Baro_t("조치가 부당하다고 생각되면요?"),
			tool.Baro_t(`<p>조치일로부터 `) + appeal + tool.Baro_t(`일 안에 이의제기를 신청할 수 있습니다. `) +
				tool.Baro_t(`원래 조치를 한 담당자는 심사에서 빠집니다.</p>`) +
				tool.Baro_button("/baro/appeal", "이의제기 신청", ""),
		},
		{
			tool.Baro_t("내가 쓴 글의 저작권은요?"),
			`<p>` + license + tool.Baro_t(` 로 공개됩니다. 다른 사람이 고쳐 쓸 수 있고, 다시 쓸 때는 출처를 밝혀야 합니다. `) +
				tool.Baro_t(`계정을 지워도 이미 공개된 정상 기여는 자동으로 삭제되지 않습니다.</p>`) +
				tool.Baro_button("/baro/copyright", tool.Baro_t("저작권 안내"), ""),
		},
		{
			tool.Baro_t("내 데이터를 내려받거나 지울 수 있나요?"),
			tool.Baro_t(`<p>개인정보 요청 센터에서 열람·수정·내려받기·삭제를 신청할 수 있습니다. `) +
				tool.Baro_t(`문서 전체 덤프는 데이터 개방 화면에서 받을 수 있습니다.</p>`) +
				tool.Baro_button("/baro/privacy", tool.Baro_t("개인정보 요청"), "") + " " +
				tool.Baro_button("/baro/export", tool.Baro_t("내 데이터 내려받기"), ""),
		},
		{
			tool.Baro_t("무엇부터 하면 좋을까요?"),
			tool.Baro_t(`<p>관심 분야를 고르면 그 분야의 쉬운 과제를 추천해 드립니다. `) +
				tool.Baro_t(`오타 수정 한 건으로 시작해도 창립 기여자 자격에는 문제가 없습니다.</p>`) +
				tool.Baro_button("/baro/onboarding", tool.Baro_t("첫 기여 도우미"), "primary") + " " +
				tool.Baro_button("/baro/task", tool.Baro_t("할 일 목록"), ""),
		},
		{
			tool.Baro_t("연습해 볼 곳이 있나요?"),
			tool.Baro_t(`<p>연습장에서 마음껏 시험해 보세요. 연습장에 쓴 글은 문서로 공개되지 않습니다.</p>`) +
				tool.Baro_button("/baro/draft", tool.Baro_t("연습장 열기"), ""),
		},
	}

	cards := []string{}
	for _, item := range faq {
		cards = append(cards, tool.Baro_card(item[0], item[1]))
	}

	body := tool.Baro_grid(cards, 2)
	body += tool.Baro_note(tool.Baro_t("찾는 답이 없다면 문제 신고나 문서 토론으로 물어보세요."))
	body += tool.Baro_button("/baro/report", tool.Baro_t("문의·신고"), "") + " " +
		tool.Baro_button("/baro/map", tool.Baro_t("화면 지도"), "")

	return tool.Baro_card(tool.Baro_t("자주 묻는 질문"), body)
}
