import csv
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt

rows = list(csv.DictReader(open("results.csv")))
rps  = [float(r["target_rps"]) for r in rows]
errs = [float(r["error_pct"])   for r in rows]

# Find breaking point - first row with errors
bp_rps = None
for r in rows:
    if float(r["error_pct"]) > 0:
        bp_rps = float(r["target_rps"])
        break

# --- Graph 1: Breaking point ---
fig, ax = plt.subplots(figsize=(10, 6))
ax.plot(rps, errs, marker="o", linewidth=2, color="#e74c3c", label="Error rate %")
if bp_rps:
    ax.axvline(x=bp_rps, color="#2c3e50", linestyle="--", linewidth=1.5,
               label=f"Breaking point: {bp_rps} RPS")
ax.set_xlabel("Offered RPS", fontsize=12)
ax.set_ylabel("Error rate (%)", fontsize=12)
ax.set_title("goboxd — MemoryHog breaking point (2vCPU/2GB)", fontsize=14)
ax.legend()
ax.grid(True, alpha=0.3)
plt.tight_layout()
plt.savefig("breaking-point.png", dpi=150, bbox_inches="tight")
print(f"Saved breaking-point.png | Breaking point: {bp_rps} RPS")

# --- Graph 2: Latency curve ---
fig, ax = plt.subplots(figsize=(10, 6))
for key, label, color in [
    ("p50_ms", "p50", "#2ecc71"),
    ("p95_ms", "p95", "#f39c12"),
    ("p99_ms", "p99", "#e74c3c"),
]:
    ax.plot(rps, [float(r[key]) for r in rows],
            marker="o", linewidth=2, label=label, color=color)
if bp_rps:
    ax.axvline(x=bp_rps, color="#2c3e50", linestyle="--", linewidth=1.5,
               label=f"Breaking point: {bp_rps} RPS")
ax.set_xlabel("Offered RPS", fontsize=12)
ax.set_ylabel("Latency (ms)", fontsize=12)
ax.set_title("goboxd — MemoryHog latency curve (2vCPU/2GB)", fontsize=14)
ax.legend()
ax.grid(True, alpha=0.3)
plt.tight_layout()
plt.savefig("latency.png", dpi=150, bbox_inches="tight")
print("Saved latency.png")
