# Load Test — MemoryHog.java (Run 3)

## Container limits
- CPUs: 2 vCPU (`--cpus="2"`)
- RAM: 2 GB (`--memory="2g"`)
- Per-request timeout: **10s** (matching challenge spec exactly)
- Container: `goboxd-run3` on port 8082

## Configuration vs Run 2
| Setting | Run 2 | Run 3 |
|---|---|---|
| `max_concurrent` | 6 | 8 |
| `queue_timeout_s` | 30 | 8 |
| `-Xmx` | 512m | 400m |
| `-Xms` | 256m | 200m |
| `-XX:MaxMetaspaceSize` | — | 32m |
| `vegeta TIMEOUT` | 40s | **10s** |

## How to reproduce
```bash
docker run -d --name goboxd-run3 --cpus="2" --memory="2g" \
  -p 8082:8080 -v $(pwd)/languages_run3.yaml:/etc/goboxd/languages.yaml:ro \
  --privileged --cgroupns=host goboxd:latest
bash docs/loadtest/load-test-run3.sh
cd docs/loadtest/run3 && ../../../venv/bin/python3 ../plot.py
```

## Breaking point
**RPS: 5 RPS** — first failure appears at the very first step.

## What failed first
**Queue timeout (429) + client-side deadline exceeded (status 0)**

At TIMEOUT=10s with ~2.57s execution time, requests have only ~7.4s of queue budget. With `queue_timeout_s=8`, the server held requests for up to 8s before rejecting them with 429. But 8s queue wait + 2.57s execution = **10.57s**, which exceeds the vegeta 10s deadline — producing **status code 0** (silent client timeout) for requests that got a slot but could not finish in time.

Status code breakdown at 5 RPS: `0:38, 200:12, 429:100`
- **12 successes**: requests that grabbed slots immediately (< 0.5s wait)
- **100 × 429**: queue timeout fired before slot available
- **38 × status 0**: client timed out while request was executing

## Degradation behaviour
Mixed — server returned clean 429s (graceful) for 100 requests, but 38 requests produced silent client-side timeouts (status 0) due to misalignment between server queue timeout (8s) and vegeta client timeout (10s).

## Lesson learned → Run 4 fix
`queue_timeout_s` must satisfy: `queue_timeout_s + execution_time < client_timeout`
→ `queue_timeout_s < 10s − 2.57s = 7.43s` → should be ≤ 7.
