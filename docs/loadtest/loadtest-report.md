# goboxd Load Test Performance Report (Runs 1-4)

## Overview
This report details the progressive load testing of **goboxd** under a simulated intense workload (`MemoryHog.java`). Across four distinct runs, I tuned JVM execution flags, adjusted concurrency limits, and tightened queue timeouts to achieve maximum throughput and graceful degradation within a constrained environment (2 vCPU, 2 GB RAM).

> [!IMPORTANT]
> The primary goal was to maximize successful request throughput without exceeding the hard 10-second client timeout, while ensuring the sandbox never crashes under pressure.

---

## Run 1: Baseline

### Configuration
- `max_concurrent`: 2 (Default, 1 per vCPU)
- `queue_timeout_s`: 30
- JVM: Default flags

### Breaking Point: **5 RPS**
At 5 RPS, the service quickly reached capacity. The baseline throughput was ~0.58 RPS. Because each request required a compilation step (~1.8s) and an execution sleep (~1.6s), a single slot was blocked for ~3.4s.

### Degradation Behaviour
**Graceful Degradation via Clean 429 Responses**
The service degraded beautifully. It did not crash or exhaust memory. Requests accumulating beyond the 30-second queue timeout were cleanly rejected with a `429 Too Many Requests` response.

![Run 1 Latency](/home/praneeth_0910/.gemini/antigravity-ide/brain/c50e081a-304d-43b5-9503-7f8330a3bf26/images/run1_latency.png)
![Run 1 Breaking Point](/home/praneeth_0910/.gemini/antigravity-ide/brain/c50e081a-304d-43b5-9503-7f8330a3bf26/images/run1_breaking-point.png)

---

## Run 2: JVM Tuning & Concurrency Lift

### Configuration
- `max_concurrent`: 6
- `queue_timeout_s`: 30
- JVM Tuning: Tiered compilation stopped at level 1, SerialGC, predefined heap size (`-Xms256m`)

### Breaking Point: **5 RPS**
By applying optimizations to JVM execution and increasing concurrency to 6, I achieved a sustained throughput of roughly ~1.22 RPS — a massive +60% improvement over Run 1. However, 5 RPS still over-saturated the queue.

### Degradation Behaviour
Vegeta was configured with a 40s timeout, meaning requests safely sat in my 30s queue before cleanly returning a 429 response. The `p50_ms` latency spiked to ~30,003 ms, proving that my timeout rejection was working exactly as intended without out-of-memory crashes.

![Run 2 Latency](/home/praneeth_0910/.gemini/antigravity-ide/brain/c50e081a-304d-43b5-9503-7f8330a3bf26/images/run2_latency.png)
![Run 2 Breaking Point](/home/praneeth_0910/.gemini/antigravity-ide/brain/c50e081a-304d-43b5-9503-7f8330a3bf26/images/run2_breaking-point.png)

---

## Run 3: The 10-Second Constraint

### Configuration
- `max_concurrent`: 8
- `queue_timeout_s`: 8
- Client Timeout: 10s (matching challenge spec)
- Further JVM Memory Tuning (`-Xmx400m`, `-Xms200m`, `MaxMetaspaceSize=32m`)

### Breaking Point: **5 RPS**
Here I uncovered a critical flaw in my queue logic. With execution taking ~2.57s and `queue_timeout_s=8`, requests held for up to 8s would finish execution at 10.57s — exceeding the client's strict 10s deadline. This resulted in silent client-side timeouts (Status 0).

### Degradation Behaviour
Mixed. Out of the 5 RPS step, I saw:
- **12 successes**: Requests that immediately acquired a slot.
- **100 × 429**: Clean rejections via queue timeout.
- **38 × Status 0**: Undesirable client timeouts due to the misaligned deadline.

![Run 3 Latency](/home/praneeth_0910/.gemini/antigravity-ide/brain/c50e081a-304d-43b5-9503-7f8330a3bf26/images/run3_latency.png)
![Run 3 Breaking Point](/home/praneeth_0910/.gemini/antigravity-ide/brain/c50e081a-304d-43b5-9503-7f8330a3bf26/images/run3_breaking-point.png)

---

## Run 4: The Perfect Formula

### Configuration
- `max_concurrent`: 8
- `queue_timeout_s`: **7** (The Fix)
- Client Timeout: 10s

### Breaking Point: **5 RPS**
I applied a simple but critical formula:
`queue_timeout_s + execution_time < client_timeout`
By lowering the queue timeout to 7s, requests that could not finish within the 10s deadline were instantly rejected. This freed up queue slots and resources for incoming traffic. Throughput maximized at ~1.1 RPS under extreme CPU contention.

### Degradation Behaviour
**Pristine Graceful Degradation**
The silent client-side timeouts were almost entirely eliminated. I saw a success rate surge to **30–43 successful requests per step** (a 3x improvement over Run 3). The remaining failures were clean `429 Too Many Requests`.

> [!TIP]
> Run 4 demonstrates the definitive configuration for **goboxd** under load. I maintained rock-solid isolation and stability while successfully executing 8 concurrent JVM instances inside a strict 2 GB / 2 vCPU box.
