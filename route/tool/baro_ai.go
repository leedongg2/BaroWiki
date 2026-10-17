package tool

import "strings"

// ============================================================
//  AI 참고 표시
//
//  AI 의 도움을 받아 쓴 편집은 그 사실을 스스로 밝히게 한다.
//  체크하면 편집 요약 뒤에 「(AI 참고)」 가 붙어 역사 · 기여 기록 · 최근 변경에 그대로 남는다.
//
//  밝히지 않은 것을 잡아내는 장치는 두지 않는다.
//  그것은 사람이 보고 판단할 일이고, 규정(이용자 관리 방침)이 맡는다.
//    · AI 를 일부 쓰고도 밝히지 않으면 3일 이상 차단
//    · AI 로 통째로 쓴 글은 밝혔든 아니든 7일 이상 차단
// ============================================================

// Baro_ai_field : 폼에 실리는 칸 이름
const Baro_ai_field = "baro_ai"

// Baro_ai_mark : 편집 요약 뒤에 붙는 표시
const Baro_ai_mark = "(AI 참고)"

// Baro_ai_on : 체크했는가
func Baro_ai_on(value string) bool {
	return strings.TrimSpace(value) != ""
}

// Baro_ai_has : 이 요약에 이미 표시가 붙어 있는가
func Baro_ai_has(send string) bool {
	return strings.Contains(send, Baro_ai_mark)
}

// Baro_ai_send : 편집 요약 뒤에 표시를 붙인다
//
//	체크하지 않았으면 그대로 두고, 이미 붙어 있으면 두 번 붙이지 않는다.
func Baro_ai_send(send string, on bool) string {
	if !on || Baro_ai_has(send) {
		return send
	}

	if strings.TrimSpace(send) == "" {
		return Baro_ai_mark
	}

	return send + " " + Baro_ai_mark
}

// Baro_ai_text : 체크 칸에 적는 말
func Baro_ai_text() string {
	return Baro_t("AI를 참고하였습니다.")
}

// Baro_ai_help : 체크 칸 아래 붙이는 안내
func Baro_ai_help() string {
	return Baro_t("AI 의 도움을 조금이라도 받았다면 표시해 주세요. ") +
		Baro_t("표시하면 편집 요약 뒤에 「(AI 참고)」 가 붙습니다. ") +
		Baro_t("일부라도 쓰고 표시하지 않으면 3일 이상, AI 로 통째로 쓴 글은 표시해도 7일 이상 차단될 수 있습니다.")
}

// Baro_ai_check_ui : 본체 편집 화면에 넣는 체크 칸
//
//	저작권 동의 칸과 같은 모양으로 맞춘다. (스킨이 __ON_...__ 를 제 클래스로 바꾼다)
func Baro_ai_check_ui() string {
	return `
            <label class="__ON_CHECKLABEL__"><input class="__ON_CHECKBOX__" type="checkbox" name="` +
		Baro_ai_field + `" value="yes"> ` + Baro_t_esc(Baro_ai_text()) + `</label>
            <div class="baro_ai_help">` + Baro_t_esc(Baro_ai_help()) + `</div>
            <hr class="main_hr">
        ` + Baro_ai_css()
}

// Baro_ai_css : 안내 글씨 모양
func Baro_ai_css() string {
	return `<style id="baro_ai_css">
.baro_ai_help { font-size: 12px; opacity: 0.75; margin: 4px 0 8px; }
</style>`
}
