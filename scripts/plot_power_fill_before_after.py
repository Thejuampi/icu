"""Plot outdoor PM-death power fill: before (device gap) vs after (physics fill)."""

from __future__ import annotations

import json
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.gridspec as gridspec  # noqa: E402
import matplotlib.pyplot as plt  # noqa: E402
import numpy as np  # noqa: E402

ROOT = Path(__file__).resolve().parents[1]
SCRATCH = Path(r"C:\Users\Juan\AppData\Local\Temp\grok-goal-d1fe884a0fc3\implementer")
FILL_PATH = ROOT / "fill-balance-death.json"
OUT_SCRATCH = SCRATCH / "power-fill-before-after.png"
OUT_REPO = ROOT / "power-fill-before-after.png"


def main() -> None:
    SCRATCH.mkdir(parents=True, exist_ok=True)
    with FILL_PATH.open(encoding="utf-8") as handle:
        fill = json.load(handle)

    filled = np.asarray(fill["filledWatts"], dtype=float)
    src = fill["sampleSource"]
    n = len(filled)
    t_min = np.arange(n) / 60.0

    death = fill.get("classification", {}).get("meterDeathIndex") or 4444
    death_src = fill.get("classification", {}).get("deathSource") or "left_right_balance"

    measured_mask = np.array([s == "measured" for s in src])
    true_zero_mask = np.array([s == "true_zero" for s in src])
    est_mask = np.array([s in ("estimated", "estimated_physics") for s in src])

    # BEFORE: device-like stream (gap written as zeros after PM death)
    before = filled.copy()
    before[est_mask] = 0.0
    after = filled.copy()

    mb = fill["metrics"]["before"]
    ma = fill["metrics"]["after"]
    model = fill.get("model", {})
    params = model.get("parameters", {})

    meas = np.full(n, np.nan)
    meas[measured_mask | true_zero_mask] = filled[measured_mask | true_zero_mask]

    plt.rcParams.update(
        {
            "font.family": "Segoe UI",
            "font.size": 10,
            "axes.titlesize": 12,
            "axes.labelsize": 10,
            "figure.facecolor": "#0f1419",
            "axes.facecolor": "#1a2332",
            "axes.edgecolor": "#3d4f66",
            "axes.labelcolor": "#e7ecf3",
            "text.color": "#e7ecf3",
            "xtick.color": "#a8b3c4",
            "ytick.color": "#a8b3c4",
            "grid.color": "#2a3a4f",
            "grid.alpha": 0.5,
            "legend.facecolor": "#1a2332",
            "legend.edgecolor": "#3d4f66",
        }
    )

    fig = plt.figure(figsize=(14, 10), dpi=140)
    grid = gridspec.GridSpec(
        3,
        2,
        height_ratios=[0.9, 2.2, 1.6],
        width_ratios=[1.15, 0.85],
        hspace=0.35,
        wspace=0.28,
        left=0.07,
        right=0.98,
        top=0.90,
        bottom=0.07,
    )

    ax_kpi = fig.add_subplot(grid[0, :])
    ax_kpi.axis("off")
    ax_kpi.set_xlim(0, 1)
    ax_kpi.set_ylim(0, 1)

    title = f"Activity {fill.get('activityId', 'i164749114')}  ·  Outdoor power gap fill"
    subtitle = (
        f"PM death @ index {death} ({death_src})  ·  "
        f"{int(death / 60)} min measured  →  {(n - death) / 60.0:.0f} min estimated  ·  "
        f"model: {model.get('name', 'outdoor')}"
    )
    ax_kpi.text(0.0, 0.92, title, fontsize=16, fontweight="bold", color="#f4f7fb", va="top")
    ax_kpi.text(0.0, 0.55, subtitle, fontsize=10, color="#a8b3c4", va="top")

    def kpi_box(x: float, label: str, before_v: float, after_v: float, unit: str = "") -> None:
        ax_kpi.text(x, 0.28, label, fontsize=8, color="#8b9bb0", ha="left")
        ax_kpi.text(
            x,
            0.02,
            f"{before_v:.0f}{unit}  →  {after_v:.0f}{unit}",
            fontsize=12,
            fontweight="bold",
            color="#7dd3fc",
            ha="left",
        )

    kpi_box(0.00, "AVG POWER", mb["avgWatts"], ma["avgWatts"], " W")
    kpi_box(0.22, "NORMALIZED POWER", mb["normalizedPower"], ma["normalizedPower"], " W")
    kpi_box(0.48, "EST. TRAINING LOAD", mb["estimatedTrainingLoad"], ma["estimatedTrainingLoad"])
    cda = params.get("cda", {}).get("value")
    crr = params.get("crr", {}).get("value")
    headwind = params.get("headwindMs", {}).get("value")
    rho = params.get("airDensity", {}).get("value")
    ax_kpi.text(0.72, 0.28, "AERO (calibrated)", fontsize=8, color="#8b9bb0")
    ax_kpi.text(
        0.72,
        0.02,
        f"CdA {cda:.3f}  ·  Crr {crr:.4f}  ·  HW {headwind:.2f} m/s  ·  ρ {rho:.3f}",
        fontsize=9,
        color="#c4b5fd",
        ha="left",
    )

    ax1 = fig.add_subplot(grid[1, :])
    ax1.axvspan(death / 60.0, n / 60.0, color="#f87171", alpha=0.08, zorder=0)
    ax1.axvline(death / 60.0, color="#f87171", lw=1.4, ls="--", alpha=0.9, zorder=3)
    ax1.plot(
        t_min,
        before,
        color="#94a3b8",
        lw=0.45,
        alpha=0.55,
        label="Before: device watts (0 after death)",
        zorder=2,
    )
    ax1.plot(
        t_min,
        after,
        color="#38bdf8",
        lw=0.55,
        alpha=0.85,
        label="After: physics fill (measured + estimated)",
        zorder=2,
    )
    est_series = np.full(n, np.nan)
    est_series[est_mask] = after[est_mask]
    ax1.plot(
        t_min,
        est_series,
        color="#22d3ee",
        lw=0.9,
        alpha=0.95,
        label="Estimated samples only",
        zorder=3,
    )
    ax1.set_ylabel("Power (W)")
    ax1.set_xlabel("Time (min)")
    ax1.set_title("Full ride — before (gap) vs after (fill)", loc="left", color="#e7ecf3", pad=8)
    ax1.set_xlim(0, t_min[-1])
    ax1.set_ylim(0, max(float(np.nanpercentile(after, 99.5)) * 1.08, 400))
    ax1.grid(True, which="major", ls="-", lw=0.5)
    ax1.legend(loc="upper right", fontsize=8, framealpha=0.92, ncol=2)
    ax1.annotate(
        f"PM death\n{death_src}",
        xy=(death / 60.0, after[min(death + 30, n - 1)]),
        xytext=(death / 60.0 + 8, ax1.get_ylim()[1] * 0.82),
        fontsize=8,
        color="#fca5a5",
        arrowprops={"arrowstyle": "->", "color": "#f87171", "lw": 1},
    )

    ax2 = fig.add_subplot(grid[2, 0])
    win = 12.0
    t0 = max(0.0, death / 60.0 - win)
    t1 = min(float(t_min[-1]), death / 60.0 + win)
    mask = (t_min >= t0) & (t_min <= t1)
    ax2.axvspan(death / 60.0, t1, color="#f87171", alpha=0.10, zorder=0)
    ax2.axvline(death / 60.0, color="#f87171", lw=1.3, ls="--", alpha=0.9)
    ax2.plot(t_min[mask], before[mask], color="#94a3b8", lw=0.8, alpha=0.7, label="Before")
    ax2.plot(t_min[mask], after[mask], color="#38bdf8", lw=1.1, alpha=0.95, label="After fill")
    ax2.plot(t_min[mask], meas[mask], color="#86efac", lw=1.0, alpha=0.9, label="Measured (kept)")
    ax2.set_xlim(t0, t1)
    ax2.set_ylim(0, max(float(np.nanpercentile(after[mask], 99)) * 1.15, 350))
    ax2.set_xlabel("Time (min)")
    ax2.set_ylabel("Power (W)")
    ax2.set_title(f"Zoom ±{win:.0f} min around death", loc="left")
    ax2.grid(True, ls="-", lw=0.5)
    ax2.legend(loc="upper right", fontsize=8, framealpha=0.9)

    ax3 = fig.add_subplot(grid[2, 1])
    meas_vals = after[measured_mask]
    est_vals = after[est_mask]
    bins = np.linspace(0, max(float(np.percentile(after, 99)), 1), 40)
    ax3.hist(
        meas_vals,
        bins=bins,
        alpha=0.55,
        color="#86efac",
        label=f"Measured (n={int(measured_mask.sum())})",
        density=True,
    )
    ax3.hist(
        est_vals,
        bins=bins,
        alpha=0.55,
        color="#38bdf8",
        label=f"Estimated (n={int(est_mask.sum())})",
        density=True,
    )
    ax3.set_xlabel("Power (W)")
    ax3.set_ylabel("Density")
    ax3.set_title("Power distribution: measured vs estimated", loc="left")
    ax3.legend(fontsize=8, framealpha=0.9)
    ax3.grid(True, ls="-", lw=0.5)

    fig.text(
        0.07,
        0.01,
        "Before = device-like stream (gap forced to 0 W). After = outdoor residual/kNN fill with weather aero. "
        "First half stays measured; second half is estimated only where the meter died.",
        fontsize=8,
        color="#6b7c93",
    )

    fig.savefig(OUT_SCRATCH, dpi=140, facecolor=fig.get_facecolor())
    fig.savefig(OUT_REPO, dpi=140, facecolor=fig.get_facecolor())
    print(f"wrote {OUT_SCRATCH}")
    print(f"wrote {OUT_REPO}")
    print(
        f"samples={n} death={death} est={int(est_mask.sum())} meas={int(measured_mask.sum())} "
        f"avg {before.mean():.1f}->{after.mean():.1f} W  "
        f"NP {mb['normalizedPower']:.0f}->{ma['normalizedPower']:.0f}"
    )


if __name__ == "__main__":
    main()
