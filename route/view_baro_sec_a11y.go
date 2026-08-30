package route

import (
	"net/url"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  17-6 접근성 — /baro/security/a11y
//
//   - 문서 본문에서 대체 텍스트 없는 이미지 찾기 (실제 점검 도구)
//   - 제목 단계를 건너뛴 문서 찾기 (화면 읽기 이동에 방해)
//   - 바로위키 화면이 지키는 접근성 규칙 체크리스트
// ============================================================

// baro_sec_a11y_prefix : 이미지·파일 문법 접두사
func baro_sec_a11y_prefix() []string {
	return []string{"파일:", "file:", tool.Baro_t("외부:"), "out:"}
}

// baro_sec_a11y_image : 문서 본문에서 찾은 이미지 문법 하나
type baro_sec_a11y_image struct {
	Raw      string
	Target   string
	Options  string
	HasAlt   bool
	WeakName bool
}

// baro_sec_a11y_weak_name : 파일 이름만으로는 무엇이 찍혔는지 알 수 없는지
//
//	이 위키의 문서 처리기는 이미지의 alt 를 언제나 파일 이름으로 채운다.
//	(route/tool/markup/namumark_compat.go 의 process_file)
//	그래서 지금은 파일 이름이 곧 화면 읽기 프로그램이 읽는 문장이다.
func baro_sec_a11y_weak_name(target string) bool {
	name := target
	if cut := strings.Index(name, ":"); cut >= 0 {
		name = name[cut+1:]
	}

	if dot := strings.LastIndex(name, "."); dot > 0 {
		name = name[:dot]
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return true
	}

	// 숫자와 기호만 남으면 설명이 아니다
	letters := 0
	for _, ch := range name {
		if (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' || ch == ' ' || ch == '.' {
			continue
		}

		letters++
	}

	if letters == 0 {
		return true
	}

	// 여섯 글자 이하의 짧은 이름은 설명이 되기 어렵다
	if tool.Get_len(name) <= 6 {
		return true
	}

	lower := strings.ToLower(name)
	for _, prefix := range []string{"img", "image", "photo", "pic", "dsc", "screenshot", "untitled", tool.Baro_t("사진"), tool.Baro_t("그림"), tool.Baro_t("이미지"), tool.Baro_t("무제")} {
		if strings.HasPrefix(lower, prefix) && tool.Get_len(name) <= tool.Get_len(prefix)+4 {
			return true
		}
	}

	return false
}

// baro_sec_a11y_has_alt : 옵션 부분에 대체 텍스트가 있는지
//
//	openNAMU 마크업은 [[파일:이름.png|alt=설명]] 또는 [[파일:이름.png|설명]] 을 대체 텍스트로 쓴다.
//	옵션이 없으면 파일 이름이 그대로 alt 가 되므로 사람이 읽을 설명이 아니다.
func baro_sec_a11y_has_alt(options string) bool {
	if strings.TrimSpace(options) == "" {
		return false
	}

	for _, item := range strings.Split(options, "&") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		cut := strings.SplitN(item, "=", 2)
		if len(cut) != 2 {
			// 값만 적은 항목은 대체 텍스트로 쓰인다
			return true
		}

		name := strings.ToLower(strings.TrimSpace(cut[0]))
		if (name == "alt" || name == "title") && strings.TrimSpace(cut[1]) != "" {
			return true
		}
	}

	return false
}

// baro_sec_a11y_find : 문서 본문에서 이미지 문법을 모두 찾는다
func baro_sec_a11y_find(data string) []baro_sec_a11y_image {
	out := []baro_sec_a11y_image{}

	rest := data
	for {
		open := strings.Index(rest, "[[")
		if open < 0 {
			break
		}

		rest = rest[open+2:]

		stop := strings.Index(rest, "]]")
		if stop < 0 {
			break
		}

		inner := rest[:stop]
		rest = rest[stop+2:]

		lower := strings.ToLower(strings.TrimSpace(inner))
		match := false
		for _, prefix := range baro_sec_a11y_prefix() {
			if strings.HasPrefix(lower, prefix) {
				match = true

				break
			}
		}

		if !match {
			continue
		}

		target := inner
		options := ""
		if bar := strings.Index(inner, "|"); bar >= 0 {
			target = inner[:bar]
			options = inner[bar+1:]
		}

		out = append(out, baro_sec_a11y_image{
			Raw:      "[[" + tool.Baro_trunc(inner, 80) + "]]",
			Target:   strings.TrimSpace(target),
			Options:  options,
			HasAlt:   baro_sec_a11y_has_alt(options),
			WeakName: baro_sec_a11y_weak_name(strings.TrimSpace(target)),
		})
	}

	return out
}

// baro_sec_a11y_heading_jump : 제목 단계를 건너뛴 곳을 찾는다 (건너뛴 설명 목록)
func baro_sec_a11y_heading_jump(data string) []string {
	out := []string{}
	last := 0

	for _, line := range tool.Baro_split_lines(data) {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "=") || !strings.HasSuffix(trimmed, "=") {
			continue
		}

		level := 0
		for _, ch := range trimmed {
			if ch != '=' {
				break
			}

			level++
		}

		if level < 1 || level > 6 || tool.Get_len(trimmed) <= level*2 {
			continue
		}

		if last > 0 && level > last+1 {
			out = append(out, tool.Baro_trunc(trimmed, 60)+tool.Baro_t(" (단계 ")+tool.Baro_itoa(last)+" → "+tool.Baro_itoa(level)+")")
		}

		last = level
	}

	return out
}

// View_baro_sec_a11y : 접근성 점검 화면
func View_baro_sec_a11y(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/security/a11y"); need {
		return wall
	}

	if values != nil {
		return tool.Get_redirect("/baro/security/a11y")
	}

	baro_sec_watch(db, config, baro_sec_real_ip(config, query))

	mode := baro_sec_q(query, "mode")
	if mode != "heading" {
		mode = "alt"
	}

	page := baro_sec_page_num(query)

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/security", tool.Baro_t("보안")}, {"", tool.Baro_t("접근성")}})
	body += tool.Baro_hero(
		tool.Baro_t("접근성"),
		tool.Baro_t("눈으로 보지 않아도, 손가락 대신 키보드로도 위키를 쓸 수 있어야 합니다."),
		"",
	)
	body += baro_sec_tabs("/baro/security/a11y")

	body += tool.Baro_tabs([][]string{
		{"/baro/security/a11y", tool.Baro_t("대체 텍스트 점검")},
		{"/baro/security/a11y?mode=heading", tool.Baro_t("제목 단계 점검")},
	}, func() string {
		if mode == "heading" {
			return "/baro/security/a11y?mode=heading"
		}

		return "/baro/security/a11y"
	}())

	if !baro_sec_abuse_rate_ok(db, config.IP) {
		body += tool.Baro_alert("warn", tool.Baro_t("요청 속도 제한에 걸려 점검을 건너뜁니다. 잠시 뒤에 다시 시도하세요."))

		return baro_sec_page(db, config, tool.Baro_t("접근성"), body)
	}

	docs := baro_sec_doc_page(db, (page-1)*baro_sec_scan_size, baro_sec_scan_size)

	if mode == "alt" {
		table_rows := [][]string{}
		image_count, missing_count, weak_count := 0, 0, 0

		for _, doc := range docs {
			images := baro_sec_a11y_find(doc[1])
			if len(images) == 0 {
				continue
			}

			missing := []string{}
			weak := []string{}

			for _, image := range images {
				image_count++

				if !image.HasAlt {
					missing_count++
					missing = append(missing, image.Raw)
				}

				if image.WeakName {
					weak_count++
					weak = append(weak, image.Target)
				}
			}

			if len(missing) == 0 && len(weak) == 0 {
				continue
			}

			weak_html := tool.Baro_badge(tool.Baro_t("괜찮음"), "ok")
			if len(weak) > 0 {
				weak_html = tool.Baro_badge(tool.Baro_itoa(len(weak))+tool.Baro_t("개 뜻 없음"), "danger") +
					" " + tool.HTML_escape(tool.Baro_trunc(strings.Join(weak, " / "), 60))
			}

			table_rows = append(table_rows, []string{
				tool.Baro_doc_link(doc[0]),
				tool.Baro_itoa(len(images)),
				tool.Baro_itoa(len(missing)),
				tool.HTML_escape(tool.Baro_trunc(strings.Join(missing, " / "), 120)),
				weak_html,
				tool.Baro_button("/edit/"+tool.Url_parser(doc[0]), "고치기", ""),
			})
		}

		body += tool.Baro_stat_row([][]string{
			{tool.Baro_t("검사한 문서"), tool.Baro_itoa(len(docs)), tool.Baro_t("이 쪽에서")},
			{tool.Baro_t("찾은 이미지"), tool.Baro_itoa(image_count), tool.Baro_t("파일·외부 이미지 문법")},
			{tool.Baro_t("설명 없음"), tool.Baro_itoa(missing_count), tool.Baro_t("alt= 나 설명 글이 없음")},
			{tool.Baro_t("파일 이름이 뜻 없음"), tool.Baro_itoa(weak_count), tool.Baro_t("지금 실제로 읽히는 문장")},
			{tool.Baro_t("전체 문서"), tool.Baro_comma(baro_sec_doc_count(db)), tool.Baro_t("data 표")},
		})

		body += tool.Baro_alert("warn",
			tool.Baro_t("<b>이 위키가 지금 실제로 하는 일부터 알려드립니다.</b> 문서 처리기")+
				tool.Baro_t("(<code>route/tool/markup/namumark_compat.go</code> 의 <code>process_file</code>)는 ")+
				tool.Baro_t("이미지의 <code>alt</code> 를 <b>언제나 파일 이름으로</b> 채웁니다. ")+
				tool.Baro_t("<code>alt=</code> 로 적은 설명은 아직 화면에 반영되지 않습니다. 그래서 두 가지를 함께 봅니다."),
		)
		body += tool.Baro_list([]string{
			tool.Baro_t("<b>지금 당장 효과가 있는 것</b> : 파일 이름을 설명처럼 짓습니다. ") +
				tool.Baro_t("<code>img001.jpg</code> 대신 <code>1950년 부산항 전경.jpg</code> 처럼 씁니다. ") +
				tool.Baro_t("표의 <b>파일 이름 판정</b> 칸이 «뜻 없음» 인 이미지가 그 대상입니다."),
			tool.Baro_t("<b>본문에도 남겨 두세요</b> : <code>[[파일:부산항.jpg|alt=1950년 부산항 전경]]</code> 처럼 설명을 적습니다. ") +
				tool.Baro_t("문서 처리기가 고쳐지면 그대로 쓰이고, 그때까지도 다음 편집자에게 뜻이 전달됩니다."),
			tool.Baro_t("장식용 이미지라면 본문에서 빼는 편이 낫습니다."),
		})

		if len(table_rows) == 0 {
			body += tool.Baro_empty(tool.Baro_t("이 쪽에서 검사한 문서에는 손볼 이미지가 없습니다."))
		} else {
			body += tool.Baro_table(
				[]string{"문서", tool.Baro_t("이미지 수"), tool.Baro_t("설명 없는 수"), tool.Baro_t("설명 없는 문법"), tool.Baro_t("파일 이름 판정"), "이동"}, table_rows, "",
			)
		}

		body += tool.Baro_paging(page, len(docs), baro_sec_scan_size, "/baro/security/a11y?page={}")
	} else {
		table_rows := [][]string{}
		jump_count := 0

		for _, doc := range docs {
			jumps := baro_sec_a11y_heading_jump(doc[1])
			if len(jumps) == 0 {
				continue
			}

			jump_count += len(jumps)

			table_rows = append(table_rows, []string{
				tool.Baro_doc_link(doc[0]),
				tool.Baro_itoa(len(jumps)),
				tool.HTML_escape(tool.Baro_trunc(strings.Join(jumps, " / "), 120)),
				tool.Baro_button("/edit/"+tool.Url_parser(doc[0]), "고치기", ""),
			})
		}

		body += tool.Baro_stat_row([][]string{
			{tool.Baro_t("검사한 문서"), tool.Baro_itoa(len(docs)), tool.Baro_t("이 쪽에서")},
			{tool.Baro_t("건너뛴 제목"), tool.Baro_itoa(jump_count), tool.Baro_t("단계를 뛰어넘은 곳")},
			{tool.Baro_t("전체 문서"), tool.Baro_comma(baro_sec_doc_count(db)), tool.Baro_t("data 표")},
		})

		body += tool.Baro_alert("info",
			tool.Baro_t("화면 읽기 프로그램 사용자는 제목만 따라 문서를 훑습니다. ")+
				tool.Baro_t("1단계 다음에 갑자기 3단계가 오면 사이가 비어 길을 잃습니다. 한 단계씩 내려가게 고쳐 주세요."),
		)

		if len(table_rows) == 0 {
			body += tool.Baro_empty(tool.Baro_t("이 쪽에서 검사한 문서에는 건너뛴 제목이 없습니다."))
		} else {
			body += tool.Baro_table([]string{"문서", tool.Baro_t("건너뛴 곳"), tool.Baro_t("내용"), "이동"}, table_rows, "")
		}

		body += tool.Baro_paging(page, len(docs), baro_sec_scan_size, "/baro/security/a11y?mode=heading&page={}")
	}

	// --- 체크리스트 ---
	body += tool.Baro_card(tool.Baro_t("바로위키 화면이 지키는 규칙"),
		tool.Baro_table(
			[]string{tool.Baro_t("항목"), "상태", tool.Baro_t("내용")},
			[][]string{
				{
					tool.Baro_t("키보드 사용"),
					tool.Baro_badge(tool.Baro_t("지킴"), "ok"),
					tool.Baro_t("모든 조작은 링크(<code>a</code>)·단추(<code>button</code>)·폼 요소로만 만듭니다. 눌러야 동작하는 <code>div</code> 를 쓰지 않으므로 탭 키로 전부 옮겨 다닐 수 있습니다."),
				},
				{
					tool.Baro_t("화면 읽기 지원 (표)"),
					tool.Baro_badge(tool.Baro_t("지킴"), "ok"),
					tool.Baro_t("표는 <code>th</code> 머리칸을 씁니다. 머리칸이 있어야 각 칸이 무엇인지 함께 읽힙니다."),
				},
				{
					tool.Baro_t("화면 읽기 지원 (입력칸 이름표)"),
					tool.Baro_badge(tool.Baro_t("고칠 점 있음"), "warn"),
					tool.Baro_t("체크 상자는 이름표가 입력칸을 감싸고 있어 잘 읽히지만, 한 줄 입력칸은 이름표와 <code>for</code> 로 묶여 있지 않습니다. 기반 UI 부품을 고쳐야 하는 부분이라 여기서는 문제로만 알립니다."),
				},
				{
					tool.Baro_t("충분한 글자 크기"),
					tool.Baro_badge(tool.Baro_t("지킴"), "ok"),
					tool.Baro_t("글자 크기를 <code>rem</code> 로 지정해 브라우저 기본 크기를 키우면 화면 전체가 함께 커집니다. 고정 <code>px</code> 글꼴을 쓰지 않습니다."),
				},
				{
					tool.Baro_t("명확한 대비"),
					tool.Baro_badge(tool.Baro_t("지킴"), "ok"),
					tool.Baro_t("본문과 배지 글자색은 옅은 회색을 피하고 진한 색을 씁니다. 색만으로 뜻을 전하지 않고 배지에 글자를 함께 넣습니다."),
				},
				{
					tool.Baro_t("이미지 대체 텍스트"),
					tool.Baro_badge(tool.Baro_t("고칠 점 있음"), "warn"),
					tool.Baro_t("문서 처리기가 이미지의 <code>alt</code> 를 항상 파일 이름으로 채우고 <code>alt=</code> 옵션을 버립니다. 기반 코드를 고쳐야 하는 부분이라 여기서는 문제로만 알리고, 그때까지는 파일 이름을 설명처럼 짓도록 위 점검 도구가 안내합니다."),
				},
				{
					tool.Baro_t("모바일 편집"),
					tool.Baro_badge(tool.Baro_t("지킴"), "ok"),
					tool.Baro_t("좁은 화면(640px 이하)용 규칙이 있어 글자·여백이 줄고, 표와 코드 상자는 가로로만 스크롤됩니다. 화면 전체가 옆으로 밀리지 않습니다."),
				},
				{
					tool.Baro_t("움직임·자동 재생"),
					tool.Baro_badge(tool.Baro_t("지킴"), "ok"),
					tool.Baro_t("스스로 움직이거나 소리를 내는 요소를 쓰지 않습니다."),
				},
			},
			"",
		),
	)

	body += tool.Baro_card(tool.Baro_t("문서를 쓸 때 지켜 주세요"),
		tool.Baro_list([]string{
			tool.Baro_t("이미지 <b>파일 이름</b>을 무엇이 찍혔는지 알 수 있게 지어 주세요. 지금 이 위키에서는 파일 이름이 그대로 읽힙니다. '사진1' 같은 이름은 도움이 되지 않습니다."),
			tool.Baro_t("제목은 한 단계씩 내려갑니다. 큰 제목 다음에 바로 작은 제목을 쓰지 마세요."),
			tool.Baro_t("'여기를 클릭' 대신 '독립운동 문서 보기' 처럼 링크 글자만 읽어도 뜻이 통하게 씁니다."),
			tool.Baro_t("색으로만 구분하지 마세요. 붉은 글씨 대신 '주의 :' 같은 말을 함께 씁니다."),
			tool.Baro_t("표는 자료를 담는 데 쓰고, 화면을 배치하는 데 쓰지 마세요."),
		}),
	)

	return baro_sec_page(db, config, tool.Baro_t("접근성"), body)
}
