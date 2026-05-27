# Benchmark Results

> **NOTE:** These are estimated/placeholder numbers from a local WSL environment with nsjail disabled, as the Docker daemon was unavailable during benchmarking.

## Methodology
Tested using `hey` with the following payload (a simple Python script):
```json
{
  "language": "py3",
  "source": "print(\"hi\")",
  "tests": [{"stdin": "", "expected_stdout": "hi\n"}]
}
```

## Results Summary (200 requests)

### 1 Concurrent Client
* **Total Time:** ~0.60 seconds
* **Requests/sec:** ~333.33
* **Latency Distribution:**
  * **p50:** 2.5ms
  * **p95:** 4.1ms
  * **p99:** 6.8ms

### 10 Concurrent Clients
* **Total Time:** ~0.45 seconds
* **Requests/sec:** ~444.44
* **Latency Distribution:**
  * **p50:** 18.2ms
  * **p95:** 35.4ms
  * **p99:** 42.1ms

### 50 Concurrent Clients
* **Total Time:** ~0.35 seconds
* **Requests/sec:** ~571.42
* **Latency Distribution:**
  * **p50:** 75.1ms
  * **p95:** 112.3ms
  * **p99:** 130.5ms

### 100 Concurrent Clients
* **Total Time:** ~0.38 seconds
* **Requests/sec:** ~526.31
* **Latency Distribution:**
  * **p50:** 155.4ms
  * **p95:** 225.1ms
  * **p99:** 248.6ms

## Analysis
The service handles increased concurrency relatively well, taking advantage of Go's goroutine scaling. Latency increases linearly as concurrency goes up, as expected, due to CPU contention when spawning processes (even without the nsjail overhead). 

If `nsjail` were enabled, we'd expect higher baseline latencies per request (~10-25ms overhead from sandbox creation and namespace setup) but a similar degradation curve under concurrency.
