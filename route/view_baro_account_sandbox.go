package route

import (
	"net/url"
	"strconv"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  2-3 첫 기여 도우미 (연습장 · 문법 자동 안내 · 출처 입력 도우미)
// ============================================================

// View_baro_account_sandbox : 연습 문서 (/baro/draft/sandbox)
//
//	실제 문서에 전혀 영향을 주지 않는 편집 연습장이다.
func View_baro_account_sandbox(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/draft/sandbox"); need {
		return wall
	}

	user := config.IP
	from := strings.TrimSpace(page)

	sandbox := baro_account_sandbox_get(db, user)
	body_data := ""
	if sandbox != nil {
		body_data = sandbox["data"]
	}
	if strings.TrimSpace(body_data) == "" {
		body_data = baro_account_sandbox_example()
	}

	preview := ""
	saved := false

	// --- 쓰기 처리 ---
	if values != nil {
		if tool.Baro_read_only(db) {
			return tool.Baro_page(db, config, tool.Baro_t("연습장"),
				tool.Baro_alert("warn", tool.Baro_t("지금은 읽기 전용 상태입니다. ")+tool.HTML_escape(tool.Baro_read_only_reason(db))),
				baro_account_menu())
		}

		body_data = values.Get("data")

		if values.Get("action") == "preview" {
			preview = baro_account_render(db, body_data)
		} else if tool.Baro_rate_ok(db, "baro_account_sandbox", user, 120, 3600) {
			if sandbox == nil {
				tool.Exec_DB(
					db,
					"insert into baro_workspace (id, project, title, data, user, date, status, note) values (?, 'sandbox', '연습장', ?, ?, ?, 'closed', '편집 연습용')",
					tool.Baro_next_id(db, "baro_workspace"), body_data, user, tool.Baro_now(),
				)
			} else {
				tool.Exec_DB(
					db,
					"update baro_workspace set data = ?, date = ? where id = ? and user = ?",
					body_data, tool.Baro_now(), sandbox["id"], user,
				)
			}

			saved = true
			sandbox = baro_account_sandbox_get(db, user)

			// 온보딩 4단계(연습장 편집)를 자동으로 완료 처리한다
			if values.Get("from") == "onboarding" {
				baro_account_onboard_advance(db, user, 4, "done")
				baro_account_onboard_finish(db, config)
			}
		} else {
			preview = tool.Baro_alert("warn", tool.Baro_t("저장 요청이 너무 잦습니다. 잠시 뒤 다시 시도해 주세요."))
		}

		if values.Get("from") == "onboarding" {
			from = "onboarding"
		}
	}

	check := baro_account_scan(db, body_data)

	form := tool.Baro_hidden("from", from)
	form += tool.Baro_field_row_html(tool.Baro_t("연습 본문"),
		`<textarea id="bw_acc_sandbox_area" class="bw_in bw_ta" rows="20" name="data">`+tool.HTML_escape(body_data)+`</textarea>`,
		tool.Baro_t("여기에 쓴 내용은 어떤 실제 문서에도 반영되지 않습니다."))
	form += baro_account_editor_js("bw_acc_sandbox_area", "sandbox")
	form += tool.Baro_submit(tool.Baro_t("연습장에 저장"), "primary")
	form += tool.Baro_t(` <button type="submit" class="bw_btn" name="action" value="preview">미리보기</button>`)

	body := tool.Baro_hero(tool.Baro_t("연습장"), tool.Baro_t("실제 문서에 영향을 주지 않는 편집 연습 공간입니다."),
		tool.Baro_button("/baro/draft", tool.Baro_t("내 초안"), "")+" "+
			tool.Baro_button("/baro/draft/cite", tool.Baro_t("출처 입력 도우미"), "")+" "+
			tool.Baro_button("/baro/draft/new", tool.Baro_t("진짜 초안 만들기"), ""))

	body += baro_account_tabs("/baro/draft/sandbox")

	if from == "onboarding" {
		body += tool.Baro_alert("info", tool.Baro_t("첫걸음 4단계입니다. 아무 내용이나 한 번 저장해 보면 완료 처리됩니다."))
		body += tool.Baro_button("/baro/onboarding?step=4", tool.Baro_t("첫걸음으로 돌아가기"), "primary")
	}

	if saved {
		body += tool.Baro_alert("ok", tool.Baro_t("연습장에 저장했습니다. 이 내용은 공개되지 않습니다."))
	}

	body += tool.Baro_card(tool.Baro_t("연습 편집"), tool.Baro_form("/baro/draft/sandbox", form))

	if preview != "" {
		body += tool.Baro_card("미리보기", preview)
	}

	body += tool.Baro_card(tool.Baro_t("문법 자동 안내"), baro_account_scan_html(check))
	body += tool.Baro_card(tool.Baro_t("제출 전 체크리스트"),
		baro_account_checklist_html(tool.Baro_t("연습장"), check)+
			tool.Baro_note(tool.Baro_t("실제 문서로 올릴 때 이 항목들을 다시 확인합니다.")))

	body += tool.Baro_card(tool.Baro_t("문법 안내"), baro_account_syntax_guide())

	if sandbox != nil {
		body += tool.Baro_card(tool.Baro_t("연습장 기록"),
			tool.Baro_dl([][]string{
				{tool.Baro_t("마지막 저장"), tool.Baro_time_cell(sandbox["date"])},
				{"길이", tool.Baro_comma(tool.Get_len(sandbox["data"])) + tool.Baro_t("자")},
				{tool.Baro_t("보관 위치"), tool.Baro_t("내 작업실(연습장) — 나만 볼 수 있습니다.")},
			}))
	} else {
		body += tool.Baro_empty(tool.Baro_t("아직 연습장에 저장한 내용이 없습니다."))
	}

	return tool.Baro_page(db, config, tool.Baro_t("연습장"), body, baro_account_menu())
}

// baro_account_sandbox_example : 연습장 기본 예시 본문
func baro_account_sandbox_example() string {
	return "== 연습 ==\n" +
		tool.Baro_t("여기는 연습장입니다. 마음껏 고쳐 보세요.\n") +
		"\n" +
		tool.Baro_t("'''굵게''' 와 ''기울임'' 을 써 봅니다.\n") +
		tool.Baro_t("링크는 [[대문]] 처럼 씁니다.\n") +
		tool.Baro_t("출처는 이렇게 답니다.[* 보기 출처, 《연습 자료집》, 2024, 10쪽.]\n") +
		"\n" +
		tool.Baro_t(" * 목록도 만들어 보세요\n") +
		tool.Baro_t(" * 앞에 공백 한 칸이 필요합니다\n") +
		"\n" +
		tool.Baro_t("[[분류:연습]]")
}

// View_baro_account_cite : 출처 입력 도우미 (/baro/draft/cite)
func View_baro_account_cite(config tool.Config, page string, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/draft/cite"); need {
		return wall
	}

	kind := strings.TrimSpace(page)
	result := ""

	if values != nil {
		kind = strings.TrimSpace(values.Get("kind"))
		result = baro_account_cite_build(kind, values)
	}

	if kind == "" {
		kind = "book"
	}

	body := tool.Baro_hero(tool.Baro_t("출처 입력 도우미"), tool.Baro_t("항목을 채우면 각주 문법을 만들어 드립니다."),
		tool.Baro_button("/baro/draft/sandbox", tool.Baro_t("연습장에서 써보기"), "")+" "+
			tool.Baro_button("/baro/draft", tool.Baro_t("내 초안"), ""))

	body += baro_account_tabs("/baro/draft")

	tabs := [][]string{}
	for _, row := range baro_account_cite_kind_list() {
		tabs = append(tabs, []string{"/baro/draft/cite/" + row[0], row[1]})
	}
	body += tool.Baro_tabs(tabs, "/baro/draft/cite/"+kind)

	form := tool.Baro_hidden("kind", kind)

	switch kind {
	case "paper":
		form += tool.Baro_input("author", tool.Baro_t("지은이"), baro_account_values_get(values, "author"), tool.Baro_t("예: 김철수"), "")
		form += tool.Baro_input("title", tool.Baro_t("논문 제목"), baro_account_values_get(values, "title"), "", "")
		form += tool.Baro_input("publisher", tool.Baro_t("학술지 이름"), baro_account_values_get(values, "publisher"), "", "")
		form += tool.Baro_input("volume", tool.Baro_t("권·호"), baro_account_values_get(values, "volume"), tool.Baro_t("예: 제12권 3호"), "")
		form += tool.Baro_input("pub_date", tool.Baro_t("발행 연도"), baro_account_values_get(values, "pub_date"), tool.Baro_t("예: 2021"), "")
		form += tool.Baro_input("page", tool.Baro_t("쪽수"), baro_account_values_get(values, "page"), tool.Baro_t("예: 45-60"), "")
		form += tool.Baro_input("url", tool.Baro_t("주소 (있으면)"), baro_account_values_get(values, "url"), "", "")
	case "news":
		form += tool.Baro_input("author", tool.Baro_t("기자 이름"), baro_account_values_get(values, "author"), "", "")
		form += tool.Baro_input("title", tool.Baro_t("기사 제목"), baro_account_values_get(values, "title"), "", "")
		form += tool.Baro_input("publisher", tool.Baro_t("언론사"), baro_account_values_get(values, "publisher"), tool.Baro_t("예: 보기일보"), "")
		form += tool.Baro_input_type("pub_date", tool.Baro_t("보도일"), baro_account_values_get(values, "pub_date"), "date", "")
		form += tool.Baro_input("url", tool.Baro_t("기사 주소"), baro_account_values_get(values, "url"), "https://", "")
	case "web":
		form += tool.Baro_input("author", tool.Baro_t("글쓴이 (있으면)"), baro_account_values_get(values, "author"), "", "")
		form += tool.Baro_input("title", tool.Baro_t("문서 제목"), baro_account_values_get(values, "title"), "", "")
		form += tool.Baro_input("publisher", tool.Baro_t("사이트 이름"), baro_account_values_get(values, "publisher"), "", "")
		form += tool.Baro_input("url", tool.Baro_t("주소"), baro_account_values_get(values, "url"), "https://", "")
		form += tool.Baro_input_type("access_date", tool.Baro_t("확인한 날짜"), baro_account_values_get(values, "access_date"), "date", tool.Baro_t("웹 자료는 사라질 수 있어 확인 날짜를 적습니다."))
		form += tool.Baro_input("archive_url", tool.Baro_t("보존본 주소 (있으면)"), baro_account_values_get(values, "archive_url"), "", tool.Baro_t("웹 보존 서비스 주소를 함께 남기면 좋습니다."))
	default:
		form += tool.Baro_input("author", tool.Baro_t("지은이"), baro_account_values_get(values, "author"), tool.Baro_t("예: 홍길동"), "")
		form += tool.Baro_input("title", tool.Baro_t("책 제목"), baro_account_values_get(values, "title"), "", "")
		form += tool.Baro_input("publisher", tool.Baro_t("출판사"), baro_account_values_get(values, "publisher"), "", "")
		form += tool.Baro_input("pub_date", tool.Baro_t("발행 연도"), baro_account_values_get(values, "pub_date"), tool.Baro_t("예: 2020"), "")
		form += tool.Baro_input("page", tool.Baro_t("쪽수"), baro_account_values_get(values, "page"), tool.Baro_t("예: 123"), "")
	}

	form += tool.Baro_submit(tool.Baro_t("각주 문법 만들기"), "primary")

	body += tool.Baro_card(baro_account_cite_kind_name(kind)+" 출처", tool.Baro_form("/baro/draft/cite/"+kind, form))

	if values != nil {
		if result == "" {
			body += tool.Baro_alert("warn", tool.Baro_t("채운 항목이 없습니다. 최소한 제목이나 지은이는 적어 주세요."))
		} else {
			body += tool.Baro_card_ex(
				tool.Baro_t("이 문법을 본문에 붙여넣으세요"),
				tool.Baro_t("각주는 본문에서 근거를 밝히는 자리에 그대로 넣습니다."),
				tool.Baro_code(result)+baro_account_copy_js(result),
				"본문 아래쪽에 == 각주 == 문단을 두면 보기 좋게 정리됩니다.",
			)
		}
	}

	body += tool.Baro_card(tool.Baro_t("좋은 출처의 조건"),
		tool.Baro_list([]string{
			tool.Baro_t("누구나 찾아볼 수 있어야 합니다. 개인 메모나 비공개 자료는 출처가 되기 어렵습니다."),
			tool.Baro_t("어디를 봤는지 구체적이어야 합니다. 책은 쪽수까지, 웹은 확인한 날짜까지 적습니다."),
			tool.Baro_t("웹 자료는 사라지기 쉽습니다. 보존본 주소를 함께 남겨 주세요."),
			tool.Baro_t("1차 자료와 2차 자료를 섞어 쓰면 서술이 더 단단해집니다."),
		}))

	return tool.Baro_page(db, config, tool.Baro_t("출처 입력 도우미"), body, baro_account_menu())
}

// baro_account_values_get : 폼 값 읽기 (nil 이면 빈 문자열)
func baro_account_values_get(values url.Values, name string) string {
	if values == nil {
		return ""
	}

	return values.Get(name)
}

// baro_account_copy_js : 만든 문법을 복사하는 단추
func baro_account_copy_js(text string) string {
	return `<div class="bw_f"><button type="button" class="bw_btn" id="bw_acc_copy">복사하기</button> <span class="bw_note" id="bw_acc_copy_msg"></span></div>
<script>
(function(){
  var button = document.getElementById("bw_acc_copy");
  var message = document.getElementById("bw_acc_copy_msg");
  if (!button) { return; }
  var text = "` + tool.JS_escape(text) + `";
  button.addEventListener("click", function(){
    try {
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text);
      } else {
        var area = document.createElement("textarea");
        area.value = text;
        document.body.appendChild(area);
        area.select();
        document.execCommand("copy");
        document.body.removeChild(area);
      }
      if (message) { message.textContent = ` + strconv.Quote(tool.Baro_t("복사했습니다.")) + `; }
    } catch (e) {
      if (message) { message.textContent = ` + strconv.Quote(tool.Baro_t("복사에 실패했습니다. 위 상자에서 직접 선택해 주세요.")) + `; }
    }
  });
})();
</script>`
}
