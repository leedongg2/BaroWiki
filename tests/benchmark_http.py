"""읽기 경로의 반복 요청 성능 측정. 테스트 DB의 BenchLongDocument를 사용한다."""
import concurrent.futures
import json
import pathlib
import statistics
import sys
import time
import urllib.request
import urllib.parse

base, output = sys.argv[1:3]
assert urllib.parse.urlparse(base).hostname in ('127.0.0.1', 'localhost', '::1'), 'Use an isolated local test server'
paths = ['/baro', '/login', '/w/BenchLongDocument', '/recent_changes',
         '/history/BenchDocument00001', '/search/BenchDocument']
results = {}
for path in paths:
    def fetch(_):
        start = time.perf_counter()
        with urllib.request.urlopen(base + path, timeout=60) as response:
            body = response.read()
            assert response.status == 200
            if path == '/w/BenchLongDocument':
                assert b'BenchDocument00001' in body, 'Benchmark document is missing; run benchmark_seed.py on an isolated database'
        return (time.perf_counter() - start) * 1000, len(body)
    fetch(0)
    with concurrent.futures.ThreadPoolExecutor(max_workers=8) as workers:
        measurements = list(workers.map(fetch, range(32)))
    times = sorted(row[0] for row in measurements)
    results[path] = {'median_ms': round(statistics.median(times), 2),
                     'p95_ms': round(times[30], 2), 'bytes': measurements[0][1]}
    print(path, results[path], flush=True)
pathlib.Path(output).write_text(json.dumps(results, indent=2), encoding='utf-8')
