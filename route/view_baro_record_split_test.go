package route

import (
	"strings"
	"testing"
)

// ============================================================
//  기여 기록 갈래 나누기 시험
// ============================================================

// TestBaroRecordEditKind : 판 하나가 어떤 일이었는지 가른다
func TestBaroRecordEditKind(t *testing.T) {
	table := []struct {
		id, kind, prev, want string
	}{
		{"1", "r1", "", "edit_new"},
		{"1", "", "", "edit_new"},
		{"2", "", "", "edit_normal"},
		{"3", "delete", "", "edit_delete"},
		{"4", "", "delete", "edit_restore"},
		{"5", "move", "", "edit_move"},
		{"6", "move_all", "", "edit_move"},
		{"7", "revert", "", "edit_revert"},

		// 삭제·이동·되돌림은 앞 판이 삭제였더라도 그 자신의 갈래를 지킨다.
		{"8", "delete", "delete", "edit_delete"},
		{"9", "move", "delete", "edit_move"},
		{"10", "revert", "delete", "edit_revert"},

		// 문서 종류가 type 에 들어오는 경우 (파일 문서 등) 도 보통 편집으로 본다.
		{"11", "file", "", "edit_normal"},
	}

	for _, one := range table {
		got := baro_record_edit_kind(one.id, one.kind, one.prev)
		if got != one.want {
			t.Fatalf("id=%s type=%q prev=%q → %s (기대 %s)", one.id, one.kind, one.prev, got, one.want)
		}
	}
}

// TestBaroRecordKindList : 갈래 목록이 성하다
func TestBaroRecordKindList(t *testing.T) {
	seen := map[string]bool{}

	for _, one := range baro_record_kind_all() {
		if one.Key == "" || one.Name == "" {
			t.Fatalf("비어 있는 갈래가 있다: %+v", one)
		}
		if seen[one.Key] {
			t.Fatalf("갈래 열쇠가 겹친다: %s", one.Key)
		}

		// 잔 갈래의 부모는 반드시 큰 갈래여야 한다.
		if one.Parent != "" {
			parent, ok := baro_record_kind_get(one.Parent)
			if !ok || parent.Parent != "" {
				t.Fatalf("%s 의 부모 %q 가 큰 갈래가 아니다", one.Key, one.Parent)
			}
		}

		seen[one.Key] = true
	}

	// 길 등록 쪽에서 쓰는 확인 함수와 목록이 어긋나지 않아야 한다.
	for key := range seen {
		if !Baro_record_kind_ok(key) {
			t.Fatalf("목록에 있는데 아는 갈래가 아니라고 한다: %s", key)
		}
	}
	if Baro_record_kind_ok("그런거없음") {
		t.Fatal("없는 갈래를 안다고 한다")
	}

	// 사용자가 요청한 갈래가 모두 있어야 한다.
	for _, want := range []string{
		"all", "edit_all", "edit_new", "edit_delete", "edit_restore", "edit_move", "edit_revert",
		"req_make", "req_handle", "bbs_all", "bbs_post", "bbs_comment",
	} {
		if !seen[want] {
			t.Fatalf("있어야 할 갈래가 없다: %s", want)
		}
	}
}

// TestBaroRecordTop : 잔 갈래가 어느 큰 갈래에 딸리는가
func TestBaroRecordTop(t *testing.T) {
	table := map[string]string{
		"all":         "all",
		"edit":        "edit",
		"edit_new":    "edit",
		"edit_revert": "edit",
		"req_handle":  "req",
		"bbs_comment": "bbs",
		"topic":       "topic",
		"그런거없음":       "all",
	}

	for key, want := range table {
		if got := baro_record_top(key); got != want {
			t.Fatalf("%s → %s (기대 %s)", key, got, want)
		}
	}
}

// TestBaroRecordSendEsc : 이동 기록의 껍데기 태그가 글자로 새지 않는다
func TestBaroRecordSendEsc(t *testing.T) {
	got := baro_record_send_esc("이름 바꿈 (<a>옛 이름</a> → <a>새 이름</a>)")

	for _, bad := range []string{"<a>", "&lt;a&gt;", "</a>"} {
		if strings.Contains(got, bad) {
			t.Fatalf("껍데기가 남았다 (%s): %s", bad, got)
		}
	}

	if !strings.Contains(got, "옛 이름") || !strings.Contains(got, "새 이름") {
		t.Fatalf("이름이 사라졌다: %s", got)
	}

	// 진짜 사용자 입력은 여전히 escape 되어야 한다.
	danger := baro_record_send_esc(`<img src=x onerror=alert(1)>`)
	if strings.Contains(danger, "<img") {
		t.Fatalf("태그가 그대로 나갔다: %s", danger)
	}
}
