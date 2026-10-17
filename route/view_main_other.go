package route

import (
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  기타 도구 — /other
//
//  「그 화면으로 가는 길이 하나뿐」 이라는 말을 없애기 위한 화면이다.
//  위키의 모든 기능 화면을 묶음별로 모아 두고, 위쪽 칸에 글자를 넣으면
//  그 자리에서 걸러 볼 수 있게 한다.
//
//  묶음 하나를 늘릴 때는 other_tool_group 에 한 줄만 더하면 된다.
// ============================================================

// other_tool_item : 화면 하나 (주소, 이름, 한 줄 설명)
type other_tool_item struct {
	Url  string
	Name string
	Note string
}

// other_tool_group : 화면 묶음
type other_tool_group struct {
	Name  string
	Items []other_tool_item
}

// other_tool_groups : 화면 묶음 정의
func other_tool_groups(admin bool) []other_tool_group {
	groups := []other_tool_group{
		{
			Name: tool.Lang("document"),
			Items: []other_tool_item{
				{"/recent_changes", tool.Lang("recent_change"), tool.Baro_t("방금 바뀐 문서를 봅니다.")},
				{"/recent_discuss", tool.Lang("recent_discussion"), tool.Baro_t("방금 열린 토론을 봅니다.")},
				{"/random", tool.Lang("random"), tool.Baro_t("아무 문서나 하나 열어 봅니다.")},
				{"/list/document/all", tool.Lang("all_document_list"), tool.Baro_t("문서를 이름 차례로 모두 봅니다.")},
				{"/list/document/need", tool.Lang("need_document"), tool.Baro_t("링크만 걸려 있고 아직 없는 문서입니다.")},
				{"/list/document/long", tool.Lang("long_page"), tool.Baro_t("본문이 긴 문서입니다.")},
				{"/list/document/short", tool.Lang("short_page"), tool.Baro_t("본문이 짧은 문서입니다.")},
				{"/list/document/old", tool.Lang("old_page"), tool.Baro_t("오래 손대지 않은 문서입니다.")},
				{"/list/document/new", tool.Lang("new_page"), tool.Baro_t("가장 늦게 만들어진 문서입니다.")},
				{"/list/document/no_link", tool.Lang("no_link_document_list"), tool.Baro_t("어디서도 링크되지 않은 문서입니다.")},
				{"/list/document/acl", tool.Lang("acl_document_list"), tool.Baro_t("편집이 제한된 문서입니다.")},
				{"/namespace", tool.Lang("namespace"), tool.Baro_t("이름공간별로 문서를 봅니다.")},
				{"/trash", tool.Baro_t("휴지통"), tool.Baro_t("지워진 문서를 봅니다.")},
			},
		},
		{
			Name: tool.Baro_t("내 기여"),
			Items: []other_tool_item{
				{"/baro/record", tool.Lang("contribution"), tool.Baro_t("편집 · 토론 · 요청 기록을 한자리에서 봅니다.")},
				{"/baro/land", tool.Lang("contribution_land"), tool.Baro_t("누가 어느 문서의 주인인지 땅으로 봅니다.")},
				{"/baro/profile", tool.Lang("my_contribution"), tool.Baro_t("기여 요약과 배지를 봅니다.")},
				{"/baro/rank", tool.Baro_t("기여 순위"), tool.Baro_t("기간별 기여 순위를 봅니다.")},
				{"/baro/contrib", tool.Baro_t("기여 이벤트"), tool.Baro_t("기여로 인정된 사건을 봅니다.")},
				{"/baro/badge", tool.Baro_t("배지"), tool.Baro_t("받을 수 있는 배지와 조건을 봅니다.")},
				{"/watch_list", tool.Lang("watchlist"), tool.Baro_t("주시하는 문서의 변화를 봅니다.")},
				{"/star_doc", tool.Lang("star_doc"), tool.Baro_t("별을 달아 둔 문서를 봅니다.")},
				{"/change/social", tool.Lang("social_account"), tool.Baro_t("유튜브 · 인스타그램 · 블로그 계정을 걸어 둡니다.")},
			},
		},
		{
			Name: tool.Lang("request_center"),
			Items: []other_tool_item{
				{"/baro/req", tool.Lang("request_center"), tool.Baro_t("편집 · 이동 · 삭제 · 되돌림 요청을 모아 봅니다.")},
				{"/baro/req/kind/edit", tool.Baro_t("편집 요청"), tool.Baro_t("본문을 이렇게 바꿔 달라고 부탁합니다.")},
				{"/baro/req/kind/move", tool.Baro_t("이동 요청"), tool.Baro_t("문서 이름을 바꿔 달라고 부탁합니다.")},
				{"/baro/req/kind/delete", tool.Baro_t("삭제 요청"), tool.Baro_t("문서를 지워 달라고 부탁합니다.")},
				{"/baro/req/kind/revert", tool.Baro_t("되돌림 요청"), tool.Baro_t("예전 판으로 되돌려 달라고 부탁합니다.")},
				{"/baro/req/log", tool.Baro_t("요청 처리 기록"), tool.Baro_t("누가 어떤 요청을 어떻게 처리했는지 봅니다.")},
				{"/baro/report/new", tool.Baro_t("신고하기"), tool.Baro_t("규정을 어긴 편집이나 글을 알립니다.")},
				{"/baro/appeal", tool.Baro_t("제재 소명"), tool.Baro_t("제재에 대해 다시 봐 달라고 요청합니다.")},
			},
		},
		{
			Name: tool.Baro_t("놀이 · 발견"),
			Items: []other_tool_item{
				{"/baro/game", tool.Lang("baro_game"), tool.Baro_t("문서 맞히기와 위키 레이스를 합니다.")},
				{"/baro/game/quiz", tool.Baro_t("문서 맞히기"), tool.Baro_t("첫 대목만 보고 어떤 문서인지 맞힙니다.")},
				{"/baro/game/race", tool.Baro_t("위키 레이스"), tool.Baro_t("링크만 밟아 도착 문서까지 갑니다.")},
				{"/baro/game/rank", tool.Baro_t("바로게임 순위표"), tool.Baro_t("놀이별 잘한 순서를 봅니다.")},
				{"/baro/discover", tool.Baro_t("발견"), tool.Baro_t("읽을 만한 문서를 추천받습니다.")},
				{"/baro/ceremony", tool.Lang("wiki_award"), tool.Baro_t("위키 수상식 후보와 결과를 봅니다.")},
				{"/baro/wrapped", tool.Baro_t("연말 결산"), tool.Baro_t("한 해의 기여를 정리해 봅니다.")},
			},
		},
		{
			Name: tool.Lang("wiki_link"),
			Items: []other_tool_item{
				{"/baro/wikilink", tool.Lang("wiki_link"), tool.Baro_t("다른 위키의 같은 문서를 이어 두고 비교합니다.")},
				{"/baro/wikilink/wiki", tool.Baro_t("등록된 위키"), tool.Baro_t("어떤 위키와 이어져 있는지 봅니다.")},
				{"/baro/import/url", tool.Baro_t("옛 주소 연결"), tool.Baro_t("예전 주소로 들어와도 새 문서로 이어 줍니다.")},
				{"/baro/api", tool.Baro_t("공개 API"), tool.Baro_t("읽기 API 사용법과 요청 한도를 봅니다.")},
				{"/baro/export", tool.Baro_t("내보내기"), tool.Baro_t("문서 덤프와 내 데이터를 내려받습니다.")},
			},
		},
		{
			Name: tool.Baro_t("함께 만들기"),
			Items: []other_tool_item{
				{"/baro", tool.Baro_t("바로위키 홈"), tool.Baro_t("지금 무엇이 진행되고 있는지 한눈에 봅니다.")},
				{"/baro/founding", tool.Baro_t("창립 시즌"), tool.Baro_t("창립 기여자 모집 현황을 봅니다.")},
				{"/baro/field", tool.Baro_t("분야"), tool.Baro_t("분야별 개척률과 남은 몫을 봅니다.")},
				{"/baro/project", tool.Baro_t("프로젝트"), tool.Baro_t("함께 채우는 목표와 진행률을 봅니다.")},
				{"/baro/task", tool.Baro_t("할 일"), tool.Baro_t("지금 열려 있는 과제를 난이도별로 찾습니다.")},
				{"/baro/talk", tool.Baro_t("이야기방"), tool.Baro_t("문서 · 프로젝트 · 사용자별 이야기입니다.")},
				{"/baro/draft", tool.Baro_t("내 초안"), tool.Baro_t("쓰다 만 문서를 모아 둡니다.")},
				{"/baro/draft/sandbox", tool.Baro_t("연습장"), tool.Baro_t("문법을 마음껏 시험해 봅니다.")},
				{"/baro/help", tool.Baro_t("도움말"), tool.Baro_t("30초 편집 안내와 문법 요약을 봅니다.")},
				{"/baro/map", tool.Lang("all_tab"), tool.Baro_t("바로위키의 모든 주소를 한 표로 봅니다.")},
			},
		},
		{
			Name: tool.Lang("bbs"),
			Items: []other_tool_item{
				{"/bbs/main", tool.Lang("bbs_main"), tool.Baro_t("게시판을 고르고 글을 씁니다.")},
				{"/vote", tool.Lang("vote_list"), tool.Baro_t("열려 있는 투표를 봅니다.")},
				{"/upload", tool.Lang("upload"), tool.Baro_t("그림과 파일을 올립니다.")},
				{"/list/file", tool.Lang("image_file_list"), tool.Baro_t("올라온 파일을 봅니다.")},
			},
		},
		{
			Name: tool.Lang("user"),
			Items: []other_tool_item{
				{"/user", tool.Lang("user_tool"), tool.Baro_t("다른 사용자의 기록과 도구를 봅니다.")},
				{"/change", tool.Lang("user_setting"), tool.Baro_t("스킨 · 언어 · 계정 설정입니다.")},
				{"/change/skin_set", tool.Lang("skin_setting"), tool.Baro_t("본문 보기 방식을 바꿉니다.")},
				{"/list/user", tool.Lang("member_list"), tool.Baro_t("가입한 사용자를 봅니다.")},
				{"/recent_block", tool.Lang("recent_ban"), tool.Baro_t("최근 차단 기록입니다.")},
				{"/baro/security/2fa", tool.Baro_t("2단계 인증"), tool.Baro_t("계정을 한 겹 더 지킵니다.")},
			},
		},
		{
			Name: tool.Baro_t("규정 · 투명성"),
			Items: []other_tool_item{
				{"/baro/policy", tool.Baro_t("정책"), tool.Baro_t("규정 전문과 개정 이력을 봅니다.")},
				{"/baro/log", tool.Baro_t("공개 운영 로그"), tool.Baro_t("어떤 조치를 왜 했는지 봅니다.")},
				{"/baro/transparency", tool.Baro_t("투명성 보고서"), tool.Baro_t("기간별 운영 통계를 봅니다.")},
				{"/baro/status", tool.Baro_t("서비스 상태"), tool.Baro_t("장애 · 점검 이력을 봅니다.")},
				{"/baro/privacy", tool.Baro_t("개인정보"), tool.Baro_t("처리 방침과 요청 신청입니다.")},
				{"/baro/copyright", tool.Baro_t("저작권"), tool.Baro_t("라이선스 안내와 침해 신고입니다.")},
				{"/filter/edit_filter", tool.Baro_t("편집 필터 규칙"), tool.Baro_t("걸러지는 규칙을 봅니다.")},
			},
		},
	}

	if admin {
		groups = append(groups, other_tool_group{
			Name: tool.Lang("admin"),
			Items: []other_tool_item{
				{"/manager", tool.Lang("admin_tool"), tool.Baro_t("위키 전체 설정입니다.")},
				{"/baro/admin", tool.Baro_t("바로위키 관리"), tool.Baro_t("바로위키 기능 관리 화면입니다.")},
				{"/baro/quality/filter", tool.Baro_t("편집 필터 관리"), tool.Baro_t("규칙을 만들고 조건식을 시험합니다.")},
				{"/baro/review", tool.Baro_t("검토 큐"), tool.Baro_t("위험해 보이는 편집을 확인합니다.")},
				{"/baro/report/queue", tool.Baro_t("신고 처리"), tool.Baro_t("들어온 신고를 처리합니다.")},
				{"/baro/wikilink/new", tool.Baro_t("위키 등록"), tool.Baro_t("연동할 다른 위키를 등록합니다.")},
				{"/baro/wikilink/set", tool.Baro_t("AI 비교 설정"), tool.Baro_t("문서 비교에 쓸 AI 주소를 넣습니다.")},
				{"/list/admin", tool.Lang("admin_list"), tool.Baro_t("관리자 목록입니다.")},
				{"/list/admin/auth_use", tool.Lang("authority_use_list"), tool.Baro_t("권한 사용 기록입니다.")},
				{"/baro/security/audit", tool.Baro_t("감사 기록"), tool.Baro_t("관리 행동 기록입니다.")},
			},
		})
	}

	return groups
}

// other_tool_html : 묶음을 카드로 그린다
func other_tool_html(groups []other_tool_group) string {
	html := ""
	for _, group := range groups {
		card := `<div class="namu_tool_grid">`
		for _, item := range group.Items {
			card += `<a class="namu_tool_item" href="` + tool.HTML_escape(item.Url) + `"` +
				` data-find="` + tool.HTML_escape(strings.ToLower(item.Name+" "+item.Note+" "+item.Url)) + `">` +
				`<span class="namu_tool_name">` + tool.HTML_escape(item.Name) + `</span>` +
				`<span class="namu_tool_note">` + tool.HTML_escape(item.Note) + `</span></a>`
		}
		card += `</div>`

		html += `<section class="namu_tool_group"><h2>` + tool.HTML_escape(group.Name) + `</h2>` + card + `</section>`
	}

	return html
}

func View_main_other(config tool.Config) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	admin := tool.Check_acl(db, "", "", "owner_auth", config.IP)

	data := `<input class="__ON_INPUT__ namu_tool_find" id="namu_tool_find" type="search" autocomplete="off" placeholder="` +
		tool.Lang_esc("tool_find_placeholder") + `" aria-label="` + tool.Lang_esc("search") + `">`
	data += `<p class="namu_tool_help">` + tool.Lang_esc("tool_find_help") + `</p>`
	data += `<div id="namu_tool_list">` + other_tool_html(other_tool_groups(admin)) + `</div>`
	data += `<p class="namu_tool_empty" id="namu_tool_empty" style="display: none;">` + tool.Lang_esc("tool_find_empty") + `</p>`
	data += `
        <script>
        (function () {
            var find = document.getElementById('namu_tool_find');
            var list = document.getElementById('namu_tool_list');
            var empty = document.getElementById('namu_tool_empty');
            if (find === null || list === null) { return; }

            var items = list.querySelectorAll('.namu_tool_item');
            var groups = list.querySelectorAll('.namu_tool_group');

            find.addEventListener('input', function () {
                var word = find.value.trim().toLowerCase();
                var shown = 0;

                for (var index = 0; index < items.length; index++) {
                    var hit = word === '' || (items[index].dataset.find || '').indexOf(word) >= 0;
                    items[index].hidden = !hit;
                    if (hit) { shown++; }
                }

                for (var group = 0; group < groups.length; group++) {
                    var alive = groups[group].querySelectorAll('.namu_tool_item:not([hidden])').length;
                    groups[group].hidden = alive === 0;
                }

                empty.style.display = shown === 0 ? '' : 'none';
            });
        })();
        </script>`

	return tool.Get_template(
		db,
		config,
		tool.Get_language(db, "other_tool", true),
		data,
		[]any{},
		[][]any{
			{"baro/map", tool.Lang("all_tab")},
			{"", ""},
		},
		map[string]string{},
	)
}
