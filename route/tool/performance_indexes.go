package tool

import "database/sql"

// 기존 컬럼만 사용한다. SQLite의 숫자 판 정렬도 인덱스로 처리한다.
func create_performance_indexes(db *sql.DB) {
	queries := []string{
		"create index if not exists other_name_coverage_idx on other (name, coverage)",
		"create index if not exists acl_title_type_idx on acl (title, type)",
		"create index if not exists alist_name_idx on alist (name)",
		"create index if not exists rc_type_date_idx on rc (type, date)",
		"create index if not exists history_date_idx on history (date)",
		"create index if not exists history_id_date_title_idx on history (id, date, title)",
		"create index if not exists history_title_numeric_idx on history (title, (id + 0))",
		"create index if not exists data_title_nocase_idx on data (title collate nocase)",
		"create index if not exists html_filter_kind_plus_idx on html_filter (kind, plus)",
	}
	if Get_DB_type() == "mysql" {
		queries = []string{
			"create index other_name_coverage_idx on other (name(191), coverage(191))",
			"create index acl_title_type_idx on acl (title(191), type(191))",
			"create index alist_name_idx on alist (name(191))",
			"create index rc_type_date_idx on rc (type(191), date(191))",
			"create index history_date_idx on history (date(191))",
			"create index history_id_date_title_idx on history (id(191), date(191), title(191))",
			"create index html_filter_kind_plus_idx on html_filter (kind(191), plus(191))",
		}
	}
	for _, query := range queries {
		_, _ = db.Exec(query)
	}
}
