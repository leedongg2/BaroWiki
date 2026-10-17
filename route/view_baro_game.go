package route

import (
	"database/sql"
	"math/rand"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"opennamu/route/tool"
)

// ============================================================
//  바로게임 — /baro/game
//
//  위키를 읽게 만드는 가장 쉬운 방법은 놀게 만드는 것이다.
//  바로게임은 위키 안의 실제 문서만 가지고 노는 놀이 모음이다.
//
//    문서 맞히기 (/baro/game/quiz)
//      한 문서의 첫 대목을 보여 주고 제목을 가린다. 넷 가운데 골라 맞힌다.
//
//    위키 레이스 (/baro/game/race)
//      출발 문서에서 도착 문서까지 문서 안 링크만 밟아 간다. 적게 밟으면 이긴다.
//
//  기록은 baro_game_play 에 남고 /baro/game/rank 에서 함께 본다.
//  풀이 중인 문제는 baro_game_state 에 담아 두어 답이 화면에 새지 않게 한다.
// ============================================================

// baro_game_quiz_choice : 문제 하나의 보기 수
const baro_game_quiz_choice = 4

// baro_game_race_max_move : 레이스 최대 걸음 (넘으면 실패)
const baro_game_race_max_move = 40

// baro_game_snippet_length : 문제에 쓰는 본문 길이
const baro_game_snippet_length = 320

// baro_game_pool_max : 문제로 쓸 문서를 고를 때 훑는 최대 수
//
//	위키가 커져도 한 판 만드는 값이 크게 늘지 않게 막아 둔다.
const baro_game_pool_max = 5000

// baro_game_block_regex : 여러 줄에 걸친 문법 덩어리 ({{{ … }}})
var baro_game_block_regex = regexp.MustCompile(`(?s)\{\{\{.*?\}\}\}`)

// baro_game_line_regex : 한 줄짜리 문법 (표 · 제목 · 목록 · 주석)
var baro_game_line_regex = regexp.MustCompile(`(?m)^[ \t]*(?:\|\|.*|=+.*|#+.*|\*.*|[0-9aAiI]\..*)$`)

// baro_game_inline_regex : 줄 안에 들어가는 문법
var baro_game_inline_regex = regexp.MustCompile(`\[\[[^\]\[]*\]\]|\[[^\]\[]*\]|&[a-zA-Z#0-9]+;|'''|''|__|\^\^|,,|--|~~`)

var baro_game_space_regex = regexp.MustCompile(`[ \t]+`)

// baro_game_list : 놀이 목록 (열쇠말, 이름, 설명, 주소)
func baro_game_list() [][]string {
	return [][]string{
		{"quiz", tool.Baro_t("문서 맞히기"), tool.Baro_t("첫 대목만 보고 어떤 문서인지 맞힙니다."), "/baro/game/quiz"},
		{"race", tool.Baro_t("위키 레이스"), tool.Baro_t("문서 안 링크만 밟아 도착 문서까지 갑니다."), "/baro/game/race"},
	}
}

// baro_game_name : 놀이 이름
func baro_game_name(key string) string {
	for _, one := range baro_game_list() {
		if one[0] == key {
			return one[1]
		}
	}

	return key
}

// baro_game_menu : 공통 메뉴
func baro_game_menu() [][]any {
	return [][]any{
		{"baro/game", tool.Baro_t("바로게임")},
		{"baro", "바로위키"},
	}
}

// baro_game_tabs : 탭 줄
func baro_game_tabs(active string) string {
	items := [][]string{{"/baro/game", tool.Baro_t("놀이 고르기")}}
	for _, one := range baro_game_list() {
		items = append(items, []string{one[3], one[1]})
	}
	items = append(items, []string{"/baro/game/rank", tool.Baro_t("순위표")})

	return tool.Baro_tabs(items, active)
}

// baro_game_player : 순위표에 남길 이름 (로그인 안 했으면 아이피)
func baro_game_player(config tool.Config) string {
	return config.IP
}

// baro_game_seat : 풀이 중인 상태를 담아 둘 자리
//
//	로그인한 사람은 이름으로 나뉜다.
//	로그인하지 않은 사람을 아이피로만 나누면 같은 공유기를 쓰는 두 사람이
//	서로의 문제를 덮어쓴다. 그래서 웹 자리(세션)를 함께 쓴다.
func baro_game_seat(config tool.Config) string {
	if tool.Baro_is_user(config.IP) {
		return config.IP
	}

	if config.Session != nil {
		if seat, ok := config.Session.Get("baro_game_seat").(string); ok && seat != "" {
			return "seat:" + seat
		}

		// 이 자리에 처음 온 사람이면 자리 번호를 하나 만들어 세션에 적어 둔다
		seat := tool.Sha224(config.IP + "|" + config.UserAgent + "|" + tool.Baro_now())[:16]
		config.Session.Set("baro_game_seat", seat)
		_ = config.Session.Save()

		return "seat:" + seat
	}

	return config.IP
}

// baro_game_state_get : 풀이 중인 상태 읽기
func baro_game_state_get(db *sql.DB, player string, game string) map[string]string {
	data := ""
	tool.QueryRow_DB(db, "select data from baro_game_state where user = ? and game = ?", []any{&data}, player, game)

	return baro_game_state_decode(data)
}

// baro_game_state_set : 풀이 중인 상태 저장
func baro_game_state_set(db *sql.DB, player string, game string, state map[string]string) {
	tool.Exec_DB(db, "delete from baro_game_state where user = ? and game = ?", player, game)
	tool.Exec_DB(
		db,
		"insert into baro_game_state (user, game, data, date) values (?, ?, ?, ?)",
		player, game, baro_game_state_encode(state), tool.Baro_now(),
	)
}

// baro_game_state_clear : 풀이 중인 상태 지우기
func baro_game_state_clear(db *sql.DB, player string, game string) {
	tool.Exec_DB(db, "delete from baro_game_state where user = ? and game = ?", player, game)
}

// baro_game_state_encode : 상태를 한 줄로 (탭과 줄바꿈으로 나눈다)
func baro_game_state_encode(state map[string]string) string {
	keys := []string{}
	for key := range state {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	lines := []string{}
	for _, key := range keys {
		value := strings.NewReplacer("\t", " ", "\n", " ").Replace(state[key])
		lines = append(lines, key+"\t"+value)
	}

	return strings.Join(lines, "\n")
}

// baro_game_state_decode : 한 줄짜리 상태를 다시 map 으로
func baro_game_state_decode(data string) map[string]string {
	state := map[string]string{}
	for _, line := range strings.Split(data, "\n") {
		key, value, has := strings.Cut(line, "\t")
		if !has {
			continue
		}

		state[key] = value
	}

	return state
}

// baro_game_score_add : 기록 한 줄 남기기
func baro_game_score_add(db *sql.DB, player string, game string, score int, detail string) {
	tool.Exec_DB(
		db,
		"insert into baro_game_play (id, user, game, score, detail, date) values (?, ?, ?, ?, ?, ?)",
		tool.Baro_next_id(db, "baro_game_play"),
		player, game, strconv.Itoa(score), tool.Baro_trunc(detail, 300), tool.Baro_now(),
	)
}

// baro_game_quiz_record : 문서 맞히기 최고 기록 남기기
//
//	이미 남은 최고 기록을 넘을 때만 한 줄 넣는다.
//	한 판마다 줄을 쌓지 않고, 이어 맞히는 중에도 순위표에 바로 오른다.
func baro_game_quiz_record(db *sql.DB, player string, streak int) {
	if streak < 1 {
		return
	}

	best := 0
	tool.QueryRow_DB(
		db,
		"select coalesce(max(score + 0), 0) from baro_game_play where user = ? and game = 'quiz'",
		[]any{&best}, player,
	)
	if streak <= best {
		return
	}

	baro_game_score_add(db, player, "quiz", streak, tool.Baro_t("연속 ")+strconv.Itoa(streak)+tool.Baro_t("문제 맞힘"))
}

// baro_game_random_titles : 놀이에 쓸 문서 이름 뽑기
//
//	본문이 어느 정도 있는 일반 문서만 쓴다 (틀 · 분류 · 파일 · 사용자 문서는 뺀다).
//	**읽을 수 없는 문서는 절대 넣지 않는다.**
//	문서 맞히기는 본문 한 대목을 그대로 보여 주므로, 권한을 빠뜨리면 본문이 새어 나간다.
func baro_game_random_titles(db *sql.DB, config tool.Config, need int) []string {
	rows := tool.Query_DB(
		db,
		"select title from data where length(data) > 200 and title not like 'template:%' and title not like 'category:%'"+
			" and title not like 'file:%' and title not like 'user:%' and title not like 'ipuser:%' and title not like 'help:%'"+
			" and title not like 'baro:%' and title not like 'trash:%' limit ?",
		baro_game_pool_max,
	)
	defer rows.Close()

	all := []string{}
	for rows.Next() {
		title := ""
		if rows.Scan(&title) != nil {
			continue
		}

		all = append(all, title)
	}
	rows.Close()

	if len(all) == 0 {
		return nil
	}

	source := rand.New(rand.NewSource(time.Now().UnixNano()))
	source.Shuffle(len(all), func(a int, b int) {
		all[a], all[b] = all[b], all[a]
	})

	// 섞은 뒤에 거른다. 필요한 만큼만 확인하면 되므로 값이 적게 든다.
	filter := tool.New_acl_render_filter(db, config.IP)
	out := []string{}
	for _, title := range all {
		if !filter.Allow(title) {
			continue
		}

		out = append(out, title)
		if need > 0 && len(out) >= need {
			break
		}
	}

	return out
}

// baro_game_snippet : 문제로 쓸 첫 대목 (문법을 걷어 내고 제목을 가린다)
func baro_game_snippet(db *sql.DB, title string) string {
	body := ""
	tool.QueryRow_DB(db, "select data from data where title = ?", []any{&body}, title)
	if body == "" {
		return ""
	}

	// 문법을 지워 읽을 수 있는 글만 남긴다
	body = strings.ReplaceAll(body, "\r", "")
	body = baro_game_block_regex.ReplaceAllString(body, "\n")
	body = baro_game_line_regex.ReplaceAllString(body, "")
	body = baro_game_inline_regex.ReplaceAllString(body, " ")

	clean_lines := []string{}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(baro_game_space_regex.ReplaceAllString(line, " "))
		if tool.Get_len(line) < 20 {
			continue
		}

		clean_lines = append(clean_lines, line)
		if tool.Get_len(strings.Join(clean_lines, " ")) > baro_game_snippet_length {
			break
		}
	}

	text := strings.Join(clean_lines, " ")
	if tool.Get_len(text) < 40 {
		return ""
	}
	text = tool.Get_slice(text, 0, baro_game_snippet_length)

	// 제목이 본문에 그대로 있으면 가린다
	display := tool.Namespace_display(title)
	for _, word := range []string{display, title} {
		if tool.Get_len(word) < 2 {
			continue
		}

		text = strings.ReplaceAll(text, word, "○○○")
	}
	// 괄호 안 한자·원어 표기도 힌트가 되므로 함께 가린다
	if base := strings.SplitN(display, "(", 2)[0]; tool.Get_len(base) >= 2 && base != display {
		text = strings.ReplaceAll(text, base, "○○○")
	}

	return text
}

// View_baro_game : 놀이 고르기
func View_baro_game(config tool.Config) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	player := baro_game_player(config)

	card_html := `<div class="bw_game">`
	for _, one := range baro_game_list() {
		card_html += `<a class="bw_game_card" href="` + one[3] + `">` +
			`<span class="bw_game_t">` + tool.HTML_escape(one[1]) + `</span>` +
			`<span class="bw_game_s">` + tool.HTML_escape(one[2]) + `</span></a>`
	}
	card_html += `</div>`

	play_count := 0
	tool.QueryRow_DB(db, "select count(*) from baro_game_play where user = ?", []any{&play_count}, player)

	// 기록이 하나도 없으면 max/min 이 NULL 이라 숫자로 못 읽는다. coalesce 로 0 을 받는다.
	best_quiz := 0
	tool.QueryRow_DB(db, "select coalesce(max(score + 0), 0) from baro_game_play where user = ? and game = 'quiz'", []any{&best_quiz}, player)

	best_race := 0
	tool.QueryRow_DB(db, "select coalesce(min(score + 0), 0) from baro_game_play where user = ? and game = 'race' and score != '0'", []any{&best_race}, player)

	race_text := "-"
	if best_race > 0 {
		race_text = strconv.Itoa(best_race) + tool.Baro_t("걸음")
	}

	body := tool.Baro_hero(
		tool.Baro_t("바로게임"),
		tool.Baro_t("위키에 실제로 있는 문서만 가지고 노는 놀이입니다. 놀다 보면 문서가 읽힙니다."),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("내가 놀아 본 횟수"), tool.Baro_comma(play_count), ""},
			{tool.Baro_t("문서 맞히기 최고"), tool.Baro_comma(best_quiz) + tool.Baro_t("연속"), ""},
			{tool.Baro_t("레이스 최소 걸음"), race_text, ""},
		}),
	)

	body += tool.Baro_card("", baro_game_tabs("/baro/game")+card_html)
	body += tool.Baro_card(tool.Baro_t("내 기록"), baro_game_my_record_html(db, player, 10))

	return tool.Baro_page(db, config, tool.Baro_t("바로게임"), body, baro_game_menu())
}

// baro_game_my_record_html : 내 기록 표
func baro_game_my_record_html(db *sql.DB, player string, limit int) string {
	rows := tool.Query_DB(
		db,
		"select game, score, detail, date from baro_game_play where user = ? order by date desc, id + 0 desc limit ?",
		player, limit,
	)
	defer rows.Close()

	list := [][]string{}
	for rows.Next() {
		game, score, detail, date := "", "", "", ""
		if rows.Scan(&game, &score, &detail, &date) != nil {
			continue
		}

		list = append(list, []string{
			tool.Baro_time_cell(date),
			tool.HTML_escape(baro_game_name(game)),
			tool.HTML_escape(score),
			tool.HTML_escape(detail),
		})
	}

	return tool.Baro_table(
		[]string{tool.Baro_t("때"), tool.Baro_t("놀이"), tool.Baro_t("점수"), tool.Baro_t("자세히")},
		list, tool.Baro_t("아직 놀아 본 기록이 없습니다."),
	)
}

// View_baro_game_quiz : 문서 맞히기
func View_baro_game_quiz(config tool.Config, values map[string]string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	player := baro_game_player(config)
	seat := baro_game_seat(config)
	state := baro_game_state_get(db, seat, "quiz")
	message := ""

	if values != nil {
		switch values["action"] {
		case "answer":
			answer := state["answer"]
			if answer == "" {
				message = tool.Baro_alert("warn", tool.Baro_t("문제가 사라졌습니다. 새 문제를 받아 주세요."))
				break
			}

			streak := tool.Str_to_int(state["streak"])
			if values["choice"] == answer {
				streak++
				message = tool.Baro_alert("ok",
					tool.Baro_t("맞혔습니다. ")+tool.HTML_escape(tool.Namespace_display(answer))+
						tool.Baro_t(" 문서입니다. 지금 ")+strconv.Itoa(streak)+tool.Baro_t("연속입니다."))

				// 이어 맞히는 동안에도 순위표에 바로 오르게, 최고 기록을 넘을 때만 남긴다
				baro_game_quiz_record(db, player, streak)
			} else {
				baro_game_quiz_record(db, player, streak)
				message = tool.Baro_alert("danger",
					tool.Baro_t("아쉽습니다. 정답은 ")+tool.HTML_escape(tool.Namespace_display(answer))+
						tool.Baro_t(" 였습니다. 연속 기록은 ")+strconv.Itoa(streak)+tool.Baro_t("에서 멈췄습니다."))
				streak = 0
			}

			message += `<div class="bw_card_b">` +
				tool.Baro_button("/w/" + tool.Doc_url(answer), tool.Baro_t("이 문서 읽어 보기"), "") + `</div>`

			state = baro_game_quiz_make(db, config, streak)
			baro_game_state_set(db, seat, "quiz", state)

		case "next", "start":
			state = baro_game_quiz_make(db, config, tool.Str_to_int(state["streak"]))
			baro_game_state_set(db, seat, "quiz", state)
		}
	}

	if state["answer"] == "" {
		state = baro_game_quiz_make(db, config, tool.Str_to_int(state["streak"]))
		baro_game_state_set(db, seat, "quiz", state)
	}

	body := tool.Baro_hero(
		tool.Baro_t("문서 맞히기"),
		tool.Baro_t("첫 대목만 보고 어떤 문서인지 고릅니다. 제목은 ○○○ 로 가려 둡니다."),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("지금 연속"), tool.Baro_comma(tool.Str_to_int(state["streak"])), ""},
		}),
	)

	inner := baro_game_tabs("/baro/game/quiz")
	inner += message

	if state["answer"] == "" {
		inner += tool.Baro_alert("warn", tool.Baro_t("문제로 쓸 만한 문서가 아직 모자랍니다. 본문이 200자를 넘는 일반 문서가 네 개 이상 있어야 합니다."))
		body += tool.Baro_card("", inner)

		return tool.Baro_page(db, config, tool.Baro_t("문서 맞히기"), body, baro_game_menu())
	}

	inner += `<div class="bw_quiz_q">` + tool.HTML_escape(state["snippet"]) + `</div>`

	choice_html := `<div class="bw_choice">`
	for _, one := range strings.Split(state["choices"], "|") {
		if one == "" {
			continue
		}

		choice_html += `<form method="post" action="/baro/game/quiz">` +
			tool.Baro_hidden("action", "answer") + tool.Baro_hidden("choice", one) +
			`<button class="bw_choice_b" type="submit">` + tool.HTML_escape(tool.Namespace_display(one)) + `</button></form>`
	}
	choice_html += `</div>`

	inner += choice_html
	inner += tool.Baro_form("/baro/game/quiz", tool.Baro_hidden("action", "next")+tool.Baro_submit(tool.Baro_t("다른 문제 주세요"), ""))

	body += tool.Baro_card("", inner)

	return tool.Baro_page(db, config, tool.Baro_t("문서 맞히기"), body, baro_game_menu())
}

// baro_game_quiz_make : 문제 하나 만들기
func baro_game_quiz_make(db *sql.DB, config tool.Config, streak int) map[string]string {
	state := map[string]string{"streak": strconv.Itoa(streak)}

	candidates := baro_game_random_titles(db, config, 60)
	if len(candidates) < baro_game_quiz_choice {
		return state
	}

	answer := ""
	snippet := ""
	for _, title := range candidates {
		if text := baro_game_snippet(db, title); text != "" {
			answer = title
			snippet = text

			break
		}
	}
	if answer == "" {
		return state
	}

	choices := []string{answer}
	for _, title := range candidates {
		if len(choices) >= baro_game_quiz_choice {
			break
		}
		if title == answer {
			continue
		}

		choices = append(choices, title)
	}
	if len(choices) < baro_game_quiz_choice {
		return state
	}

	source := rand.New(rand.NewSource(time.Now().UnixNano()))
	source.Shuffle(len(choices), func(a int, b int) {
		choices[a], choices[b] = choices[b], choices[a]
	})

	state["answer"] = answer
	state["snippet"] = snippet
	state["choices"] = strings.Join(choices, "|")

	return state
}

// baro_game_out_links : 그 문서에서 나가는 문서 링크 (실제로 있는 문서만)
func baro_game_out_links(db *sql.DB, filter *tool.Acl_render_filter, title string) []string {
	rows := tool.Query_DB(
		db,
		"select distinct b.title from back b where b.link = ? and b.type = '' and exists (select 1 from data d where d.title = b.title)",
		title,
	)
	defer rows.Close()

	found := []string{}
	for rows.Next() {
		target := ""
		if rows.Scan(&target) != nil {
			continue
		}
		if target == "" || target == title {
			continue
		}

		found = append(found, target)
	}
	rows.Close()

	// 읽을 수 없는 문서로는 갈 수 없다 (제목이 단추로 보이면 그것만으로도 새어 나간다)
	out := []string{}
	for _, one := range found {
		if filter.Allow(one) {
			out = append(out, one)
		}
	}

	sort.Strings(out)

	return out
}

// View_baro_game_race : 위키 레이스
func View_baro_game_race(config tool.Config, values map[string]string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	player := baro_game_player(config)
	seat := baro_game_seat(config)
	state := baro_game_state_get(db, seat, "race")
	message := ""

	// 읽기 권한 거르개는 이 요청 동안 한 번만 만든다
	race_filter := tool.New_acl_render_filter(db, config.IP)

	if values != nil {
		switch values["action"] {
		case "start":
			new_state, make_error := baro_game_race_make(db, config)
			if make_error != "" {
				message = tool.Baro_alert("warn", make_error)
				state = map[string]string{}
			} else {
				state = new_state
				baro_game_state_set(db, seat, "race", state)
			}

		case "move":
			if state["goal"] == "" {
				message = tool.Baro_alert("warn", tool.Baro_t("진행 중인 레이스가 없습니다. 새로 시작해 주세요."))

				break
			}

			step := strings.TrimSpace(values["target"])
			allowed := false
			for _, one := range baro_game_out_links(db, race_filter, state["current"]) {
				if one == step {
					allowed = true

					break
				}
			}
			if !allowed {
				message = tool.Baro_alert("danger", tool.Baro_t("지금 문서에서 갈 수 없는 문서입니다."))

				break
			}

			moves := tool.Str_to_int(state["moves"]) + 1
			state["current"] = step
			state["moves"] = strconv.Itoa(moves)
			state["path"] = strings.TrimPrefix(state["path"]+" › "+tool.Namespace_display(step), " › ")

			if step == state["goal"] {
				spent := baro_game_race_seconds(state["start_date"])
				baro_game_score_add(db, player, "race", moves,
					tool.Namespace_display(state["start"])+" → "+tool.Namespace_display(state["goal"])+
						" / "+strconv.Itoa(spent)+tool.Baro_t("초"))
				message = tool.Baro_alert("ok",
					tool.Baro_t("도착했습니다! ")+strconv.Itoa(moves)+tool.Baro_t("걸음, ")+strconv.Itoa(spent)+tool.Baro_t("초 걸렸습니다."))
				baro_game_state_clear(db, seat, "race")
				state = map[string]string{}

				break
			}

			if moves >= baro_game_race_max_move {
				message = tool.Baro_alert("danger", tool.Baro_t("걸음이 너무 많아졌습니다. 이번 레이스는 여기서 끝냅니다."))
				baro_game_score_add(db, player, "race", 0, tool.Baro_t("실패 (")+strconv.Itoa(moves)+tool.Baro_t("걸음)"))
				baro_game_state_clear(db, seat, "race")
				state = map[string]string{}

				break
			}

			baro_game_state_set(db, seat, "race", state)

		case "give_up":
			if state["goal"] != "" {
				baro_game_score_add(db, player, "race", 0, tool.Baro_t("포기 (")+state["moves"]+tool.Baro_t("걸음)"))
			}
			baro_game_state_clear(db, seat, "race")
			state = map[string]string{}
			message = tool.Baro_alert("info", tool.Baro_t("레이스를 그만뒀습니다."))
		}
	}

	inner := baro_game_tabs("/baro/game/race")
	inner += message

	if state["goal"] == "" {
		inner += tool.Baro_note("출발 문서와 도착 문서를 무작위로 뽑아 드립니다. 문서 안 링크만 밟아서 도착까지 가면 됩니다.")
		inner += tool.Baro_form("/baro/game/race", tool.Baro_hidden("action", "start")+tool.Baro_submit(tool.Baro_t("레이스 시작"), "primary"))

		body := tool.Baro_hero(tool.Baro_t("위키 레이스"), tool.Baro_t("문서 안 링크만 밟아 도착 문서까지 갑니다. 적게 밟으면 이깁니다."), "")
		body += tool.Baro_card("", inner)
		body += tool.Baro_card(tool.Baro_t("최근 기록"), baro_game_rank_html(db, config, "race", 10))

		return tool.Baro_page(db, config, tool.Baro_t("위키 레이스"), body, baro_game_menu())
	}

	links := baro_game_out_links(db, race_filter, state["current"])
	link_html := `<div class="bw_race_links">`
	for _, one := range links {
		class_name := "bw_race_link"
		if one == state["goal"] {
			class_name += " bw_race_goal"
		}

		link_html += `<form method="post" action="/baro/game/race" style="display:inline">` +
			tool.Baro_hidden("action", "move") + tool.Baro_hidden("target", one) +
			`<button class="` + class_name + `" type="submit">` + tool.HTML_escape(tool.Namespace_display(one)) + `</button></form>`
	}
	link_html += `</div>`

	if len(links) == 0 {
		link_html = tool.Baro_alert("warn", tool.Baro_t("이 문서에는 밟을 수 있는 링크가 없습니다. 막다른 길입니다."))
	}

	body := tool.Baro_hero(
		tool.Baro_t("위키 레이스"),
		tool.Namespace_display(state["start"])+" → "+tool.Namespace_display(state["goal"]),
		tool.Baro_stat_row([][]string{
			{tool.Baro_t("지금 문서"), tool.Namespace_display(state["current"]), ""},
			{tool.Baro_t("걸음"), state["moves"], tool.Baro_t("최대 ") + strconv.Itoa(baro_game_race_max_move)},
			{tool.Baro_t("지난 시간"), strconv.Itoa(baro_game_race_seconds(state["start_date"])) + tool.Baro_t("초"), ""},
		}),
	)

	inner += tool.Baro_alert("info", tool.Baro_t("도착 문서는 <b>")+tool.HTML_escape(tool.Namespace_display(state["goal"]))+tool.Baro_t("</b> 입니다. 초록색 단추가 보이면 바로 누르세요."))
	inner += `<div class="bw_card_s">` + tool.HTML_escape(state["path"]) + `</div>`
	inner += link_html
	inner += tool.Baro_button("/w/" + tool.Doc_url(state["current"]), tool.Baro_t("지금 문서 읽어 보기"), "") + " "
	inner += tool.Baro_form("/baro/game/race", tool.Baro_hidden("action", "give_up")+tool.Baro_submit(tool.Baro_t("그만두기"), "danger"))

	body += tool.Baro_card("", inner)

	return tool.Baro_page(db, config, tool.Baro_t("위키 레이스"), body, baro_game_menu())
}

// baro_game_race_seconds : 시작한 뒤 지난 초
func baro_game_race_seconds(start_date string) int {
	started, err := time.Parse("2006-01-02 15:04:05", start_date)
	if err != nil {
		return 0
	}

	spent := int(time.Since(started).Seconds())
	if spent < 0 {
		return 0
	}

	return spent
}

// baro_game_race_make : 출발 · 도착 문서 뽑기
//
//	출발 문서는 나가는 링크가 있어야 하고, 도착 문서는 들어오는 링크가 있어야 한다.
//	둘 다 없으면 애초에 갈 수가 없다.
func baro_game_race_make(db *sql.DB, config tool.Config) (map[string]string, string) {
	filter := tool.New_acl_render_filter(db, config.IP)
	candidates := baro_game_random_titles(db, config, 80)
	if len(candidates) < 2 {
		return nil, tool.Baro_t("레이스에 쓸 문서가 모자랍니다. 문서가 조금 더 쌓이면 놀 수 있습니다.")
	}

	start := ""
	for _, title := range candidates {
		if len(baro_game_out_links(db, filter, title)) >= 2 {
			start = title

			break
		}
	}
	if start == "" {
		return nil, tool.Baro_t("문서 사이 링크가 아직 모자랍니다. 문서에 [[링크]]를 달면 레이스를 할 수 있습니다.")
	}

	goal := ""
	for index := len(candidates) - 1; index >= 0; index-- {
		title := candidates[index]
		if title == start {
			continue
		}

		count := 0
		tool.QueryRow_DB(db, "select count(*) from back where title = ? and type = ''", []any{&count}, title)
		if count > 0 {
			goal = title

			break
		}
	}
	if goal == "" {
		return nil, tool.Baro_t("도착으로 쓸 문서를 찾지 못했습니다. 다른 문서에서 링크된 문서가 있어야 합니다.")
	}

	return map[string]string{
		"start":      start,
		"goal":       goal,
		"current":    start,
		"moves":      "0",
		"path":       tool.Namespace_display(start),
		"start_date": tool.Baro_now(),
	}, ""
}

// baro_game_rank_html : 놀이별 순위표
func baro_game_rank_html(db *sql.DB, config tool.Config, game string, limit int) string {
	order := "score + 0 desc"
	if game == "race" {
		// 레이스는 걸음이 적어야 잘한 것이다 (0 은 실패라 뺀다)
		order = "score + 0 asc"
	}

	query := "select user, score, detail, date from baro_game_play where game = ?"
	if game == "race" {
		query += " and score != '0'"
	}
	query += " order by " + order + " limit ?"

	rows := tool.Query_DB(db, query, game, limit)
	defer rows.Close()

	list := [][]string{}
	rank := 0
	for rows.Next() {
		user_name, score, detail, date := "", "", "", ""
		if rows.Scan(&user_name, &score, &detail, &date) != nil {
			continue
		}

		rank++
		list = append(list, []string{
			strconv.Itoa(rank),
			tool.Baro_user_link(db, user_name, config.IP),
			tool.HTML_escape(score),
			tool.HTML_escape(detail),
			tool.Baro_time_cell(date),
		})
	}

	score_label := tool.Baro_t("연속 맞힘")
	if game == "race" {
		score_label = tool.Baro_t("걸음")
	}

	return tool.Baro_table(
		[]string{tool.Baro_t("등수"), tool.Get_language(db, "user", true), score_label, tool.Baro_t("자세히"), tool.Baro_t("때")},
		list, tool.Baro_t("아직 기록이 없습니다."),
	)
}

// View_baro_game_rank : 순위표
func View_baro_game_rank(config tool.Config) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)

	body := tool.Baro_hero(tool.Baro_t("바로게임 순위표"), tool.Baro_t("놀이마다 잘한 순서대로 모았습니다."), "")
	body += tool.Baro_card("", baro_game_tabs("/baro/game/rank"))
	body += tool.Baro_card(tool.Baro_t("문서 맞히기"), baro_game_rank_html(db, config, "quiz", 20))
	body += tool.Baro_card(tool.Baro_t("위키 레이스"), baro_game_rank_html(db, config, "race", 20))

	return tool.Baro_page(db, config, tool.Baro_t("바로게임 순위표"), body, baro_game_menu())
}
