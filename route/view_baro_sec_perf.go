package route

import (
	"database/sql"
	"net/url"
	"sort"
	"strings"

	"opennamu/route/tool"
)

// ============================================================
//  17-5 성능 — /baro/security/perf
//
//   - 표별 줄 수와 대략적인 크기, 인덱스 목록
//   - 느려질 수 있는 지점 점검 (긴 역사, 큰 문서)
//   - 이미지 지연 로딩 · 캐시 현황
//
//  각 항목에는 "무엇을 하면 되는지" 를 함께 적는다.
// ============================================================

// baro_sec_perf_main_tables : openNAMU 본체 표
func baro_sec_perf_main_tables() []string {
	return []string{
		"data", "history", "rc", "acl", "back", "data_set",
		"topic", "topic_set", "rd", "rb", "other", "html_filter",
		"vote", "alist", "re_admin", "ua_d", "user_set", "user_notice",
		"bbs_set", "bbs_data",
	}
}

// baro_sec_perf_table_list : 점검할 표 이름 목록 (본체 + 바로위키)
func baro_sec_perf_table_list() []string {
	out := append([]string{}, baro_sec_perf_main_tables()...)

	baro := []string{}
	for table_name := range tool.Baro_table_list() {
		baro = append(baro, table_name)
	}

	sort.Strings(baro)

	return append(out, baro...)
}

// baro_sec_perf_text_size : 본문 길이 합계 (해당 컬럼이 없으면 -1)
func baro_sec_perf_text_size(db *sql.DB, table_name string, column_name string) int {
	if !baro_sec_table_name_ok(table_name) || !baro_sec_table_name_ok(column_name) {
		return -1
	}

	size := 0
	if !tool.QueryRow_DB(db, "select coalesce(sum(length("+column_name+")), 0) from "+table_name, []any{&size}) {
		return -1
	}

	return size
}

// baro_sec_perf_index_rows : SQLite 인덱스 목록
func baro_sec_perf_index_rows(db *sql.DB) [][]string {
	out := [][]string{}

	if tool.Get_DB_type() != "sqlite" {
		return out
	}

	rows := tool.Query_DB(db, "select name, tbl_name, coalesce(sql, '') from sqlite_master where type = 'index' order by tbl_name, name")
	defer rows.Close()

	for rows.Next() {
		name, table_name, sql_text := "", "", ""
		if err := rows.Scan(&name, &table_name, &sql_text); err != nil {
			continue
		}

		out = append(out, []string{name, table_name, sql_text})
	}

	return out
}

// View_baro_sec_perf : 성능 점검 화면
func View_baro_sec_perf(config tool.Config, query url.Values, values url.Values) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	if wall, need := tool.Baro_login_wall(db, config, "/baro/security/perf"); need {
		return wall
	}

	if !baro_sec_is_system(db, config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("성능 점검은 system 권한만 볼 수 있습니다."))
	}

	if values != nil {
		return baro_sec_perf_post(db, config, values)
	}

	baro_sec_watch(db, config, baro_sec_real_ip(config, query))

	big_doc := tool.Baro_get_int(db, "perf_big_doc", 60000)
	long_history := tool.Baro_get_int(db, "perf_long_history", 500)

	body := tool.Baro_crumb([][]string{{"/baro", "바로위키"}, {"/baro/security", tool.Baro_t("보안")}, {"", tool.Baro_t("성능")}})
	body += tool.Baro_hero(tool.Baro_t("성능"), tool.Baro_t("무엇이 느려질 수 있는지 실제 데이터로 확인합니다."), "")
	body += baro_sec_tabs("/baro/security/perf")

	if baro_sec_q(query, "done") != "" {
		body += tool.Baro_alert("ok", tool.Baro_t("기준값을 저장했습니다."))
	}

	doc_count := baro_sec_table_count(db, "data")
	history_count := baro_sec_table_count(db, "history")
	doc_size := baro_sec_perf_text_size(db, "data", "data")
	history_size := baro_sec_perf_text_size(db, "history", "data")

	body += tool.Baro_stat_row([][]string{
		{"문서", tool.Baro_comma(doc_count), tool.Baro_t("data 표")},
		{tool.Baro_t("편집 기록"), tool.Baro_comma(history_count), tool.Baro_t("history 표")},
		{tool.Baro_t("본문 총 길이"), tool.Baro_comma(doc_size) + tool.Baro_t("자"), tool.Baro_t("문서 본문 글자 수 합계")},
		{tool.Baro_t("DB 파일"), baro_sec_size_text(baro_sec_db_file_size()), func() string {
			if tool.Get_DB_type() == "sqlite" {
				return tool.Baro_t("SQLite 파일")
			}

			return tool.Baro_t("MySQL 은 파일 크기를 못 봅니다")
		}()},
	})

	// --- 표별 줄 수 ---
	tables := baro_sec_perf_table_list()
	page := baro_sec_page_num(query)
	size := 40

	start := (page - 1) * size
	end := start + size
	if start > len(tables) {
		start = len(tables)
	}
	if end > len(tables) {
		end = len(tables)
	}

	table_rows := [][]string{}
	for _, table_name := range tables[start:end] {
		count := baro_sec_table_count(db, table_name)

		size_text := tool.Baro_note("-")
		switch table_name {
		case "data":
			size_text = tool.HTML_escape(tool.Baro_comma(doc_size) + tool.Baro_t("자"))
		case "history":
			size_text = tool.HTML_escape(tool.Baro_comma(history_size) + tool.Baro_t("자"))
		}

		state := tool.Baro_badge(tool.Baro_t("작음"), "ok")
		if count >= 100000 {
			state = tool.Baro_badge(tool.Baro_t("큼 (인덱스 확인)"), "danger")
		} else if count >= 10000 {
			state = tool.Baro_badge(tool.Baro_t("보통"), "warn")
		}

		table_rows = append(table_rows, []string{
			tool.HTML_escape(table_name),
			tool.Baro_comma(count),
			size_text,
			state,
		})
	}

	table_body := tool.Baro_note(tool.Baro_t("본체 표와 바로위키 표를 모두 셉니다. 크기는 본문 길이를 잴 수 있는 표만 글자 수로 표시합니다."))
	table_body += tool.Baro_table([]string{tool.Baro_t("표 이름"), tool.Baro_t("줄 수"), tool.Baro_t("대략적인 크기"), tool.Baro_t("판정")}, table_rows, tool.Baro_t("표가 없습니다."))
	table_body += tool.Baro_paging(page, len(tables[start:end]), size, "/baro/security/perf?page={}")
	table_body += tool.Baro_note(tool.Baro_t("무엇을 하면 되나 : 줄 수가 10만을 넘는 표는 자주 쓰는 조회 조건에 인덱스가 있는지 아래 목록에서 확인하세요."))

	body += tool.Baro_card(tool.Baro_t("표별 줄 수와 크기"), table_body)

	// --- 인덱스 ---
	index_rows := baro_sec_perf_index_rows(db)
	index_table := [][]string{}

	for _, row := range index_rows {
		index_table = append(index_table, []string{
			tool.HTML_escape(row[0]),
			tool.HTML_escape(row[1]),
			tool.HTML_escape(tool.Baro_trunc(row[2], 90)),
		})
	}

	index_body := ""
	if tool.Get_DB_type() != "sqlite" {
		index_body = tool.Baro_alert("info", tool.Baro_t("MySQL 에서는 이 화면이 인덱스를 읽지 않습니다. <code>show index from 표이름</code> 으로 확인하세요."))
	} else if len(index_table) == 0 {
		index_body = tool.Baro_empty(tool.Baro_t("인덱스를 찾지 못했습니다."))
	} else {
		index_body = tool.Baro_table([]string{tool.Baro_t("인덱스"), tool.Baro_t("표"), tool.Baro_t("정의")}, index_table, "")
	}

	index_body += tool.Baro_note(tool.Baro_t("무엇을 하면 되나 : 목록·검색이 느리면 그 조회가 쓰는 컬럼 조합에 인덱스가 있는지 봅니다. 없으면 위키 관리자에게 추가를 요청하세요."))
	body += tool.Baro_card(tool.Baro_t("인덱스 목록 (")+tool.Baro_itoa(len(index_rows))+tool.Baro_t("개)"), index_body)

	// --- 큰 문서 ---
	big_rows := [][]string{}
	big_query := tool.Query_DB(db, "select title, length(data) from data order by length(data) desc limit 20")
	defer big_query.Close()

	for big_query.Next() {
		title, length := "", 0
		if err := big_query.Scan(&title, &length); err != nil {
			continue
		}

		state := tool.Baro_badge(tool.Baro_t("괜찮음"), "ok")
		if length >= big_doc {
			state = tool.Baro_badge(tool.Baro_t("쪽 나눔 권장"), "danger")
		} else if length >= big_doc/2 {
			state = tool.Baro_badge(tool.Baro_t("지켜보기"), "warn")
		}

		big_rows = append(big_rows, []string{
			tool.Baro_doc_link(title),
			tool.Baro_comma(length),
			state,
		})
	}

	big_body := tool.Baro_note(tool.Baro_t("기준 : ") + tool.Baro_comma(big_doc) + tool.Baro_t("자 이상이면 쪽 나눔을 권합니다."))
	big_body += tool.Baro_table([]string{"문서", tool.Baro_t("본문 길이"), tool.Baro_t("판정")}, big_rows, tool.Baro_t("문서가 없습니다."))
	big_body += tool.Baro_note(
		tool.Baro_t("무엇을 하면 되나 : 긴 문서는 하위 문서로 나누고, 본문에는 요약과 링크만 남깁니다. ") +
			tool.Baro_t("표가 아주 크면 목록을 별도 문서로 옮기세요. 이 위키는 문서 렌더 결과를 저장해 두지 않으므로, 긴 문서는 볼 때마다 처리 비용이 듭니다."),
	)

	body += tool.Baro_card(tool.Baro_t("큰 문서"), big_body)

	// --- 역사가 긴 문서 ---
	long_rows := [][]string{}
	long_query := tool.Query_DB(db, "select title, count(*) from history group by title order by count(*) desc limit 20")
	defer long_query.Close()

	for long_query.Next() {
		title, count := "", 0
		if err := long_query.Scan(&title, &count); err != nil {
			continue
		}

		state := tool.Baro_badge(tool.Baro_t("괜찮음"), "ok")
		if count >= long_history {
			state = tool.Baro_badge(tool.Baro_t("역사 쪽 나눔 필요"), "danger")
		} else if count >= long_history/2 {
			state = tool.Baro_badge(tool.Baro_t("지켜보기"), "warn")
		}

		long_rows = append(long_rows, []string{
			tool.Baro_doc_link(title),
			tool.Baro_comma(count),
			state,
			tool.Baro_button("/history/"+tool.Url_parser(title), "역사 보기", ""),
		})
	}

	long_body := tool.Baro_note(tool.Baro_t("기준 : 판 ") + tool.Baro_comma(long_history) + tool.Baro_t("개 이상이면 역사 목록이 무거워집니다."))
	long_body += tool.Baro_table([]string{"문서", tool.Baro_t("판 수"), tool.Baro_t("판정"), "이동"}, long_rows, tool.Baro_t("편집 기록이 없습니다."))
	long_body += tool.Baro_note(
		tool.Baro_t("무엇을 하면 되나 : 역사 화면은 한 번에 전부 불러오지 말고 쪽 단위로 봅니다. ") +
			tool.Baro_t("편집 전쟁으로 판이 급격히 늘었다면 문서 보호와 토론으로 먼저 진정시키세요."),
	)

	body += tool.Baro_card(tool.Baro_t("역사가 긴 문서"), long_body)

	// --- 이미지 지연 로딩 · 캐시 ---
	image_dir := tool.Get_file_main_dir(db)
	file_names, cut := baro_sec_file_list(image_dir, 3000)

	cache_body := tool.Baro_table(
		[]string{tool.Baro_t("항목"), tool.Baro_t("현재 상태"), tool.Baro_t("무엇을 하면 되나")},
		[][]string{
			{
				tool.Baro_t("이미지 지연 로딩 (문서 본문)"),
				tool.Baro_badge(tool.Baro_t("적용 안 됨"), "warn"),
				tool.Baro_t("본문 이미지 태그에는 <code>loading</code> 속성이 없습니다. 긴 문서에서 이미지가 많으면 첫 화면이 느려집니다. 이미지를 적게 쓰거나 문서를 나누세요."),
			},
			{
				tool.Baro_t("이미지 지연 로딩 (프로필·삽입 틀)"),
				tool.Baro_badge(tool.Baro_t("적용됨"), "ok"),
				tool.Baro_t("프로필 이미지와 삽입 틀에는 <code>loading=\"lazy\"</code> 가 붙어 있습니다. 그대로 두면 됩니다."),
			},
			{
				tool.Baro_t("이미지 캐시 무효화"),
				tool.Baro_badge(tool.Baro_t("적용됨"), "ok"),
				tool.Baro_t("이미지 주소에 판 번호(<code>.cache_v</code>)가 붙어 있어, 파일이 바뀌면 주소도 바뀝니다. 브라우저 캐시를 길게 잡아도 안전합니다."),
			},
			{
				tool.Baro_t("문서 렌더 결과 캐시"),
				tool.Baro_badge("없음", "warn"),
				tool.Baro_t("문서를 볼 때마다 마크업을 새로 처리합니다. 아주 긴 문서를 나누는 것이 가장 효과가 큽니다."),
			},
			{
				tool.Baro_t("보관된 파일 수"),
				tool.Baro_badge(tool.Baro_comma(len(file_names))+tool.Baro_t("개"), "info"),
				func() string {
					if cut {
						return tool.Baro_t("3000개까지만 셌습니다. 파일이 많으면 정기적으로 쓰이지 않는 파일을 정리하세요.")
					}

					return tool.Baro_t("파일 보관 폴더 : ") + tool.HTML_escape(image_dir)
				}(),
			},
		},
		"",
	)

	body += tool.Baro_card(tool.Baro_t("이미지 지연 로딩 · 캐시"), cache_body)

	// --- 기준값 설정 ---
	form := tool.Baro_hidden("op", "save")
	form += tool.Baro_input_type("big_doc", tool.Baro_t("큰 문서 기준 (글자)"), tool.Baro_itoa(big_doc), "number", tool.Baro_t("이 길이를 넘으면 쪽 나눔을 권합니다."))
	form += tool.Baro_input_type("long_history", tool.Baro_t("긴 역사 기준 (판)"), tool.Baro_itoa(long_history), "number", tool.Baro_t("이 수를 넘으면 역사 목록이 무거워집니다."))
	form += baro_sec_reauth_field(db, config)
	form += tool.Baro_submit(tool.Baro_t("기준값 저장"), "primary")

	body += tool.Baro_card(tool.Baro_t("점검 기준"), tool.Baro_form("/baro/security/perf", form))

	return baro_sec_page(db, config, tool.Baro_t("성능"), body)
}

// baro_sec_perf_post : 성능 기준값 저장
func baro_sec_perf_post(db *sql.DB, config tool.Config, values url.Values) string {
	if !baro_sec_is_system(db, config.IP) {
		return tool.Baro_deny(db, config, tool.Baro_t("성능 기준값은 system 권한만 바꿀 수 있습니다."))
	}

	if baro_sec_v(values, "op") != "save" {
		return tool.Get_redirect("/baro/security/perf")
	}

	if ok, message := baro_sec_reauth_ok(db, config, values); !ok {
		return tool.Get_redirect("/baro/security/perf?fail=" + url.QueryEscape(message))
	}

	before := "big_doc=" + tool.Baro_itoa(tool.Baro_get_int(db, "perf_big_doc", 60000)) +
		" long_history=" + tool.Baro_itoa(tool.Baro_get_int(db, "perf_long_history", 500))

	tool.Baro_put(db, "perf_big_doc", tool.Baro_itoa(baro_sec_int_def(baro_sec_v(values, "big_doc"), 60000)))
	tool.Baro_put(db, "perf_long_history", tool.Baro_itoa(baro_sec_int_def(baro_sec_v(values, "long_history"), 500)))

	after := "big_doc=" + tool.Baro_itoa(tool.Baro_get_int(db, "perf_big_doc", 60000)) +
		" long_history=" + tool.Baro_itoa(tool.Baro_get_int(db, "perf_long_history", 500))

	baro_sec_log_op(db, config, "sec_perf_set", tool.Baro_t("성능 점검 기준"), before, after, tool.Baro_t("성능 점검 기준값 변경"))

	return tool.Get_redirect("/baro/security/perf?done=1")
}

// baro_sec_perf_summary : 다른 화면에서 쓰는 한 줄 요약
func baro_sec_perf_summary(db *sql.DB) string {
	return "문서 " + tool.Baro_comma(baro_sec_table_count(db, "data")) +
		tool.Baro_t("개 · 편집 기록 ") + tool.Baro_comma(baro_sec_table_count(db, "history")) +
		tool.Baro_t("개 · DB ") + strings.ToUpper(tool.Get_DB_type())
}
