package tool

import (
	"database/sql"
	"strings"
)

// ============================================================
//  줄 단위 비교와 3자 병합
//
//  편집 충돌이 났을 때 "충돌입니다" 하고 끝내면 신규 기여자는 쓰던 글을 잃고 떠난다. (3-3)
//  기준 판 · 내 수정 · 최신 수정을 셋 다 놓고 자동 병합할 수 있는 부분은 합치고,
//  겹치는 부분만 사람에게 고르게 한다.
// ============================================================

// baro_merge_max : 이 줄 수를 넘으면 자동 병합을 포기하고 통째로 충돌로 다룬다
const baro_merge_max = 1200

// Baro_split_line : 본문을 줄 목록으로 (CR 제거)
func Baro_split_line(text string) []string {
	if text == "" {
		return []string{}
	}

	return strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")
}

// Baro_join_line : 줄 목록을 본문으로
func Baro_join_line(lines []string) string {
	return strings.Join(lines, "\n")
}

// baro_lcs_pairs : 두 줄 목록의 공통 부분 (a 인덱스, b 인덱스) 짝
func baro_lcs_pairs(a []string, b []string) [][2]int {
	pairs := [][2]int{}

	// 앞뒤로 같은 줄은 먼저 떼어 낸다 (표 크기를 줄이기 위해)
	head := 0
	for head < len(a) && head < len(b) && a[head] == b[head] {
		pairs = append(pairs, [2]int{head, head})
		head++
	}

	tail := 0
	for tail < len(a)-head && tail < len(b)-head &&
		a[len(a)-1-tail] == b[len(b)-1-tail] {
		tail++
	}

	mid_a := a[head : len(a)-tail]
	mid_b := b[head : len(b)-tail]

	if len(mid_a) > 0 && len(mid_b) > 0 {
		if len(mid_a) <= baro_merge_max && len(mid_b) <= baro_merge_max {
			table := make([][]int32, len(mid_a)+1)
			for i := range table {
				table[i] = make([]int32, len(mid_b)+1)
			}

			for i := len(mid_a) - 1; i >= 0; i-- {
				for j := len(mid_b) - 1; j >= 0; j-- {
					if mid_a[i] == mid_b[j] {
						table[i][j] = table[i+1][j+1] + 1
					} else if table[i+1][j] >= table[i][j+1] {
						table[i][j] = table[i+1][j]
					} else {
						table[i][j] = table[i][j+1]
					}
				}
			}

			i, j := 0, 0
			for i < len(mid_a) && j < len(mid_b) {
				if mid_a[i] == mid_b[j] {
					pairs = append(pairs, [2]int{head + i, head + j})
					i++
					j++
				} else if table[i+1][j] >= table[i][j+1] {
					i++
				} else {
					j++
				}
			}
		}
	}

	for t := tail; t > 0; t-- {
		pairs = append(pairs, [2]int{len(a) - t, len(b) - t})
	}

	return pairs
}

// Baro_diff_line : 줄 단위 변경 목록
//
//	각 항목 : {종류, 내용}  — 종류는 same / del / add
func Baro_diff_line(old_text string, new_text string) [][2]string {
	a := Baro_split_line(old_text)
	b := Baro_split_line(new_text)

	out := [][2]string{}
	pairs := baro_lcs_pairs(a, b)

	ai, bi := 0, 0
	for _, pair := range pairs {
		for ai < pair[0] {
			out = append(out, [2]string{"del", a[ai]})
			ai++
		}
		for bi < pair[1] {
			out = append(out, [2]string{"add", b[bi]})
			bi++
		}

		out = append(out, [2]string{"same", a[pair[0]]})
		ai = pair[0] + 1
		bi = pair[1] + 1
	}

	for ai < len(a) {
		out = append(out, [2]string{"del", a[ai]})
		ai++
	}
	for bi < len(b) {
		out = append(out, [2]string{"add", b[bi]})
		bi++
	}

	return out
}

// Baro_diff_html : 줄 단위 비교를 표로 (변경된 곳만, 앞뒤 3줄씩)
func Baro_diff_html(old_text string, new_text string, label_old string, label_new string) string {
	ops := Baro_diff_line(old_text, new_text)

	// 변경 없는 긴 구간은 접는다
	show := make([]bool, len(ops))
	for i, op := range ops {
		if op[0] == "same" {
			continue
		}

		for j := i - 3; j <= i+3; j++ {
			if j >= 0 && j < len(ops) {
				show[j] = true
			}
		}
	}

	html := `<div class="bw_diff_head"><span class="bw_diff_o">− ` + HTML_escape(label_old) + `</span> <span class="bw_diff_n">+ ` + HTML_escape(label_new) + `</span></div>`
	html += `<div class="bw_table_wrap"><table class="bw_table bw_diff"><tbody>`

	changed := false
	skipped := 0

	flush_skip := func() {
		if skipped > 0 {
			html += `<tr><td class="bw_diff_skip" colspan="2">… 같은 내용 ` + Baro_itoa(skipped) + `줄 생략 …</td></tr>`
			skipped = 0
		}
	}

	for i, op := range ops {
		if !show[i] {
			skipped++
			continue
		}

		flush_skip()

		switch op[0] {
		case "del":
			changed = true
			html += `<tr class="bw_diff_r_o"><td class="bw_diff_m">−</td><td>` + HTML_escape(op[1]) + `</td></tr>`
		case "add":
			changed = true
			html += `<tr class="bw_diff_r_n"><td class="bw_diff_m">+</td><td>` + HTML_escape(op[1]) + `</td></tr>`
		default:
			html += `<tr><td class="bw_diff_m"> </td><td>` + HTML_escape(op[1]) + `</td></tr>`
		}
	}

	flush_skip()

	if !changed {
		html += `<tr><td class="bw_table_e" colspan="2">달라진 내용이 없습니다.</td></tr>`
	}

	html += `</tbody></table></div>` + Baro_diff_css()

	return html
}

// Baro_diff_css : 비교 표 스타일
func Baro_diff_css() string {
	return `<style>
.bw_diff td{font-family:ui-monospace,SFMono-Regular,Consolas,monospace;font-size:.85rem;white-space:pre-wrap;word-break:break-all;}
.bw_diff_m{width:1.4rem;text-align:center;color:#888;user-select:none;}
.bw_diff_r_o{background:rgba(220,67,67,.12);}
.bw_diff_r_n{background:rgba(40,180,114,.14);}
.bw_diff_skip{text-align:center;color:#888;font-size:.8rem;background:rgba(127,127,127,.06);}
.bw_diff_head{font-size:.86rem;margin:.4rem 0;}
.bw_diff_o{color:#b12b2b;font-weight:600;}
.bw_diff_n{color:#1a7f4f;font-weight:600;margin-left:.6rem;}
.bw_conf{background:rgba(230,160,20,.14);}
</style>`
}

// Baro_merge_mark : 충돌 표시 줄
const (
	Baro_mark_mine   = "<<<<<<< 내 수정"
	Baro_mark_split  = "======="
	Baro_mark_theirs = ">>>>>>> 최신 수정"
)

// Baro_merge3 : 기준 판을 놓고 내 수정과 최신 수정을 합친다
//
//	돌려주는 값 : (합쳐진 본문, 충돌 덩어리 수, 자동으로 합쳐진 덩어리 수)
func Baro_merge3(base_text string, mine_text string, theirs_text string) (string, int, int) {
	base := Baro_split_line(base_text)
	mine := Baro_split_line(mine_text)
	theirs := Baro_split_line(theirs_text)

	// 기준 판을 못 찾았거나 너무 크면 통째로 충돌로 다룬다
	if base_text == "" || len(base) > baro_merge_max*3 {
		out := []string{Baro_mark_mine}
		out = append(out, mine...)
		out = append(out, Baro_mark_split)
		out = append(out, theirs...)
		out = append(out, Baro_mark_theirs)

		return Baro_join_line(out), 1, 0
	}

	mine_pairs := baro_lcs_pairs(base, mine)
	theirs_pairs := baro_lcs_pairs(base, theirs)

	mine_map := map[int]int{}
	for _, pair := range mine_pairs {
		mine_map[pair[0]] = pair[1]
	}

	theirs_map := map[int]int{}
	for _, pair := range theirs_pairs {
		theirs_map[pair[0]] = pair[1]
	}

	// 양쪽 모두에서 그대로 남은 기준 줄만 닻으로 쓴다
	anchors := [][3]int{}
	last_mine, last_theirs := -1, -1
	for base_index := 0; base_index < len(base); base_index++ {
		mine_index, ok_mine := mine_map[base_index]
		theirs_index, ok_theirs := theirs_map[base_index]
		if !ok_mine || !ok_theirs {
			continue
		}

		if mine_index <= last_mine || theirs_index <= last_theirs {
			continue
		}

		anchors = append(anchors, [3]int{base_index, mine_index, theirs_index})
		last_mine = mine_index
		last_theirs = theirs_index
	}

	out := []string{}
	conflict_count := 0
	merged_count := 0

	base_at, mine_at, theirs_at := 0, 0, 0

	handle := func(base_end int, mine_end int, theirs_end int) {
		base_part := base[base_at:base_end]
		mine_part := mine[mine_at:mine_end]
		theirs_part := theirs[theirs_at:theirs_end]

		same_mine := baro_same_lines(base_part, mine_part)
		same_theirs := baro_same_lines(base_part, theirs_part)

		switch {
		case same_mine && same_theirs:
			out = append(out, base_part...)
		case same_mine:
			out = append(out, theirs_part...)
			if len(theirs_part) != len(base_part) || !same_theirs {
				merged_count++
			}
		case same_theirs:
			out = append(out, mine_part...)
			merged_count++
		case baro_same_lines(mine_part, theirs_part):
			out = append(out, mine_part...)
		default:
			conflict_count++
			out = append(out, Baro_mark_mine)
			out = append(out, mine_part...)
			out = append(out, Baro_mark_split)
			out = append(out, theirs_part...)
			out = append(out, Baro_mark_theirs)
		}
	}

	for _, anchor := range anchors {
		handle(anchor[0], anchor[1], anchor[2])

		out = append(out, base[anchor[0]])
		base_at = anchor[0] + 1
		mine_at = anchor[1] + 1
		theirs_at = anchor[2] + 1
	}

	handle(len(base), len(mine), len(theirs))

	return Baro_join_line(out), conflict_count, merged_count
}

// baro_same_lines : 두 줄 목록이 같은지
func baro_same_lines(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

// Baro_has_conflict_mark : 본문에 충돌 표시가 남아 있는지
func Baro_has_conflict_mark(text string) bool {
	return strings.Contains(text, Baro_mark_mine) ||
		strings.Contains(text, Baro_mark_split) ||
		strings.Contains(text, Baro_mark_theirs)
}

// Baro_pick_side : 충돌 덩어리에서 한쪽만 골라 남긴다 (side : mine / theirs)
func Baro_pick_side(text string, side string) string {
	lines := Baro_split_line(text)
	out := []string{}

	mode := ""
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t")

		switch trimmed {
		case Baro_mark_mine:
			mode = "mine"
			continue
		case Baro_mark_split:
			if mode == "mine" {
				mode = "theirs"
				continue
			}
		case Baro_mark_theirs:
			mode = ""
			continue
		}

		if mode == "" || mode == side {
			out = append(out, line)
		}
	}

	return Baro_join_line(out)
}

// Baro_revision_text : 특정 판의 본문 (없으면 "")
func Baro_revision_text(db *sql.DB, doc_name string, rev string) string {
	data := ""
	QueryRow_DB(
		db,
		"select data from history where title = ? and id = ?",
		[]any{&data},
		doc_name, rev,
	)

	return data
}
