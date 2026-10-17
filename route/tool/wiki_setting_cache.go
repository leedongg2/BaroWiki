package tool

import (
	"database/sql"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var wiki_setting_generation atomic.Uint64
var wiki_setting_lock sync.RWMutex
var wiki_settings struct {
	db         *sql.DB
	generation uint64
	expires    time.Time
	values     map[string]string
}

const cached_wiki_settings = "'language','skin','markup','link_case_insensitive','name','license','logo','head','top_menu','template_var_1','template_var_2','template_var_3','namumark_compatible'"

func cacheable_wiki_setting(name string) bool {
	switch name {
	case "language", "skin", "markup", "link_case_insensitive", "name", "license", "logo", "head", "top_menu", "template_var_1", "template_var_2", "template_var_3", "namumark_compatible":
		return true
	}
	return strings.HasPrefix(name, "main_css_")
}

// 화면 공통 설정만 재사용한다. 계정·권한·차단·비밀번호는 캐시하지 않는다.
// 앱에서 설정을 쓰면 즉시 무효화하고, 외부 DB 변경도 1초 안에 다시 읽는다.
func Wiki_setting(db *sql.DB, name string, coverage ...string) string {
	if !cacheable_wiki_setting(name) {
		value := ""
		query := "select data from other where name = ?"
		args := []any{name}
		if len(coverage) > 0 {
			query += " and coverage = ?"
			args = append(args, coverage[0])
		}
		QueryRow_DB(db, query, []any{&value}, args...)
		return value
	}
	key := name
	if len(coverage) > 0 {
		key += "\x00" + coverage[0]
	}
	now := time.Now()
	generation := wiki_setting_generation.Load()
	wiki_setting_lock.RLock()
	if wiki_settings.db == db && wiki_settings.generation == generation && now.Before(wiki_settings.expires) {
		value := wiki_settings.values[key]
		wiki_setting_lock.RUnlock()
		return value
	}
	wiki_setting_lock.RUnlock()
	wiki_setting_lock.Lock()
	defer wiki_setting_lock.Unlock()
	generation = wiki_setting_generation.Load()
	if wiki_settings.db == db && wiki_settings.generation == generation && now.Before(wiki_settings.expires) {
		return wiki_settings.values[key]
	}
	rows := Query_DB(db, "select name, data, coverage from other where name in ("+cached_wiki_settings+") or name like 'main_css_%'")
	defer rows.Close()
	values := map[string]string{}
	for rows.Next() {
		var name, value, scope string
		if err := rows.Scan(&name, &value, &scope); err != nil {
			panic(err)
		}
		if !cacheable_wiki_setting(name) {
			continue
		}
		if _, exists := values[name]; !exists {
			values[name] = value
		}
		scoped := name + "\x00" + scope
		if _, exists := values[scoped]; !exists {
			values[scoped] = value
		}
	}
	if err := rows.Err(); err != nil {
		panic(err)
	}
	wiki_settings.db, wiki_settings.generation = db, generation
	wiki_settings.expires, wiki_settings.values = time.Now().Add(time.Second), values
	return values[key]
}

func invalidate_wiki_settings(query string) {
	if strings.Contains(strings.ToLower(query), "other") {
		wiki_setting_generation.Add(1)
	}
}
