# Load Test — MemoryHog.java (Run 2)

## Container limits
- CPUs: 2 vCPU (`--cpus="2"`)
- RAM: 2 GB (`--memory="2g"`)
- Per-request timeout: 40s (Vegeta)

## Setup for Run 2
In this run, we applied optimizations to the JVM execution within `goboxd`:
- Javac tuned flags: `-J-XX:TieredStopAtLevel=1`, `-J-XX:+UseSerialGC`, `-J-XX:CICompilerCount=1`, `-J-XX:-UsePerfData`
- Java run tuned flags: `-Xms256m` (pre-allocate heap)
- Concurrency limit `max_concurrent` increased to `6`.

## Breaking point
**RPS: 5 RPS**

The breaking point for Run 2 is 5 RPS. While the sustained throughput increased to roughly ~1.22 RPS (a +60% improvement over Run 1's ~0.76 RPS) due to the JVM tuning and increased concurrency limit, 5 RPS is still beyond the service's capacity. As a result, the queue filled up quickly. 

## What failed first
**Queue timeout (429) & Client Timeout**

In this run, `TIMEOUT=40s` was used in `vegeta`, but the server's `queue_timeout_s` was `30s`. This means requests sat in the queue for up to 30s before the server returned a `429 Too Many Requests` (graceful degradation). The `p50_ms` latency reported in Vegeta was ~30,003 ms, confirming that queued requests were reaching the 30s timeout and being rejected properly by the server before Vegeta's 40s timeout.

## Degradation behaviour
**Clean 429 responses (Graceful Degradation)**

The service degraded gracefully. It did not crash or experience memory exhaustion. Failed requests correctly received HTTP 429 responses after spending 30 seconds in the queue. Memory limits successfully prevented JVM process pile-up or container out-of-memory crashes.
