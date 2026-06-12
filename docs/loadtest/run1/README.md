# Load Test — MemoryHog.java

## Container limits
- CPUs: 2 vCPU (`--cpus="2"`)
- RAM: 2 GB (`--memory="2g"`)
- Per-request timeout: 10s

## Tool
vegeta v12.x — rate-based load generator

## How to reproduce
```bash
docker run -d --name goboxd-loadtest --cpus="2" --memory="2g" \
  -p 8080:8080 -v $(pwd)/languages.yaml:/etc/goboxd/languages.yaml:ro \
  --privileged --cgroupns=host goboxd:dev
bash docs/loadtest/load-test.sh
cd docs/loadtest && ../../venv/bin/python3 plot.py
```

## Breaking point
**RPS: 5 RPS**

The sustainable throughput of the service is roughly ~0.58 RPS on 2 vCPUs, because each request requires a compilation step (~1.8 seconds) and execution with a 1-second sleep (~1.6 seconds), consuming a single slot for ~3.4 seconds. Since concurrency is limited to 2 concurrent jobs (`max_concurrent_jobs` defaults to CPU count = 2), offered rates above 0.58 RPS quickly saturate the processing capacity and fill the request queue.

## What failed first
**Queue timeout (429)**

At 5 RPS, requests accumulated in the queue and exceeded the 30-second queue timeout (`queue_timeout_s: 30`), triggering the server to return clean `429 Too Many Requests` responses.

## Degradation behaviour
**Clean 429 responses (Graceful Degradation)**

The service degraded gracefully. It did not crash, drop connections, or experience memory exhaustion. Every single failed request received a clean HTTP `429` response with the payload:
`{"error":{"code":"queue_full","message":"server is busy, retry after 30 seconds"}}`

Memory limits inside `nsjail` and the JVM (`-Xmx512m` and `memory_kb`) successfully prevented JVM process pile-up or container out-of-memory crashes.

