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
**RPS: [FILL AFTER RUN]**

## What failed first
[FILL AFTER RUN — one of: queue timeout (429), OOM kill (memory_exceeded), JVM process pile-up]

## Degradation behaviour
[FILL AFTER RUN — did it return clean 429s or hard crash?]
