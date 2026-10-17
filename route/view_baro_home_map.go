package route

import (
	"database/sql"
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  19장 화면 지도 (/baro/map)
//
//  바로위키의 모든 주요 주소를
//   - 공개 화면 (누구나)
//   - 로그인 사용자 화면
//   - 관리자 · 운영자 화면
//  세 묶음으로 정리해 링크한다.
// ============================================================

// baro_home_map_group : 화면 묶음
type baro_home_map_group struct {
	Key   string
	Name  string
	Note  string
	Items [][]string // {주소, 이름, 한 줄 설명}
}

// baro_home_map_groups : 화면 지도 정의
func baro_home_map_groups() []baro_home_map_group {
	return []baro_home_map_group{
		{
			Key:  "public",
			Name: tool.Baro_t("공개 화면"),
			Note: tool.Baro_t("로그인하지 않아도 볼 수 있는 화면입니다."),
			Items: [][]string{
				{"/baro", tool.Baro_t("바로위키 홈"), tool.Baro_t("창립 시즌 요약과 사이트 지표를 한눈에 봅니다.")},
				{"/baro/founding", tool.Baro_t("창립 시즌"), tool.Baro_t("바로위키를 만드는 이유와 창립 기여자 모집 현황을 봅니다.")},
				{"/baro/about", tool.Baro_t("바로위키 소개"), tool.Baro_t("0장 고정 원칙 전문과 운영 약속을 봅니다.")},
				{"/baro/help", tool.Baro_t("도움말"), tool.Baro_t("30초 편집 안내, 문법 요약표, 출처 다는 법을 봅니다.")},
				{"/baro/map", tool.Baro_t("화면 지도"), tool.Baro_t("바로위키의 모든 주소를 한 표로 봅니다.")},
				{"/baro/field", "분야", tool.Baro_t("분야별 개척률과 남은 몫을 봅니다.")},
				{"/baro/project", "프로젝트", tool.Baro_t("함께 채우는 목표와 진행률을 봅니다.")},
				{"/baro/task", tool.Baro_t("할 일"), tool.Baro_t("지금 열려 있는 과제를 난이도별로 찾습니다.")},
				{"/baro/rank", tool.Baro_t("기여 순위"), tool.Baro_t("기여 점수와 기간별 순위를 봅니다.")},
				{"/baro/badge", "배지", tool.Baro_t("받을 수 있는 배지와 조건을 봅니다.")},
				{"/baro/discover", tool.Baro_t("발견"), tool.Baro_t("읽을 만한 문서를 추천받습니다.")},
				{"/baro/search", tool.Baro_t("검색"), tool.Baro_t("제목·본문·초성으로 문서를 찾습니다.")},
				{"/baro/log", tool.Baro_t("공개 운영 로그"), tool.Baro_t("어떤 조치를 왜 했는지 확인합니다.")},
				{"/baro/policy", "정책", tool.Baro_t("규정 전문과 개정 이력을 봅니다.")},
				{"/baro/transparency", tool.Baro_t("투명성 보고서"), tool.Baro_t("기간별 운영 통계를 봅니다.")},
				{"/baro/award", tool.Baro_t("어워드"), tool.Baro_t("MVP·올해의 위키인 후보와 결과를 봅니다.")},
				{"/baro/wrapped", tool.Baro_t("연말 결산"), tool.Baro_t("한 해의 기여를 정리해 봅니다.")},
				{"/baro/privacy", tool.Baro_t("개인정보"), tool.Baro_t("개인정보 처리 방침과 요청 신청을 봅니다.")},
				{"/baro/copyright", tool.Baro_t("저작권"), tool.Baro_t("라이선스 안내와 권리 침해 신고를 봅니다.")},
				{"/baro/export", tool.Baro_t("내보내기"), tool.Baro_t("문서 덤프와 내 데이터 내려받기를 신청합니다.")},
				{"/baro/api", tool.Baro_t("공개 API"), tool.Baro_t("읽기 API 사용법과 요청 한도를 봅니다.")},
				{"/baro/status", tool.Baro_t("서비스 상태"), tool.Baro_t("장애·점검 이력과 현재 상태를 봅니다.")},
				{"/baro/land", tool.Baro_t("기여 땅따먹기"), tool.Baro_t("문서 한 개를 땅 한 칸으로 보고 누가 주인인지 봅니다.")},
				{"/baro/game", tool.Baro_t("바로게임"), tool.Baro_t("문서 맞히기와 위키 레이스를 합니다.")},
				{"/baro/game/rank", tool.Baro_t("바로게임 순위표"), tool.Baro_t("놀이별 잘한 순서를 봅니다.")},
				{"/baro/shop", "바로샵", tool.Baro_t("모은 밐으로 칭호와 캡차 면제권을 삽니다.")},
				{"/baro/cooperation", tool.Baro_t("편집방"), tool.Baro_t("여럿이 모여 문서를 함께 손봅니다.")},
				{"/baro/theme", tool.Baro_t("테마"), tool.Baro_t("위키 색을 꾸며 두고 불러다 씁니다.")},
				{"/baro/req", tool.Baro_t("요청"), tool.Baro_t("편집 · 이동 · 삭제 · 되돌림 요청을 모아 봅니다.")},
				{"/baro/wikilink", tool.Baro_t("위키 데이터 연동"), tool.Baro_t("다른 위키의 같은 문서를 이어 두고 비교합니다.")},
				{"/baro/record", tool.Baro_t("기여 기록"), tool.Baro_t("편집 · 토론 · 요청 기록을 한자리에서 봅니다.")},
				{"/other", tool.Baro_t("기능 모아 보기"), tool.Baro_t("위키의 모든 기능을 묶음별로 찾습니다.")},
			},
		},
		{
			Key:  "user",
			Name: tool.Baro_t("로그인 사용자 화면"),
			Note: tool.Baro_t("로그인해야 쓸 수 있습니다. 기여 기록이 사람 단위로 남습니다."),
			Items: [][]string{
				{"/baro/onboarding", tool.Baro_t("첫 기여 도우미"), tool.Baro_t("관심 분야를 고르고 첫 과제를 안내받습니다.")},
				{"/baro/profile", tool.Baro_t("내 프로필"), tool.Baro_t("내 기여·배지·관심 문서를 모아 봅니다.")},
				{"/baro/mik", tool.Baro_t("밐 지갑"), tool.Baro_t("가진 밐과 오간 기록을 보고 칭호를 답니다.")},
				{"/baro/mik/send", tool.Baro_t("밐 양도"), tool.Baro_t("고유 번호로 다른 사용자에게 밐을 보냅니다.")},
				{"/baro/avatar", tool.Baro_t("프로필 사진"), tool.Baro_t("그라바타 대신 쓸 얼굴 그림을 올리고 지웁니다.")},
				{"/baro/draft", tool.Baro_t("연습장"), tool.Baro_t("공개되지 않는 곳에서 편집을 연습합니다.")},
				{"/baro/notify", tool.Baro_t("알림"), tool.Baro_t("되돌림·답글·검토 결과 알림을 확인합니다.")},
				{"/baro/invite", tool.Baro_t("친구 초대"), tool.Baro_t("내 초대 링크를 만들고 초대 현황을 봅니다.")},
				{"/baro/report", tool.Baro_t("문제 신고"), tool.Baro_t("문서·이용자 문제를 근거와 함께 신고합니다.")},
				{"/baro/appeal", tool.Baro_t("이의제기"), tool.Baro_t("받은 조치에 대해 재심을 신청합니다.")},
				{"/change/social", tool.Baro_t("연결된 계정"), tool.Baro_t("유튜브 · 인스타그램 · 블로그 계정을 걸어 둡니다.")},
				{"/baro/req/new?kind=move", tool.Baro_t("이동 요청 만들기"), tool.Baro_t("문서 이름을 바꿔 달라고 부탁합니다.")},
				{"/baro/req/new?kind=delete", tool.Baro_t("삭제 요청 만들기"), tool.Baro_t("문서를 지워 달라고 부탁합니다.")},
				{"/baro/req/new?kind=revert", tool.Baro_t("되돌림 요청 만들기"), tool.Baro_t("예전 판으로 되돌려 달라고 부탁합니다.")},
			},
		},
		{
			Key:  "admin",
			Name: tool.Baro_t("관리자 · 운영자 화면"),
			Note: tool.Baro_t("역할을 가진 사람만 들어갈 수 있습니다. 권한이 없으면 안내 화면이 뜹니다."),
			Items: [][]string{
				{"/baro/admin", tool.Baro_t("백오피스"), tool.Baro_t("운영 도구를 한곳에서 엽니다.")},
				{"/baro/review", tool.Baro_t("검토 큐"), tool.Baro_t("위험도가 높은 편집을 먼저 검토합니다.")},
				{"/baro/quality", tool.Baro_t("문서 품질"), tool.Baro_t("출처 없는 문서와 중복 문서를 관리합니다.")},
				{"/baro/mass", tool.Baro_t("대량 작업"), tool.Baro_t("2인 승인·스냅샷·유예를 거쳐 대량 작업을 실행합니다.")},
				{"/baro/case", "사건", tool.Baro_t("신고를 사건으로 묶어 처리하고 근거를 남깁니다.")},
				{"/baro/import", tool.Baro_t("문서 가져오기"), tool.Baro_t("외부 문서를 라이선스 기록과 함께 들여옵니다.")},
				{"/baro/metrics", tool.Baro_t("분석 지표"), tool.Baro_t("기여·품질·운영 지표를 기간별로 봅니다.")},
				{"/baro/security", tool.Baro_t("보안"), tool.Baro_t("2FA·악용 방지·감사 로그를 관리합니다.")},
				{"/baro/wikilink/new", tool.Baro_t("연동 위키 등록"), tool.Baro_t("문서를 비교할 다른 위키를 등록합니다.")},
				{"/baro/wikilink/set", tool.Baro_t("AI 비교 설정"), tool.Baro_t("문서 비교에 쓸 AI 주소를 넣습니다.")},
				{"/baro/quality/filter", tool.Baro_t("편집 필터"), tool.Baro_t("정규식과 조건식으로 규칙을 만듭니다.")},
			},
		},
	}
}

// View_baro_home_map : 19장 화면 지도
func View_baro_home_map(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	_ = values

	keyword := ""
	if query != nil {
		keyword = strings.TrimSpace(query.Get("q"))
	}

	lower := strings.ToLower(keyword)

	body := baro_home_notice(query)
	body += baro_home_map_hero(db, config)
	body += baro_home_map_search(keyword)

	total, shown := 0, 0

	// 권한 확인은 한 번만 하고 결과를 돌려 쓴다 (표 한 줄마다 조회하지 않는다)
	state_cache := map[string]string{}

	for _, group := range baro_home_map_groups() {
		if _, ok := state_cache[group.Key]; !ok {
			state_cache[group.Key] = baro_home_map_state(db, config, group.Key)
		}

		rows := [][]string{}

		for _, item := range group.Items {
			total++

			if lower != "" &&
				!strings.Contains(strings.ToLower(item[0]), lower) &&
				!strings.Contains(strings.ToLower(item[1]), lower) &&
				!strings.Contains(strings.ToLower(item[2]), lower) {
				continue
			}

			shown++

			rows = append(rows, []string{
				`<a href="` + tool.HTML_escape(item[0]) + `">` + tool.HTML_escape(item[1]) + `</a>`,
				tool.Baro_code(item[0]),
				tool.HTML_escape(item[2]),
				state_cache[group.Key],
			})
		}

		inner := tool.Baro_note(group.Note)
		inner += tool.Baro_table(
			[]string{tool.Baro_t("화면"), tool.Baro_t("주소"), tool.Baro_t("설명"), tool.Baro_t("지금 내 상태")},
			rows,
			tool.Baro_t("검색어에 맞는 화면이 없습니다."),
		)

		body += tool.Baro_card(group.Name+" ("+tool.Baro_itoa(len(group.Items))+tool.Baro_t("개)"), inner)
	}

	body += baro_home_map_foot(total, shown, keyword)

	return tool.Baro_page(db, config, tool.Baro_t("화면 지도"), body, baro_home_menu())
}

// baro_home_map_hero : 화면 지도 머리말
func baro_home_map_hero(db *sql.DB, config tool.Config) string {
	groups := baro_home_map_groups()

	stats := [][]string{}
	total := 0
	for _, group := range groups {
		total += len(group.Items)
		stats = append(stats, []string{group.Name, tool.Baro_itoa(len(group.Items)) + tool.Baro_t("개"), ""})
	}

	stats = append(stats, []string{tool.Baro_t("전체"), tool.Baro_itoa(total) + tool.Baro_t("개"), tool.Baro_t("바로위키 주요 주소")})

	state := tool.Baro_t("비로그인 상태입니다. 공개 화면만 열립니다.")
	if tool.Baro_is_admin(db, config.IP) {
		state = tool.Baro_t("관리자 권한이 있습니다. 운영자 화면까지 열립니다.")
	} else if tool.Baro_is_user(config.IP) {
		state = tool.Baro_t("로그인 상태입니다. 사용자 화면까지 열립니다.")
	}

	return tool.Baro_hero(
		tool.Baro_t("화면 지도"),
		tool.Baro_t("바로위키의 모든 주요 주소를 한 표에 모았습니다."),
		tool.Baro_stat_row(stats)+tool.Baro_alert("info", tool.HTML_escape(state)),
	)
}

// baro_home_map_search : 주소 찾기 상자
func baro_home_map_search(keyword string) string {
	form := `<form class="bw_form" method="get" action="/baro/map">`
	form += tool.Baro_input("q", tool.Baro_t("화면 찾기"), keyword, tool.Baro_t("예: 신고, /baro/task, 출처"), tool.Baro_t("이름·주소·설명에서 찾습니다."))
	form += tool.Baro_submit(tool.Baro_t("찾기"), "primary")
	if keyword != "" {
		form += " " + tool.Baro_button("/baro/map", tool.Baro_t("전체 보기"), "")
	}
	form += `</form>`

	return tool.Baro_card(tool.Baro_t("화면 찾기"), form)
}

// baro_home_map_state : 지금 이 사용자가 그 묶음을 쓸 수 있는지
func baro_home_map_state(db *sql.DB, config tool.Config, group string) string {
	switch group {
	case "public":
		return tool.Baro_badge(tool.Baro_t("열림"), "ok")
	case "user":
		if tool.Baro_is_user(config.IP) {
			return tool.Baro_badge(tool.Baro_t("열림"), "ok")
		}

		return tool.Baro_badge(tool.Baro_t("로그인 필요"), "warn")
	}

	if tool.Baro_is_admin(db, config.IP) || tool.Baro_can(db, config.IP, "review") {
		return tool.Baro_badge(tool.Baro_t("열림"), "ok")
	}

	return tool.Baro_badge(tool.Baro_t("권한 필요"), "muted")
}

// baro_home_map_foot : 화면 지도 꼬리말
func baro_home_map_foot(total int, shown int, keyword string) string {
	body := ""

	if keyword != "" {
		body += tool.Baro_note("\"" + keyword + "\" 로 찾은 결과 " + tool.Baro_itoa(shown) + " / " + tool.Baro_itoa(total) + tool.Baro_t("개"))
	} else {
		body += tool.Baro_note(tool.Baro_t("모두 ") + tool.Baro_itoa(total) + tool.Baro_t("개의 화면이 있습니다."))
	}

	body += tool.Baro_alert("info",
		tool.Baro_t("권한이 없는 화면에 들어가도 오류가 아니라 안내 화면이 뜹니다. 무엇이 필요한지 그 화면에서 알려 드립니다."))

	body += tool.Baro_button("/baro", tool.Baro_t("홈으로"), "primary") + " " +
		tool.Baro_button("/baro/help", tool.Baro_t("도움말"), "") + " " +
		tool.Baro_button("/baro/about", tool.Baro_t("소개"), "")

	return tool.Baro_card(tool.Baro_t("안내"), body)
}
