"""Create an isolated benchmark DB from an initialized test wiki. Never overwrite files.

python tests/benchmark_seed.py source-test.db .cache/benchmark/data.db
"""
import pathlib
import sqlite3
import sys

source, target = (pathlib.Path(value).resolve() for value in sys.argv[1:3])
assert source.is_file(), 'Initialize a test wiki first'
assert source != target, 'The benchmark must use a separate database'
target.parent.mkdir(parents=True, exist_ok=True)
with target.open('xb'):
    pass
with sqlite3.connect(source.as_uri() + '?mode=ro', uri=True) as origin, sqlite3.connect(target) as db:
    origin.backup(db)
    assert not db.execute("select 1 from data where title like 'BenchDocument%' or title = 'BenchLongDocument'").fetchone(), 'Source already contains benchmark documents'
    db.executemany('insert into data (title, data) values (?, ?)',
                   ((f'BenchDocument{i:05d}', 'Benchmark document body') for i in range(10000)))
    long_text = '[목차]\n' + '\n'.join(
        f'= 문단 {i} =\n[[BenchDocument00001]] [[BenchDocument00002]] [[BenchDocument00001]]\n'
        f'||항목||설명||\n||{i}||본문 내용||' for i in range(100))
    db.execute('insert into data (title, data) values (?, ?)', ('BenchLongDocument', long_text))
    history, recent = [], []
    for document in range(1000):
        for revision in range(1, 51):
            title, rev = f'BenchDocument{document:05d}', str(revision)
            date = f'2026-09-09 10:{revision // 60:02d}:{revision % 60:02d}'
            history.append((rev, title, '본문', date, 'admin', 'benchmark', '+2', '', ''))
            recent.append((rev, title, date, ''))
    db.executemany('insert into history (id,title,data,date,ip,send,leng,hide,type) values (?,?,?,?,?,?,?,?,?)', history)
    db.executemany('insert into rc (id,title,date,type) values (?,?,?,?)', recent)
    db.commit()
print('Created 10,001 benchmark documents and 50,000 history/recent-change rows:', target)
