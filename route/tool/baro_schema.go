package tool

import (
	"database/sql"
	"fmt"
	"strings"
)

// ============================================================
//  바로위키 확장 스키마
//
//  openNAMU 본체 테이블은 건드리지 않고 baro_ 접두사 테이블만 추가한다.
//  기존 openNAMU 관례를 따라 모든 컬럼은 text 이며,
//  DB_make 가 test 컬럼을 먼저 만든 뒤 나머지 컬럼을 alter 로 붙인다.
// ============================================================

// Baro_table_list : 바로위키가 사용하는 확장 테이블 정의
func Baro_table_list() map[string][]string {
	t := map[string][]string{}

	// --- 설정 / 운영 기반 ---
	t["baro_set"] = []string{"name", "data"}
	t["baro_flag"] = []string{"key_name", "name", "scope", "value", "note", "date"}
	t["baro_metric"] = []string{"id", "key_name", "period", "value", "detail", "date"}
	t["baro_rate"] = []string{"key_name", "who", "num", "window_start"}
	t["baro_incident"] = []string{"id", "title", "status", "severity", "detail", "date", "resolve_date", "postmortem", "actor"}

	// --- 사용자 / 창립 시즌 ---
	t["baro_user_set"] = []string{"user", "name", "data"}
	t["baro_role"] = []string{"user", "role", "granted_by", "date", "memo"}
	t["baro_interest"] = []string{"user", "field", "kind", "date"}
	t["baro_expertise"] = []string{"user", "field", "score", "is_main", "date"}
	t["baro_founder"] = []string{"user", "number", "date", "status", "first_event", "field", "apply_date", "note"}
	t["baro_invite"] = []string{"id", "code", "owner", "target", "date", "used_date", "status"}
	t["baro_consent"] = []string{"id", "user", "kind", "version", "date", "ip"}
	t["baro_onboard"] = []string{"user", "step", "status", "date"}
	t["baro_2fa"] = []string{"user", "secret", "backup", "status", "date"}
	t["baro_login_log"] = []string{"id", "user", "ip", "ua", "date", "kind", "detail"}
	t["baro_share_card"] = []string{"id", "user", "kind", "title", "data", "date", "views"}

	// --- 문서 부가 정보 ---
	t["baro_doc_status"] = []string{"title", "status", "reason", "actor", "date"}
	t["baro_doc_alias"] = []string{"id", "title", "alias", "kind", "note", "date"}
	t["baro_doc_meta"] = []string{"title", "name", "data"}
	t["baro_citation"] = []string{"id", "title", "rev", "kind", "url", "author", "publisher", "pub_date", "access_date", "archive_url", "quality", "status", "date"}
	t["baro_quarantine"] = []string{"id", "title", "rev", "kind", "reason", "actor", "date", "status", "backup", "release_date", "case_id"}
	t["baro_stable"] = []string{"title", "rev", "actor", "date", "note"}
	t["baro_review"] = []string{"id", "title", "rev", "user", "kind", "status", "reviewer", "review_date", "comment", "risk", "date"}
	t["baro_dup"] = []string{"id", "title", "target", "kind", "ratio", "date", "status", "detail"}
	t["baro_file_meta"] = []string{"title", "source", "author", "photographer", "license", "basis", "shot_date", "description", "related", "hash", "exif_removed", "date", "uploader"}

	// --- 프로젝트 / 분야 개척 ---
	t["baro_field"] = []string{"key_name", "name", "description", "sort", "target_count", "recruit_count", "recruit_open", "kind", "date"}
	t["baro_project"] = []string{"id", "name", "field", "goal", "scope", "dataset", "start_date", "end_date", "owner", "status", "target_count", "done_count", "rule", "finish_cond", "date", "summary", "report"}
	t["baro_project_member"] = []string{"id", "project", "user", "role", "date", "status"}
	t["baro_project_target"] = []string{"id", "project", "name", "note", "status", "doc", "date", "assignee"}
	t["baro_project_task"] = []string{"id", "project", "kind", "title", "detail", "difficulty", "est_time", "assignee", "status", "doc", "date", "done_date", "requester", "field"}
	t["baro_project_progress"] = []string{"id", "project", "date", "done", "total", "note"}
	t["baro_project_source"] = []string{"id", "project", "title", "url", "note", "user", "date"}
	t["baro_workspace"] = []string{"id", "project", "title", "data", "user", "date", "status", "note"}

	// --- 기여 인정 ---
	t["baro_contrib_event"] = []string{"id", "user", "kind", "title", "rev", "project", "weight", "detail", "date", "status", "field"}
	t["baro_contrib_score"] = []string{"id", "user", "event", "base", "source", "novelty", "review", "survive", "penalty", "final", "confirmed", "confirm_date", "date"}
	t["baro_impact"] = []string{"user", "name", "data", "date"}
	t["baro_badge"] = []string{"key_name", "name", "description", "kind", "icon", "criteria", "sort"}
	t["baro_user_badge"] = []string{"id", "user", "badge", "date", "detail"}
	t["baro_rank"] = []string{"id", "period", "kind", "field", "user", "score", "rank_no", "date"}
	t["baro_thanks"] = []string{"id", "from_user", "to_user", "title", "rev", "reason", "is_public", "date"}
	t["baro_calendar"] = []string{"user", "date", "num", "detail"}

	// --- 커뮤니티 ---
	t["baro_watch"] = []string{"id", "user", "kind", "target", "date", "mode"}
	t["baro_notify"] = []string{"id", "user", "kind", "title", "url", "data", "readme", "date"}
	t["baro_notify_set"] = []string{"user", "kind", "mode"}
	t["baro_mentor"] = []string{"id", "mentor", "mentee", "status", "date", "field", "note"}

	// --- 신고 / 제재 / 이의제기 ---
	t["baro_report"] = []string{"id", "kind", "target_kind", "target", "reporter", "detail", "evidence", "status", "case_id", "date", "priority", "note"}
	t["baro_case"] = []string{"id", "number", "kind", "target_kind", "target", "rule", "actor", "status", "action", "public_summary", "private_note", "date", "close_date", "deadline", "appeal_status", "priority"}
	t["baro_case_action"] = []string{"id", "case_id", "actor", "action", "detail", "date", "target"}
	t["baro_case_evidence"] = []string{"id", "case_id", "kind", "data", "actor", "date", "is_private"}
	t["baro_sanction"] = []string{"id", "user", "kind", "scope", "reason", "case_id", "actor", "start_date", "end_date", "status", "date"}
	t["baro_appeal"] = []string{"id", "case_id", "user", "reason", "extra", "status", "reviewer", "result", "result_reason", "date", "deadline", "close_date"}
	t["baro_mass_op"] = []string{"id", "kind", "actor", "target_list", "num", "reason", "status", "approver", "created", "approved_date", "execute_date", "grace_until", "urgent", "undo_status", "detail"}
	t["baro_snapshot"] = []string{"id", "mass_id", "kind", "title", "data", "meta", "date"}

	// --- 거버넌스 / 투명성 ---
	t["baro_public_log"] = []string{"id", "kind", "actor", "target", "reason", "detail", "date"}
	t["baro_audit_log"] = []string{"id", "actor", "action", "target", "before_data", "after_data", "auth", "reason", "ip", "ua", "date", "sign_data"}
	t["baro_policy"] = []string{"id", "key_name", "title", "version", "data", "effective_date", "proposer", "reason", "discussion", "date", "status"}
	t["baro_coi"] = []string{"id", "case_id", "actor", "reason", "date"}
	t["baro_admin_stat"] = []string{"user", "name", "data", "period", "date"}
	t["baro_transparency"] = []string{"id", "period", "data", "date", "publisher"}

	// --- 어워드 ---
	t["baro_award"] = []string{"id", "season", "category", "name", "description", "status", "date", "result_reason", "winner"}
	t["baro_nomination"] = []string{"id", "award", "target_kind", "target", "nominator", "kind", "score", "status", "date", "reason"}
	t["baro_award_vote"] = []string{"id", "award", "nomination", "voter", "weight", "date"}
	t["baro_wrapped"] = []string{"id", "user", "year_data", "data", "date"}

	// --- 이전 / 데이터 개방 ---
	t["baro_import"] = []string{"id", "kind", "source", "file", "status", "actor", "date", "report", "num", "license", "options"}
	t["baro_import_item"] = []string{"id", "import_id", "title", "status", "note", "orig_author", "orig_url", "license", "date"}
	t["baro_url_map"] = []string{"id", "old_url", "new_url", "kind", "date"}
	t["baro_export"] = []string{"id", "user", "kind", "scope", "status", "path", "date", "expire"}
	t["baro_backup"] = []string{"id", "kind", "path", "size", "status", "date", "note", "checksum"}
	t["baro_api_key"] = []string{"id", "user", "key_hash", "name", "scope", "rate", "status", "date", "last_used"}

	// --- 개인정보 / 저작권 / 법무 ---
	t["baro_privacy_req"] = []string{"id", "user", "kind", "detail", "status", "handler", "date", "close_date", "verify", "result"}
	t["baro_copyright_req"] = []string{"id", "claimant", "contact", "target_kind", "target", "proof", "status", "handler", "date", "close_date", "result", "counter"}
	t["baro_legal_log"] = []string{"id", "kind", "num", "period", "date", "note"}
	t["baro_deletion_req"] = []string{"id", "user", "kind", "reason", "status", "date", "close_date", "handler", "result"}

	// --- 편집 필터 (바로위키 확장) ---
	t["baro_edit_filter"] = []string{"id", "name", "pattern", "kind", "action", "status", "note", "date", "hits"}
	t["baro_filter_log"] = []string{"id", "filter_id", "user", "title", "action", "detail", "date"}

	// --- 편집 요청 ---
	//   baro_edit_req      : 요청 자체 (생성 기록의 원본)
	//   baro_edit_req_log  : 만들고 · 승인하고 · 거절하고 · 지운 기록
	t["baro_edit_req"] = []string{"id", "title", "base_rev", "user", "data", "summary", "status", "handler", "handle_date", "reason", "date", "new_rev"}
	t["baro_edit_req_log"] = []string{"id", "req_id", "title", "actor", "action", "detail", "date"}

	// --- 위키 수상식 ---
	//   baro_ceremony      : 수상식 한 번 (운영자만 개최)
	//   baro_ceremony_cat  : 부문
	//   baro_ceremony_run  : 출마 · 추천된 후보
	//   baro_ceremony_vote : 투표 (가입한 사용자만)
	t["baro_ceremony"] = []string{"id", "name", "doc", "opener", "status", "run_end", "vote_end", "note", "date", "close_date", "result"}
	t["baro_ceremony_cat"] = []string{"id", "ceremony", "name", "description", "sort", "winner"}
	t["baro_ceremony_run"] = []string{"id", "ceremony", "cat", "target_kind", "target", "runner", "reason", "status", "date"}
	t["baro_ceremony_vote"] = []string{"id", "ceremony", "cat", "run_id", "voter", "date"}

	return t
}

// baro_index_list : (인덱스 이름, 테이블, 컬럼...) 목록
func baro_index_list() [][]string {
	return [][]string{
		{"baro_set_name_idx", "baro_set", "name"},
		{"baro_user_set_idx", "baro_user_set", "user", "name"},
		{"baro_role_idx", "baro_role", "user", "role"},
		{"baro_interest_idx", "baro_interest", "user", "field"},
		{"baro_expertise_idx", "baro_expertise", "user", "field"},
		{"baro_founder_user_idx", "baro_founder", "user"},
		{"baro_founder_num_idx", "baro_founder", "number"},
		{"baro_invite_code_idx", "baro_invite", "code"},
		{"baro_invite_owner_idx", "baro_invite", "owner"},
		{"baro_consent_idx", "baro_consent", "user", "kind"},
		{"baro_onboard_idx", "baro_onboard", "user", "step"},
		{"baro_login_log_idx", "baro_login_log", "user", "date"},
		{"baro_doc_status_idx", "baro_doc_status", "title"},
		{"baro_doc_alias_title_idx", "baro_doc_alias", "title"},
		{"baro_doc_alias_alias_idx", "baro_doc_alias", "alias"},
		{"baro_doc_meta_idx", "baro_doc_meta", "title", "name"},
		{"baro_citation_title_idx", "baro_citation", "title"},
		{"baro_citation_status_idx", "baro_citation", "status"},
		{"baro_quarantine_title_idx", "baro_quarantine", "title", "status"},
		{"baro_stable_idx", "baro_stable", "title"},
		{"baro_review_status_idx", "baro_review", "status", "date"},
		{"baro_review_title_idx", "baro_review", "title"},
		{"baro_dup_title_idx", "baro_dup", "title"},
		{"baro_file_meta_idx", "baro_file_meta", "title"},
		{"baro_field_key_idx", "baro_field", "key_name"},
		{"baro_project_field_idx", "baro_project", "field", "status"},
		{"baro_project_member_idx", "baro_project_member", "project", "user"},
		{"baro_project_target_idx", "baro_project_target", "project", "status"},
		{"baro_project_task_idx", "baro_project_task", "project", "status"},
		{"baro_project_task_field_idx", "baro_project_task", "field", "status"},
		{"baro_project_progress_idx", "baro_project_progress", "project", "date"},
		{"baro_project_source_idx", "baro_project_source", "project"},
		{"baro_workspace_idx", "baro_workspace", "project", "status"},
		{"baro_contrib_user_idx", "baro_contrib_event", "user", "date"},
		{"baro_contrib_title_idx", "baro_contrib_event", "title"},
		{"baro_contrib_kind_idx", "baro_contrib_event", "kind", "date"},
		{"baro_score_user_idx", "baro_contrib_score", "user", "confirmed"},
		{"baro_score_event_idx", "baro_contrib_score", "event"},
		{"baro_impact_idx", "baro_impact", "user", "name"},
		{"baro_user_badge_idx", "baro_user_badge", "user", "badge"},
		{"baro_rank_idx", "baro_rank", "period", "kind", "field"},
		{"baro_thanks_to_idx", "baro_thanks", "to_user"},
		{"baro_thanks_uniq_idx", "baro_thanks", "from_user", "title", "rev"},
		{"baro_calendar_idx", "baro_calendar", "user", "date"},
		{"baro_watch_idx", "baro_watch", "user", "kind", "target"},
		{"baro_watch_target_idx", "baro_watch", "kind", "target"},
		{"baro_notify_idx", "baro_notify", "user", "readme"},
		{"baro_notify_set_idx", "baro_notify_set", "user", "kind"},
		{"baro_mentor_idx", "baro_mentor", "mentor", "mentee"},
		{"baro_report_status_idx", "baro_report", "status", "date"},
		{"baro_report_target_idx", "baro_report", "target_kind", "target"},
		{"baro_report_reporter_idx", "baro_report", "reporter"},
		{"baro_case_status_idx", "baro_case", "status", "date"},
		{"baro_case_number_idx", "baro_case", "number"},
		{"baro_case_target_idx", "baro_case", "target_kind", "target"},
		{"baro_case_action_idx", "baro_case_action", "case_id"},
		{"baro_case_evidence_idx", "baro_case_evidence", "case_id"},
		{"baro_sanction_user_idx", "baro_sanction", "user", "status"},
		{"baro_appeal_case_idx", "baro_appeal", "case_id"},
		{"baro_appeal_status_idx", "baro_appeal", "status", "date"},
		{"baro_mass_status_idx", "baro_mass_op", "status", "created"},
		{"baro_snapshot_idx", "baro_snapshot", "mass_id"},
		{"baro_public_log_idx", "baro_public_log", "kind", "date"},
		{"baro_audit_log_idx", "baro_audit_log", "actor", "date"},
		{"baro_policy_idx", "baro_policy", "key_name", "version"},
		{"baro_coi_idx", "baro_coi", "case_id"},
		{"baro_admin_stat_idx", "baro_admin_stat", "user", "name"},
		{"baro_award_idx", "baro_award", "season", "category"},
		{"baro_nomination_idx", "baro_nomination", "award", "status"},
		{"baro_award_vote_idx", "baro_award_vote", "award", "voter"},
		{"baro_wrapped_idx", "baro_wrapped", "user", "year_data"},
		{"baro_import_idx", "baro_import", "status", "date"},
		{"baro_import_item_idx", "baro_import_item", "import_id", "status"},
		{"baro_url_map_idx", "baro_url_map", "old_url"},
		{"baro_export_idx", "baro_export", "user", "status"},
		{"baro_backup_idx", "baro_backup", "kind", "date"},
		{"baro_api_key_idx", "baro_api_key", "key_hash"},
		{"baro_privacy_req_idx", "baro_privacy_req", "user", "status"},
		{"baro_copyright_req_idx", "baro_copyright_req", "status", "date"},
		{"baro_deletion_req_idx", "baro_deletion_req", "user", "status"},
		{"baro_edit_filter_idx", "baro_edit_filter", "status"},
		{"baro_filter_log_idx", "baro_filter_log", "filter_id", "date"},
		{"baro_metric_idx", "baro_metric", "key_name", "period"},
		{"baro_rate_idx", "baro_rate", "key_name", "who"},
		{"baro_edit_req_status_idx", "baro_edit_req", "status", "date"},
		{"baro_edit_req_title_idx", "baro_edit_req", "title"},
		{"baro_edit_req_user_idx", "baro_edit_req", "user", "date"},
		{"baro_edit_req_log_req_idx", "baro_edit_req_log", "req_id"},
		{"baro_edit_req_log_actor_idx", "baro_edit_req_log", "actor", "date"},
		{"baro_ceremony_status_idx", "baro_ceremony", "status", "date"},
		{"baro_ceremony_cat_idx", "baro_ceremony_cat", "ceremony", "sort"},
		{"baro_ceremony_run_idx", "baro_ceremony_run", "ceremony", "cat"},
		{"baro_ceremony_run_target_idx", "baro_ceremony_run", "target_kind", "target"},
		{"baro_ceremony_vote_idx", "baro_ceremony_vote", "ceremony", "cat", "voter"},
	}
}

// Baro_create_index : 바로위키 테이블 인덱스 생성 (SQLite / MySQL 모두 지원)
func Baro_create_index(db *sql.DB) {
	is_mysql := Get_DB_type() == "mysql"

	for _, row := range baro_index_list() {
		if len(row) < 3 {
			continue
		}

		index_name := row[0]
		table_name := row[1]

		columns := []string{}
		for _, column_name := range row[2:] {
			if is_mysql {
				columns = append(columns, column_name+"(191)")
			} else {
				columns = append(columns, column_name)
			}
		}

		query := fmt.Sprintf(
			"create index %s on %s (%s)",
			index_name,
			table_name,
			strings.Join(columns, ", "),
		)

		if _, err := db.Exec(DB_change(query)); err != nil {
			continue
		}
	}
}
