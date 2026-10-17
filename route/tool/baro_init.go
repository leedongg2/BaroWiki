package tool

import (
	"database/sql"
	"strings"
)

// ============================================================
//  바로위키 초기 데이터
// ============================================================

// Baro_badge_default : 기본 배지 (7-6 — 의미 있는 것만, 출석판처럼 만들지 않는다)
func Baro_badge_default() [][]string {
	return [][]string{
		{"founder", "창립 기여자", "창립 시즌에 첫 유효 기여를 완료했습니다.", "founding", "★", "founder", "1"},
		{"first_doc", "첫 문서", "문서를 처음으로 작성했습니다.", "contribution", "1", "new_document:1", "2"},
		{"first_source", "첫 출처", "출처를 처음으로 추가했습니다.", "quality", "※", "source:1", "3"},
		{"doc_10", "문서 10개", "문서 10개를 작성했습니다.", "contribution", "X", "new_document:10", "4"},
		{"doc_100", "문서 100개", "문서 100개를 작성했습니다.", "contribution", "C", "new_document:100", "5"},
		{"source_100", "출처 100개", "유효한 출처 100개를 추가했습니다.", "quality", "C", "source:100", "6"},
		{"project_done", "프로젝트 완주", "프로젝트를 끝까지 완주했습니다.", "project", "▣", "project_finish:1", "7"},
		{"restore", "문서 복구", "격리·삭제된 문서를 되살렸습니다.", "care", "↺", "restore:1", "8"},
		{"reviewer", "우수 검토자", "검토 50건을 처리했습니다.", "quality", "✓", "review:50", "9"},
		{"year_1", "1년 활동", "1년 동안 기여를 이어갔습니다.", "time", "◷", "active_days:365", "10"},
		{"award_candidate", "올해의 위키인 후보", "올해의 위키인 후보로 등록됐습니다.", "award", "♛", "award_nominee", "11"},
	}
}

// Baro_policy_default : 기본 정책 문서 (0장 고정 원칙)
func Baro_policy_default() [][]string {
	return [][]string{
		{
			"principle",
			"바로위키 고정 원칙",
			"1. 이용자 제재와 콘텐츠 판단을 분리합니다.\n" +
				"   이용자가 차단됐다는 이유만으로 정상적인 기여를 지우지 않습니다.\n" +
				"   다만 저작권 침해, 개인정보 침해, 허위정보 등 콘텐츠 자체에 문제가 있으면 별도로 처리합니다.\n\n" +
				"2. 삭제보다 보존과 복구 가능성을 우선합니다.\n" +
				"   신고 → 임시 격리 → 검토 → 수정·복구·삭제 순서로 처리합니다.\n\n" +
				"3. 대량 파괴 작업에는 안전장치를 둡니다.\n" +
				"   영향 문서 수 표시, 대상 목록 공개, 2인 승인, 복구 스냅샷, 유예 시간을 반드시 거칩니다.\n" +
				"   긴급 법적 사안만 예외입니다.\n\n" +
				"4. 기여 횟수보다 기여 가치를 인정합니다.\n" +
				"   오타 한 글자 수정 1,000번과 좋은 문서 100개 작성은 같지 않습니다.\n\n" +
				"5. 운영 근거는 공개하되 개인정보는 보호합니다.\n" +
				"   조치 사유와 적용 규정은 공개하지만 IP·이메일·민감한 신고 내용은 공개하지 않습니다.",
		},
		{
			"sanction",
			"제재 규정",
			"제재는 경고 → 편집 속도 제한 → 특정 문서·분야 제한 → 토론 제한 → 업로드 제한 →\n" +
				"단기 차단 → 장기 차단 → 영구 차단 순으로 단계적으로 적용합니다.\n" +
				"무조건 영구 차단부터 적용하지 않습니다.\n\n" +
				"제재는 사람에게만 적용되며, 제재를 이유로 문서를 삭제하지 않습니다.\n" +
				"모든 제재에는 사건 번호가 붙고, 이의제기를 신청할 수 있습니다.\n" +
				"이의제기는 원조치 담당자가 아닌 다른 담당자가 검토합니다.",
		},
		{
			"quarantine",
			"문서 격리 규정",
			"문제가 제기된 문서는 즉시 삭제하지 않고 임시 격리합니다.\n" +
				"격리된 문서는 일반 이용자에게 보이지 않지만 내용은 그대로 보존되며,\n" +
				"검토 후 수정·복구·삭제 중 하나로 처리합니다.\n\n" +
				"저작권 침해와 개인정보 노출은 긴급 비공개 대상이며, 즉시 격리 후 검토합니다.",
		},
		{
			"mass",
			"대량 작업 규정",
			"10개 이상의 문서나 이용자에게 영향을 주는 작업은 대량 작업으로 분류합니다.\n\n" +
				"대량 작업은 다음 절차를 반드시 거칩니다.\n" +
				"  - 영향받는 대상 수를 미리 표시\n" +
				"  - 처리 대상 목록 공개 및 내려받기 제공\n" +
				"  - 서로 다른 운영자 2인의 승인\n" +
				"  - 복구 가능한 스냅샷 생성\n" +
				"  - 기본 24시간의 유예\n" +
				"  - 실행 후 자동 공지와 원클릭 일괄 복구\n\n" +
				"긴급 법적 사안만 유예 없이 실행할 수 있으며, 이 경우 사유를 반드시 기록합니다.",
		},
		{
			"license",
			"기여 라이선스",
			"바로위키에 게시한 기여는 CC BY-SA 4.0 으로 공개됩니다.\n\n" +
				"  - 다른 사람이 내 기여를 수정하고 재사용할 수 있습니다.\n" +
				"  - 재사용할 때는 출처를 표기해야 합니다.\n" +
				"  - 계정을 탈퇴해도 이미 공개된 정상 기여는 자동으로 삭제되지 않습니다.\n" +
				"  - 저작권을 침해하는 내용은 게시할 수 없습니다.",
		},
		{
			"privacy",
			"개인정보 처리 방침",
			"바로위키는 필요한 정보만 수집합니다.\n\n" +
				"  - 이메일은 비공개이며 계정 확인과 알림에만 사용합니다.\n" +
				"  - 실명, 전화번호, 생년월일을 요구하지 않습니다.\n" +
				"  - 공개 프로필 항목은 이용자가 직접 고를 수 있습니다.\n" +
				"  - 접속 기록 등 운영 데이터는 90일간 보관 후 삭제합니다.\n" +
				"  - 열람·수정·내려받기·삭제는 개인정보 요청 센터에서 신청할 수 있습니다.",
		},
	}
}

// Baro_default_set : 기본 설정
func baro_default_set() [][]string {
	return [][]string{
		{"wiki_name", "바로위키"},
		{"founding_goal", "100"},
		{"founding_open", "on"},
		{"mass_threshold", "10"},
		{"mass_grace_hours", "24"},
		{"score_confirm_days", "7"},
		{"review_risk_new_user_days", "30"},
		{"review_big_edit_length", "3000"},
		{"invite_limit", "10"},
		{"appeal_deadline_days", "14"},
		{"report_deadline_days", "7"},
		{"log_retention_days", "90"},
		{"license", "CC BY-SA 4.0"},
		{"api_rate_per_hour", "600"},
		{"read_only", ""},
		{"mik_attend", "5"},
		{"mik_badge", "20"},
		{"mik_send_daily", "100"},
		{"captcha_mode", "ip"},
	}
}

// Baro_init : 바로위키 초기 데이터 (여러 번 실행해도 안전)
func Baro_init(db *sql.DB) {
	// 기본 설정
	for _, row := range baro_default_set() {
		found := ""
		if QueryRow_DB(db, "select name from baro_set where name = ?", []any{&found}, row[0]) {
			continue
		}

		Exec_DB(db, "insert into baro_set (name, data) values (?, ?)", row[0], row[1])
	}

	// 분야
	for _, row := range Baro_field_default() {
		found := ""
		if QueryRow_DB(db, "select key_name from baro_field where key_name = ?", []any{&found}, row.Key) {
			continue
		}

		Baro_field_save(db, row)
	}

	// 배지
	for _, row := range Baro_badge_default() {
		found := ""
		if QueryRow_DB(db, "select key_name from baro_badge where key_name = ?", []any{&found}, row[0]) {
			continue
		}

		Exec_DB(
			db,
			"insert into baro_badge (key_name, name, description, kind, icon, criteria, sort) values (?, ?, ?, ?, ?, ?, ?)",
			row[0], row[1], row[2], row[3], row[4], row[5], row[6],
		)
	}

	// 정책 문서 (1판)
	for _, row := range Baro_policy_default() {
		found := ""
		if QueryRow_DB(db, "select key_name from baro_policy where key_name = ?", []any{&found}, row[0]) {
			continue
		}

		Exec_DB(
			db,
			"insert into baro_policy (id, key_name, title, version, data, effective_date, proposer, reason, discussion, date, status) values (?, ?, ?, '1', ?, ?, 'system', '최초 제정', '', ?, 'active')",
			Baro_next_id(db, "baro_policy"), row[0], row[1], row[2], Baro_today(), Baro_now(),
		)
	}

	// 편집 필터 기본 규칙 (9-6)
	filter_found := ""
	if !QueryRow_DB(db, "select id from baro_edit_filter limit 1", []any{&filter_found}) {
		for _, row := range [][]string{
			{"전화번호 노출", `0\d{1,2}[-. ]?\d{3,4}[-. ]?\d{4}`, "content", "review", "개인정보 노출 가능성"},
			{"이메일 노출", `[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`, "content", "review", "개인정보 노출 가능성"},
			{"주민등록번호 형식", `\d{6}[-]\d{7}`, "content", "block", "주민등록번호는 게시할 수 없습니다"},
			{"반복 링크 도배", `(https?://\S+\s*){15,}`, "content", "review", "광고 도배 의심"},
			{"대량 삭제", `^$`, "empty", "review", "본문 전체 삭제"},
		} {
			Exec_DB(
				db,
				"insert into baro_edit_filter (id, name, pattern, kind, action, status, note, date, hits) values (?, ?, ?, ?, ?, 'on', ?, ?, '0')",
				Baro_next_id(db, "baro_edit_filter"), row[0], row[1], row[2], row[3], row[4], Baro_now(),
			)
		}
	}

	// 상단 메뉴에 바로위키 넣기 (한 번만 — 사용자가 지우면 다시 넣지 않는다)
	menu_added := ""
	if !QueryRow_DB(db, "select name from baro_set where name = 'top_menu_added'", []any{&menu_added}) {
		top_menu := ""
		QueryRow_DB(db, `select data from other where name = "top_menu"`, []any{&top_menu})

		if !strings.Contains(top_menu, "/baro") {
			new_menu := "바로위키\n/baro"
			if strings.TrimSpace(top_menu) != "" {
				new_menu = strings.TrimRight(strings.ReplaceAll(top_menu, "\r", ""), "\n") + "\n" + new_menu
			}

			Exec_DB(db, `delete from other where name = "top_menu"`)
			Exec_DB(db, `insert into other (name, data, coverage) values ("top_menu", ?, "")`, new_menu)
		}

		Baro_put(db, "top_menu_added", "on")
	}

	// 기본 문서 (틀 · 도움말)
	baro_seed_documents(db)

	// 위키 수상식 안내 · 분류 · 대문 안내
	Baro_seed_ceremony(db)

	// 바로샵 기본 상품 (칭호 · 캡차 면제권)
	Baro_seed_shop(db)

	// 밐 순위 누계 채우기 (칸이 생기기 전에 쌓인 밐도 순위에 들어오게)
	Baro_mik_backfill(db)

	// 기능 플래그
	for _, row := range [][]string{
		{"realtime_sync", "실시간 동기화", "off", "필드 병합이 준비되기 전까지 꺼 둡니다"},
		{"knowledge_graph", "지식 그래프", "admin", "관계 시각화 (준비 중)"},
		{"public_api", "공개 읽기 API", "all", "외부 서비스 연동"},
		{"stable_version", "안정판", "all", "논쟁 문서의 검토 완료판 표시"},
		{"mentoring", "멘토링", "all", "신규 기여자와 숙련 기여자 연결"},
	} {
		found := ""
		if QueryRow_DB(db, "select key_name from baro_flag where key_name = ?", []any{&found}, row[0]) {
			continue
		}

		Exec_DB(
			db,
			"insert into baro_flag (key_name, name, scope, value, note, date) values (?, ?, ?, '', ?, ?)",
			row[0], row[1], row[2], row[3], Baro_now(),
		)
	}
}
