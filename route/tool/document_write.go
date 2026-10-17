package tool

import (
	"context"
	"database/sql"
	"hash/fnv"
	"sync"
	"time"
)

// 잠금 개수를 고정해 임의의 문서명 요청으로 메모리가 늘지 않게 한다.
var document_edit_locks [256]sync.Mutex

func Lock_document_edit(title string) func() {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(title))
	lock := &document_edit_locks[hash.Sum32()%uint32(len(document_edit_locks))]
	lock.Lock()
	return lock.Unlock
}

// 본문 교체 중 독자가 빈 문서나 중복된 본문을 보지 않도록 원자적으로 저장한다.
func Replace_document_data(db *sql.DB, title, data string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "delete from data where title = ?", title); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "insert into data (title, data) values (?, ?)", title, data); err != nil {
		return err
	}
	return tx.Commit()
}
