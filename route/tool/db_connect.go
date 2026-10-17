package tool

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"
)

var db_set = map[string]string{}

var db_pool_lock sync.Mutex
var db_pool *sql.DB

// 각 요청은 같은 연결 풀을 빌린다. 열린 rows 안에서 다른 조회를 하는
// 기존 경로가 있으므로 열린 연결 수를 1개로 제한하면 교착된다.
func sqlite_dsn() string {
	name, err := filepath.Abs(db_set["db_name"] + ".db")
	if err != nil {
		panic(err)
	}
	name = filepath.ToSlash(name)
	if !strings.HasPrefix(name, "/") {
		name = "/" + name
	}
	params := url.Values{"_pragma": {"busy_timeout(5000)", "journal_mode(WAL)"}}
	return (&url.URL{Scheme: "file", Path: name, RawQuery: params.Encode()}).String()
}

func Get_DB_set() map[string]string {
	new_db_set := map[string]string{}

	db_env, db_env_exist := os.LookupEnv("NAMU_DB")
	db_env_type, db_env_type_exist := os.LookupEnv("NAMU_DB_TYPE")
	if db_env_exist || db_env_type_exist {
		new_db_set["db_name"] = Choose(db_env, "data")
		new_db_set["db_type"] = Choose(db_env_type, "sqlite")

		return new_db_set
	}

	path_dir := filepath.Join("data", "set.json")
	if File_exist_check(path_dir) {
		raw, err := os.ReadFile(path_dir)
		if err == nil {
			tmp := map[string]string{}
			if err := json.Unmarshal(raw, &tmp); err == nil {
				if v, ok := tmp["db"]; ok {
					new_db_set["db_name"] = v
				} else {
					new_db_set["db_name"] = "data"
				}

				if v, ok := tmp["db_type"]; ok {
					new_db_set["db_type"] = v
				} else {
					new_db_set["db_type"] = "sqlite"
				}

				return new_db_set
			}
		}
	}

	new_db_set["db_name"] = "data"
	new_db_set["db_type"] = "sqlite"

	return new_db_set
}

func Get_DB_set_MySQL(new_db_set map[string]string) map[string]string {
	path := filepath.Join("data", "mysql.json")
	if !File_exist_check(path) {
		return new_db_set
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return new_db_set
	}

	tmp := new_db_set
	if err := json.Unmarshal(raw, &tmp); err != nil {
		return tmp
	}

	if host, ok := tmp["host"]; ok && host != "" {
		tmp["db_mysql_host"] = host
	} else {
		tmp["db_mysql_host"] = "127.0.0.1"
	}

	if port, ok := tmp["port"]; ok && port != "" {
		tmp["db_mysql_port"] = port
	} else {
		tmp["db_mysql_port"] = "3306"
	}

	if user, ok := tmp["user"]; ok {
		tmp["db_mysql_user"] = user
	}

	if pw, ok := tmp["password"]; ok {
		tmp["db_mysql_pw"] = pw
	}

	return tmp
}

func Exec_DB(db *sql.DB, query string, values ...any) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for {
		_, err := db.ExecContext(ctx, DB_change(query), values...)
		if err == nil {
			invalidate_wiki_settings(query)
			return
		}
		if ctx.Err() == nil && strings.Contains(err.Error(), "database is locked") {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		panic(err)
	}
}

func Query_DB(db *sql.DB, query string, values ...any) *sql.Rows {
	deadline := time.Now().Add(10 * time.Second)
	for {
		rows, err := db.Query(DB_change(query), values...)
		if err == nil {
			return rows
		}

		if time.Now().Before(deadline) && strings.Contains(err.Error(), "database is locked") {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		panic(err)
	}
}

// QueryRow_DB 이래서 포인터를 배우는구나...
func QueryRow_DB(db *sql.DB, query string, var_list []any, values ...any) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for {
		err := db.QueryRowContext(ctx, DB_change(query), values...).Scan(var_list...)
		switch err {
		case nil:
			return true
		case sql.ErrNoRows:
			return false
		}

		if ctx.Err() == nil && strings.Contains(err.Error(), "database is locked") {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		panic(err)
	}
}

func DB_boot() map[string]string {
	new_db_set := Get_DB_set()
	if new_db_set["db_type"] == "mysql" {
		new_db_set = Get_DB_set_MySQL(new_db_set)
	}

	db_set = new_db_set

	return new_db_set
}

func DB_connect_init() (*sql.DB, error) {
	if db_set["db_type"] == "sqlite" {
		db, err := sql.Open("sqlite", sqlite_dsn())
		if err != nil {
			return nil, err
		}

		if err := db.Ping(); err != nil {
			db.Close()
			return nil, err
		}

		return db, nil
	} else {
		db, err := sql.Open("mysql", db_set["db_mysql_user"]+":"+db_set["db_mysql_pw"]+"@tcp("+db_set["db_mysql_host"]+":"+db_set["db_mysql_port"]+")/")
		if err != nil {
			return nil, err
		}

		if err := db.Ping(); err != nil {
			db.Close()
			return nil, err
		}

		return db, nil
	}
}

func DB_connect() *sql.DB {
	db_pool_lock.Lock()
	defer db_pool_lock.Unlock()
	if db_pool != nil {
		return db_pool
	}
	var db *sql.DB
	var err error
	if db_set["db_type"] == "sqlite" {
		db, err = sql.Open("sqlite", sqlite_dsn())
	} else {
		db, err = sql.Open("mysql", db_set["db_mysql_user"]+":"+db_set["db_mysql_pw"]+"@tcp("+db_set["db_mysql_host"]+":"+db_set["db_mysql_port"]+")/"+db_set["db_name"]+"?timeout=5s&readTimeout=30s&writeTimeout=30s")
	}
	if err != nil {
		panic(err)
	}
	db.SetMaxIdleConns(16)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)
	db_pool = db
	return db
}

func DB_close(db *sql.DB) {
	db_pool_lock.Lock()
	shared := db == db_pool
	db_pool_lock.Unlock()
	if db != nil && !shared {
		db.Close()
	}
}

// 서버가 요청 처리를 마친 뒤 연결을 정리한다.
func DB_shutdown() {
	db_pool_lock.Lock()
	defer db_pool_lock.Unlock()
	if db_pool != nil {
		db_pool.Close()
		db_pool = nil
	}
}

func Get_DB_type() string {
	return db_set["db_type"]
}

func DB_change(data string) string {
	if Get_DB_type() == "mysql" {
		data = strings.Replace(data, "random()", "rand()", -1)
		data = strings.Replace(data, "collate nocase", "collate utf8mb4_general_ci", -1)
	}

	return data
}
