package tool

import (
	"crypto/rand"
	"math/big"
	"strings"
)

// ============================================================
//  편집방 — 여럿이 모여 함께 문서를 손보는 자리
//
//  방에 들어와 있는 동안 한 편집은 그 방에서 한 편집으로 잡힌다.
//  편집 자체는 여느 때와 똑같이 문서 편집 화면에서 한다. 방은 그 둘레를 맡는다.
//    · 누가 무엇을 손봤는지 모으고
//    · 댓글과 공지로 말을 맞추고
//    · 금칙어 · 정원 · 편집 제한 같은 방 규칙을 지킨다
//
//  공개 갈래
//    public  공개      : 「열린 편집방」 목록에 오른다
//    code    일부 공개 : 목록에 오르지 않는다. 12자리 숫자 열쇠나 링크로만 들어온다
//    private 비공개    : 방장이 부른 사람만
// ============================================================

// 편집방 공개 갈래
const (
	Baro_coop_public  = "public"
	Baro_coop_code    = "code"
	Baro_coop_private = "private"
)

// 편집방 상태
const (
	Baro_coop_open    = "open"    // 열려 있음
	Baro_coop_closing = "closing" // 곧 닫음 (10분 남김)
	Baro_coop_closed  = "closed"  // 닫힘 (방장이 다시 열 수 있다)
	Baro_coop_sealed  = "sealed"  // 영구 폐쇄 (관리자만 걸고 관리자만 푼다)
)

// 방 안에서의 자리
const (
	Baro_coop_owner = "owner" // 방장
	Baro_coop_sub   = "sub"   // 부방장
	Baro_coop_edit  = "edit"  // 편집 유저
	Baro_coop_watch = "watch" // 관전 유저
)

// Baro_coop_close_minute : 폐쇄를 알리고 실제로 닫히기까지 두는 시간 (분)
const Baro_coop_close_minute = 10

// Baro_coop_max_limit : 정원의 위 끝
const Baro_coop_max_limit = 200

// Baro_coop_member_show : 참여자를 접지 않고 보여 주는 수
const Baro_coop_member_show = 4

// ------------------------------------------------------------
//  방 안에서의 권한 — 어느 자리까지 할 수 있게 둘 것인가
//
//  방마다 따로 정한다. 방장은 언제나 할 수 있으므로 고를 수 있는 것은 아래 셋이다.
// ------------------------------------------------------------

// 권한을 열어 줄 범위
const (
	Baro_coop_only_owner = "owner"  // 방장만
	Baro_coop_to_sub     = "sub"    // 부방장까지
	Baro_coop_to_member  = "member" // 참여자 모두
)

// Baro_coop_act_list : 방마다 정할 수 있는 행위 {열쇠, 이름, 기본 범위, 설명}
func Baro_coop_act_list() [][]string {
	return [][]string{
		{
			"comment", Baro_t("댓글 달기"), Baro_coop_to_member,
			Baro_t("방에 들어온 사람이 댓글을 달 수 있는 범위입니다."),
		},
		{
			"notice", Baro_t("공지 올리기"), Baro_coop_to_sub,
			Baro_t("방 공지를 올리고 고칠 수 있는 범위입니다."),
		},
		{
			"block", Baro_t("유저 편집 제한"), Baro_coop_to_sub,
			Baro_t("규정을 어긴 것으로 보이는 사람의 편집을 막을 수 있는 범위입니다. 함부로 쓰면 제재를 받습니다."),
		},
		{
			"invite", Baro_t("유저 초대"), Baro_coop_to_member,
			Baro_t("다른 사람을 이 방에 부를 수 있는 범위입니다."),
		},
		{
			"doc_add", Baro_t("편집 문서 추가"), Baro_coop_to_member,
			Baro_t("작업 공간에 문서를 열 수 있는 범위입니다. 없는 문서 이름을 적으면 새 문서를 만듭니다."),
		},
		{
			"doc_close", Baro_t("편집 문서 닫기"), Baro_coop_to_sub,
			Baro_t("작업 공간에서 문서를 닫을 수 있는 범위입니다. 문서 자체가 지워지지는 않습니다."),
		},
		{
			"setting", Baro_t("방 설정 고치기"), Baro_coop_to_sub,
			Baro_t("방 이름 · 설명 · 정원 · 금칙어를 고칠 수 있는 범위입니다."),
		},
		{
			"talk_hide", Baro_t("댓글 지우기"), Baro_coop_to_sub,
			Baro_t("남이 단 댓글을 감출 수 있는 범위입니다."),
		},
	}
}

// Baro_coop_scope_list : 고를 수 있는 범위 {열쇠, 이름}
func Baro_coop_scope_list() [][]string {
	return [][]string{
		{Baro_coop_only_owner, Baro_t("방장만")},
		{Baro_coop_to_sub, Baro_t("부방장까지")},
		{Baro_coop_to_member, Baro_t("참여자 모두")},
	}
}

// Baro_coop_scope_name : 범위 이름
func Baro_coop_scope_name(scope string) string {
	for _, one := range Baro_coop_scope_list() {
		if one[0] == scope {
			return one[1]
		}
	}

	return scope
}

// Baro_coop_act_default : 그 행위의 기본 범위
func Baro_coop_act_default(act string) string {
	for _, one := range Baro_coop_act_list() {
		if one[0] == act {
			return one[2]
		}
	}

	return Baro_coop_only_owner
}

// Baro_coop_act_name : 행위 이름
func Baro_coop_act_name(act string) string {
	for _, one := range Baro_coop_act_list() {
		if one[0] == act {
			return one[1]
		}
	}

	return act
}

// Baro_coop_role_rank : 자리의 높낮이 (클수록 높다)
func Baro_coop_role_rank(role string) int {
	switch role {
	case Baro_coop_owner:
		return 3
	case Baro_coop_sub:
		return 2
	case Baro_coop_edit, Baro_coop_watch:
		return 1
	}

	return 0
}

// Baro_coop_scope_rank : 범위가 요구하는 높낮이
func Baro_coop_scope_rank(scope string) int {
	switch scope {
	case Baro_coop_to_member:
		return 1
	case Baro_coop_to_sub:
		return 2
	}

	return 3
}

// Baro_coop_open_kind_list : 공개 갈래 {열쇠, 이름, 설명}
func Baro_coop_open_kind_list() [][]string {
	return [][]string{
		{Baro_coop_public, Baro_t("공개"), Baro_t("「열린 편집방」 목록에 올라 누구나 들어올 수 있습니다.")},
		{Baro_coop_code, Baro_t("일부 공개"), Baro_t("목록에 오르지 않습니다. 링크를 받거나 12자리 열쇠를 아는 사람만 들어옵니다.")},
		{Baro_coop_private, Baro_t("비공개"), Baro_t("방장과 이미 들어와 있는 사람만 볼 수 있습니다.")},
	}
}

// Baro_coop_open_kind_name : 공개 갈래 이름
func Baro_coop_open_kind_name(kind string) string {
	for _, row := range Baro_coop_open_kind_list() {
		if row[0] == kind {
			return row[1]
		}
	}

	return kind
}

// Baro_coop_role_name : 자리 이름
func Baro_coop_role_name(role string) string {
	switch role {
	case Baro_coop_owner:
		return Baro_t("방장")
	case Baro_coop_sub:
		return Baro_t("부방장")
	case Baro_coop_edit:
		return Baro_t("편집")
	case Baro_coop_watch:
		return Baro_t("관전")
	}

	return role
}

// ------------------------------------------------------------
//  이름표 만들기
// ------------------------------------------------------------

// baro_coop_first : 소리마디 첫소리
var baro_coop_first = []string{
	"g", "n", "d", "r", "m", "b", "s", "j", "ch", "k", "t", "p", "h", "",
}

// baro_coop_vowel : 소리마디 가운뎃소리
var baro_coop_vowel = []string{
	"a", "ya", "eo", "yeo", "o", "yo", "u", "yu", "eu", "i", "ae", "e", "oe", "wi",
}

// baro_coop_last : 소리마디 끝소리 (없을 수도 있다)
var baro_coop_last = []string{
	"", "", "k", "n", "l", "m", "p", "ng", "t",
}

// baro_coop_pick : 목록에서 하나 고르기
func baro_coop_pick(list []string) string {
	if len(list) == 0 {
		return ""
	}

	num, err := rand.Int(rand.Reader, big.NewInt(int64(len(list))))
	if err != nil {
		return list[0]
	}

	return list[num.Int64()]
}

// Baro_coop_link_new : 주소에 쓸 이름표 하나를 만든다
//
//	`jeup` `jeung` `hun` 처럼 소리 나는 대로 읽히는 한글 소리마디를 로마자로 이어 붙인다.
//	`a1b2c3` 같은 것보다 입으로 불러 주기 쉽다. (예 : jeupjeunghun)
func Baro_coop_link_new() string {
	out := ""

	for i := 0; i < 3; i++ {
		syllable := baro_coop_pick(baro_coop_first) + baro_coop_pick(baro_coop_vowel) + baro_coop_pick(baro_coop_last)

		// 첫소리도 끝소리도 없으면 한 글자짜리가 되어 읽기 어색하다.
		if len(syllable) < 2 {
			syllable += "n"
		}

		out += syllable
	}

	return out
}

// Baro_coop_code_new : 12자리 숫자 열쇠
func Baro_coop_code_new() string {
	out := ""

	for i := 0; i < 12; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return ""
		}

		out += string(rune('0' + num.Int64()))
	}

	return out
}

// Baro_coop_code_norm : 적어 낸 열쇠에서 숫자만 남긴다
func Baro_coop_code_norm(data string) string {
	out := []rune{}

	for _, one := range data {
		if one >= '0' && one <= '9' {
			out = append(out, one)
		}
	}

	return string(out)
}

// ------------------------------------------------------------
//  금칙어
// ------------------------------------------------------------

// Baro_coop_ban_word : 금칙어 하나와 바꿔 넣을 글자
type Baro_coop_ban_word struct {
	Word string
	To   string
}

// Baro_coop_ban_parse : 저장된 금칙어를 읽는다
//
//	한 줄에 하나씩, `금칙어` 와 `바꿀 글자` 를 세로줄로 가른다.
//	바꿀 글자를 비우면 그 금칙어는 빈 글자로 지워진다.
func Baro_coop_ban_parse(data string) []Baro_coop_ban_word {
	out := []Baro_coop_ban_word{}

	for _, line := range Baro_split_lines(data) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		word, to, _ := strings.Cut(line, "|")

		word = strings.TrimSpace(word)
		if word == "" {
			continue
		}

		out = append(out, Baro_coop_ban_word{Word: word, To: strings.TrimSpace(to)})
	}

	return out
}

// Baro_coop_ban_text : 금칙어 목록을 다시 글로
func Baro_coop_ban_text(list []Baro_coop_ban_word) string {
	lines := []string{}

	for _, one := range list {
		lines = append(lines, one.Word+"|"+one.To)
	}

	return strings.Join(lines, "\n")
}

// Baro_coop_ban_apply : 금칙어를 바꿔 넣는다. (바뀐 글, 걸린 금칙어)
//
//	적어 둔 차례대로 하나씩 훑는다. 앞에서 바꿔 넣은 글자가 뒤 금칙어에 다시 걸릴 수 있는데,
//	그것까지 그대로 따른다. 방장이 적은 차례가 곧 규칙이다.
func Baro_coop_ban_apply(data string, list []Baro_coop_ban_word) (string, []string) {
	hit := []string{}

	for _, one := range list {
		if one.Word == "" || !strings.Contains(data, one.Word) {
			continue
		}

		hit = append(hit, one.Word)
		data = strings.ReplaceAll(data, one.Word, one.To)
	}

	return data, hit
}

// Baro_coop_ban_notice : 금칙어에 걸렸을 때 보여 줄 말
func Baro_coop_ban_notice(hit []string, after string) string {
	if len(hit) == 0 {
		return ""
	}

	quoted := []string{}
	for _, one := range hit {
		quoted = append(quoted, "'"+one+"'")
	}

	return Baro_t("경고: 금칙어 ") + strings.Join(quoted, ", ") +
		Baro_t("(이)가 포함되어 있으며, 입력시 다음으로 조정됩니다:") + "\n> " + after
}
