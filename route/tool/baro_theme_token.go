package tool

// ============================================================
//  테마 토큰 목록
//
//  스킨은 이미 --namu-* 라는 디자인 토큰으로 색을 다 뽑아 두었다.
//  테마는 그 토큰을 덮어쓰는 것이 전부다. 선택자를 새로 만들지 않는다.
//  그래서 스킨이 바뀌어도 테마가 깨지지 않는다.
//
//  값은 "밝은쪽|어두운쪽" 한 줄로 담는다. 한쪽을 비우면 스킨 값을 그대로 쓴다.
// ============================================================

// Baro_theme_token : 고칠 수 있는 색 하나
type Baro_theme_token struct {
	Key   string // 테마 안에서 쓰는 이름
	Var   string // 실제 CSS 변수
	Group string // 묶음
	Name  string // 사람이 읽는 이름
	Light string // 스킨의 밝은 쪽 값
	Dark  string // 스킨의 어두운 쪽 값
}

// Baro_theme_token_list : 고칠 수 있는 색 전부
//
//	「바로위키 색상」 표를 그대로 옮긴다. 차례도 표를 따른다.
func Baro_theme_token_list() []Baro_theme_token {
	return []Baro_theme_token{
		// --- 바탕과 글자 ---
		{"page_bg", "--namu-background-color", "바탕과 글자", "페이지 배경", "#ffffff", "#1c1d1f"},
		{"ground_bg", "--namu-ground-color", "바탕과 글자", "페이지 바깥 여백", "#f5f5f5", "#1c1d1f"},
		{"text", "--namu-text-color", "바탕과 글자", "텍스트 (문서명·문단명·본문)", "#212529", "#e0e0e0"},
		{"comment", "--namu-comment-color", "바탕과 글자", "주석", "#008000", "#608b4e"},
		{"strike", "--namu-strike-color", "바탕과 글자", "취소선", "#808080", "#808080"},

		// --- 표 ---
		{"table_bg", "--namu-table-background-color", "표", "표 배경", "#f5f5f5", "#2d2f34"},
		{"table_border", "--namu-table-border-color", "표", "표 테두리", "#dddddd", "#383b40"},

		// --- 하이퍼링크 ---
		{"link", "--namu-link-color", "하이퍼링크", "각주 · 존재하는 문서", "#0275d8", "#ec9f19"},
		{"link_hover", "--namu-link-hover-color", "하이퍼링크", "각주 · 존재하는 문서 (호버)", "#0263b8", "#bd7f14"},
		{"link_none", "--namu-link-not-exist-color", "하이퍼링크", "존재하지 않는 문서", "#ff0000", "#dc4343"},
		{"link_none_hover", "--namu-link-not-exist-hover-color", "하이퍼링크", "존재하지 않는 문서 (호버)", "#ff0000", "#dc4343"},
		{"ext_icon", "--namu-link-external-icon-color", "하이퍼링크", "바깥 고리 기호", "#008000", "#008000"},
		{"ext_text", "--namu-link-external-color", "하이퍼링크", "외부 링크 텍스트", "#009900", "#009900"},

		// --- 문단과 구분선 ---
		{"fold_mark", "--namu-fold-mark-color", "문단과 구분선", "문단 표시 기호 (V표)", "#666666", "#666666"},
		{"heading_border", "--namu-heading-border-color", "문단과 구분선", "문단 밑줄", "#cccccc", "#383b40"},
		{"hr", "--namu-hr-color", "문단과 구분선", "수평줄", "#cccccc", "#383b40"},

		// --- 인용문 ---
		{"quote_accent", "--namu-quote-accent-color", "인용문", "줄 색깔", "#71bc6d", "#666c75"},
		{"quote_bg", "--namu-quote-background-color", "인용문", "배경", "#eeeeee", "#2d2f34"},
		{"quote_border", "--namu-quote-border-color", "인용문", "테두리", "#cccccc", "#383b40"},

		// --- 알림과 강조 ---
		{"redirect_bg", "--namu-redirect-background-color", "알림과 강조", "리다이렉트 알림 배경", "#aacdec", "#2378c3"},
		{"redirect_text", "--namu-redirect-text-color", "알림과 강조", "리다이렉트 알림 텍스트", "#0b3574", "#c0d2ee"},
		{"ns_bg", "--namu-namespace-background-color", "알림과 강조", "네임스페이스 배경", "#d4f0e3", "#545c5b"},
		{"danger", "--namu-danger-color", "알림과 강조", "위험 버튼", "#c1434f", "#c1434f"},
		{"notice_bg", "--namu-site-notice-background-color", "알림과 강조", "서버 점검 알림바 배경", "#f0ef75", "#f0ef75"},

		// --- 글자수 변화 알림 ---
		{"len_minus", "--namu-length-minus-color", "글자수 변화 알림", "감소", "#ff0000", "#ff0000"},
		{"len_same", "--namu-length-same-color", "글자수 변화 알림", "무변화", "#808080", "#808080"},
		{"len_plus", "--namu-length-plus-color", "글자수 변화 알림", "증가", "#008000", "#008000"},

		// --- 문서 역사 비교 ---
		{"diff_del_bg", "--namu-diff-del-color", "문서 역사 비교", "배경 · 삭제", "#ffdddd", "#943838"},
		{"diff_add_bg", "--namu-diff-add-color", "문서 역사 비교", "배경 · 삽입", "#ccffcc", "#3b5a3b"},
		{"diff_del_strong", "--namu-diff-del-strong-color", "문서 역사 비교", "수정 배경 · 삭제", "#ffffff", "#d05d5d"},
		{"diff_add_strong", "--namu-diff-add-strong-color", "문서 역사 비교", "수정 배경 · 삽입", "#50ff50", "#1c751c"},
		{"diff_del_text", "--namu-diff-del-text-color", "문서 역사 비교", "수정 텍스트 · 삭제", "#999999", "#dddddd"},
		{"diff_add_text", "--namu-diff-add-text-color", "문서 역사 비교", "수정 텍스트 · 삽입", "#212529", "#e0e0e0"},

		// --- 토론 ---
		{"topic_stop_bg", "--namu-topic-stop-color", "토론", "중지됨 · 배경", "#757575", "#454545"},
		{"topic_stop_border", "--namu-topic-stop-border-color", "토론", "중지됨 · 테두리", "#c9c9c9", "#1b1b1b"},
		{"topic_close_bg", "--namu-topic-close-color", "토론", "닫힘 · 배경", "#8168b3", "#54278f"},
		{"topic_close_border", "--namu-topic-close-border-color", "토론", "닫힘 · 테두리", "#9355dc", "#312b3f"},
		{"topic_opener", "--namu-topic-opener-color", "토론", "댓글 · 발제자", "#b0d3ad", "#324432"},
		{"topic_member", "--namu-topic-member-color", "토론", "댓글 · 참가자", "#d5d5d5", "#2d2f34"},
		{"topic_bg", "--namu-topic-comment-background-color", "토론", "댓글 · 배경", "#f4f4f4", "#27292d"},
		{"topic_border", "--namu-topic-comment-border-color", "토론", "댓글 · 경계선", "#808080", "#383b40"},
		{"topic_state", "--namu-topic-state-change-color", "토론", "댓글 · 상태 변경", "#ffcc66", "#54411a"},
		{"topic_closed_bg", "--namu-topic-closed-write-background-color", "토론", "닫힌 토론 댓글 달기 · 배경", "#eceeef", "#27292d"},
		{"topic_closed_text", "--namu-topic-closed-write-text-color", "토론", "닫힌 토론 댓글 달기 · 텍스트", "#55595c", "#dddddd"},

		// --- 내비게이션 바 ---
		{"nav_bg", "--namu-nav-bar-background", "내비게이션 바", "배경", "#008275", "#2d2f34"},
		{"nav_text", "--namu-nav-bar-text-color", "내비게이션 바", "텍스트", "#ffffff", "#ffffff"},

		// --- 권한과 차단 알림 ---
		{"auth_tag", "--namu-auth-tag-color", "권한과 차단", "특수 권한 알림", "#ffa500", "#ffa500"},
		{"auth_tag_hover", "--namu-auth-tag-hover-color", "권한과 차단", "특수 권한 알림 (호버)", "#ff0000", "#ff0000"},
		{"ban_tag", "--namu-ban-tag-color", "권한과 차단", "차단 알림", "#ff0000", "#ff0000"},
		{"ban_tag_hover", "--namu-ban-tag-hover-color", "권한과 차단", "차단 알림 (호버)", "#0000ff", "#0000ff"},
		{"ban_normal", "--namu-ban-normal-color", "권한과 차단", "일반 차단", "#808080", "#808080"},
		{"ban_login", "--namu-ban-login-color", "권한과 차단", "로그인 허용 차단", "#008000", "#008000"},
		{"ban_idc", "--namu-ban-idc-color", "권한과 차단", "IDC 차단", "#f8c471", "#f8c471"},

		// --- 바로위키 색상 ---
		{"baro", "--namu-baro-color", "바로위키 색상", "바로위키 색상", "#0047ab", "#393939"},
	}
}

// Baro_theme_group_list : 묶음 이름을 차례대로 (중복 없이)
func Baro_theme_group_list() []string {
	out := []string{}
	seen := map[string]bool{}

	for _, row := range Baro_theme_token_list() {
		if seen[row.Group] {
			continue
		}

		seen[row.Group] = true
		out = append(out, row.Group)
	}

	return out
}

// Baro_theme_token_get : 이름으로 토큰 하나 찾기
func Baro_theme_token_get(key string) (Baro_theme_token, bool) {
	for _, row := range Baro_theme_token_list() {
		if row.Key == key {
			return row, true
		}
	}

	return Baro_theme_token{}, false
}

// Baro_theme_switch_list : 색이 아닌 켜고 끄는 설정 {열쇠, 이름, 설명}
func Baro_theme_switch_list() [][]string {
	return [][]string{
		{
			"fold", Baro_t("문단 접기"),
			Baro_t("문단 제목 옆의 접기 표시(V표)를 보일지 정합니다. 끄면 문단을 접을 수 없습니다."),
		},
	}
}
