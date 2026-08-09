"""Compare successive outdoor power-fill methodologies on the same ride.

Before = earlier full second-half weather fill (fill-second-half-weather.json)
After  = L/R balance death + weather fill (fill-balance-death.json)
"""

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

BEFORE_PATH = ROOT / "fill-second-half-weather.json"  # earlier methodology
AFTER_PATH = ROOT / "fill-balance-death.json"  # later methodology
OUT_SCRATCH = SCRATCH / "power-fill-methodologies.png"
OUT_REPO = ROOT / "power-fill-methodologies.png"


def load_fill(path: Path) -> dict:
    with path.open(encoding="utf-8") as handle:
        return json.load(handle)


def series(fill: dict) -> np.ndarray:
    return np.asarray(fill["filledWatts"], dtype=float)


def est_mask(fill: dict) -> np.ndarray:
    return np.array([s in ("estimated", "estimated_physics") for s in fill["sampleSource"]])


def measured_mask(fill: dict) -> np.ndarray:
    return np.array([s in ("measured", "true_zero") for s in fill["sampleSource"]])


def main() -> None:
    SCRATCH.mkdir(parents=True, exist_ok=True)
    before_fill = load_fill(BEFORE_PATH)
    after_fill = load_fill(AFTER_PATH)

    before = series(before_fill)
    after = series(after_fill)
    if len(before) != len(after):
        raise SystemExit(f"length mismatch: before={len(before)} after={len(after)}")

    n = len(after)
    t_min = np.arange(n) / 60.0

    death_before = before_fill.get("classification", {}).get("meterDeathIndex")
    death_after = after_fill.get("classification", {}).get("meterDeathIndex") or 4444
    death_src = after_fill.get("classification", {}).get("deathSource") or "left_right_balance"

    # Prefer after death for region shading; also mark earlier death if different.
    death_ref = int(death_after)
    est_a = est_mask(after_fill)
    est_b = est_mask(before_fill)
    either_est = est_a | est_b
    both_est = est_a & est_b

    delta = after - before
    # Stats on region either method treated as estimated (the contested second half)
    delta_est = delta[either_est]
    mae = float(np.mean(np.abs(delta_est))) if delta_est.size else 0.0
    bias = float(np.mean(delta_est)) if delta_est.size else 0.0
    rmse = float(np.sqrt(np.mean(delta_est**2))) if delta_est.size else 0.0
    max_abs = float(np.max(np.abs(delta_est))) if delta_est.size else 0.0
    agree = float(np.mean(np.abs(delta_est) <= 15)) if delta_est.size else 0.0  # within 15 W

    mb = before_fill["metrics"]["after"]
    ma = after_fill["metrics"]["after"]
    fb = before_fill.get("fill", {})
    fa = after_fill.get("fill", {})

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

    fig = plt.figure(figsize=(14.5, 11), dpi=140)
    grid = gridspec.GridSpec(
        4,
        2,
        height_ratios=[0.85, 2.0, 1.35, 1.45],
        width_ratios=[1.2, 0.8],
        hspace=0.38,
        wspace=0.28,
        left=0.07,
        right=0.98,
        top=0.91,
        bottom=0.06,
    )

    # --- KPI header ---
    ax_kpi = fig.add_subplot(grid[0, :])
    ax_kpi.axis("off")
    ax_kpi.set_xlim(0, 1)
    ax_kpi.set_ylim(0, 1)
    ax_kpi.text(
        0.0,
        0.95,
        f"Activity {after_fill.get('activityId', 'i164749114')}  ·  Fill methodology comparison",
        fontsize=16,
        fontweight="bold",
        color="#f4f7fb",
        va="top",
    )
    ax_kpi.text(
        0.0,
        0.58,
        "Before = second-half weather fill (earlier)   ·   "
        "After = L/R balance-death + weather fill (later)   ·   "
        "Not device zeros — both are full physics fills",
        fontsize=10,
        color="#a8b3c4",
        va="top",
    )

    def kpi(x: float, label: str, left: str, right: str, color: str = "#7dd3fc") -> None:
        ax_kpi.text(x, 0.28, label, fontsize=8, color="#8b9bb0")
        ax_kpi.text(x, 0.02, f"{left}  →  {right}", fontsize=11, fontweight="bold", color=color)

    kpi(0.00, "AVG POWER (ride)", f"{mb['avgWatts']:.0f} W", f"{ma['avgWatts']:.0f} W")
    kpi(0.20, "NP", f"{mb['normalizedPower']:.0f} W", f"{ma['normalizedPower']:.0f} W")
    kpi(0.36, "EST. LOAD", f"{mb['estimatedTrainingLoad']:.0f}", f"{ma['estimatedTrainingLoad']:.0f}")
    kpi(
        0.50,
        "ESTIMATED SEC",
        f"{fb.get('estimatedSeconds', 0)}",
        f"{fa.get('estimatedSeconds', 0)}",
        color="#c4b5fd",
    )
    kpi(
        0.66,
        "MEAN EST WATTS",
        f"{fb.get('meanEstimatedWatts', 0):.0f}",
        f"{fa.get('meanEstimatedWatts', 0):.0f}",
        color="#c4b5fd",
    )
    death_b_txt = str(death_before) if death_before is not None else "n/a"
    ax_kpi.text(0.84, 0.28, "DEATH INDEX", fontsize=8, color="#8b9bb0")
    ax_kpi.text(
        0.84,
        0.02,
        f"{death_b_txt} → {death_after} ({death_src})",
        fontsize=10,
        fontweight="bold",
        color="#fca5a5",
    )

    # --- Overlay timeline ---
    ax1 = fig.add_subplot(grid[1, :])
    ax1.axvspan(death_ref / 60.0, n / 60.0, color="#f87171", alpha=0.07, zorder=0)
    ax1.axvline(death_ref / 60.0, color="#f87171", lw=1.3, ls="--", alpha=0.9, zorder=4)
    if death_before is not None and int(death_before) != death_ref:
        ax1.axvline(int(death_before) / 60.0, color="#fbbf24", lw=1.0, ls=":", alpha=0.85, zorder=4)
        ax1.annotate(
            f"earlier death {death_before}",
            xy=(int(death_before) / 60.0, 50),
            xytext=(int(death_before) / 60.0 - 18, 420),
            fontsize=7,
            color="#fbbf24",
            arrowprops={"arrowstyle": "->", "color": "#fbbf24", "lw": 0.8},
        )

    # Downsample plot lines slightly for readability? Keep full 1 Hz with thin lines.
    ax1.plot(
        t_min,
        before,
        color="#fbbf24",
        lw=0.55,
        alpha=0.75,
        label="Before: second-half weather fill",
        zorder=2,
    )
    ax1.plot(
        t_min,
        after,
        color="#38bdf8",
        lw=0.55,
        alpha=0.85,
        label="After: balance-death + weather fill",
        zorder=3,
    )
    # Measured kept (from after classification — nearly identical)
    meas = np.full(n, np.nan)
    meas[measured_mask(after_fill)] = after[measured_mask(after_fill)]
    ax1.plot(t_min, meas, color="#86efac", lw=0.45, alpha=0.55, label="Measured kept (after mask)", zorder=1)

    ax1.set_ylabel("Power (W)")
    ax1.set_xlabel("Time (min)")
    ax1.set_title("Full ride — methodology overlay", loc="left", pad=8)
    ax1.set_xlim(0, float(t_min[-1]))
    ax1.set_ylim(0, max(float(np.percentile(np.concatenate([before, after]), 99.5)) * 1.08, 400))
    ax1.grid(True, ls="-", lw=0.5)
    ax1.legend(loc="upper right", fontsize=8, framealpha=0.92, ncol=2)
    ax1.annotate(
        f"PM death\n{death_src}",
        xy=(death_ref / 60.0, after[min(death_ref + 20, n - 1)]),
        xytext=(death_ref / 60.0 + 10, ax1.get_ylim()[1] * 0.78),
        fontsize=8,
        color="#fca5a5",
        arrowprops={"arrowstyle": "->", "color": "#f87171", "lw": 1},
    )

    # --- Delta timeline ---
    ax2 = fig.add_subplot(grid[2, 0])
    ax2.axhline(0, color="#64748b", lw=1.0, zorder=1)
    ax2.axvspan(death_ref / 60.0, n / 60.0, color="#f87171", alpha=0.07, zorder=0)
    ax2.axvline(death_ref / 60.0, color="#f87171", lw=1.2, ls="--", alpha=0.9)
    # Only show delta where at least one method estimated (else ~0 noise)
    delta_plot = np.where(either_est, delta, np.nan)
    ax2.fill_between(t_min, 0, delta_plot, where=np.isfinite(delta_plot) & (delta_plot >= 0), color="#38bdf8", alpha=0.45)
    ax2.fill_between(t_min, 0, delta_plot, where=np.isfinite(delta_plot) & (delta_plot < 0), color="#fbbf24", alpha=0.45)
    ax2.plot(t_min, delta_plot, color="#e2e8f0", lw=0.4, alpha=0.7)
    ax2.set_xlim(0, float(t_min[-1]))
    ax2.set_ylabel("Δ Power (W)")
    ax2.set_xlabel("Time (min)")
    ax2.set_title("After − Before  (estimated region only)", loc="left")
    ax2.grid(True, ls="-", lw=0.5)
    ax2.text(
        0.01,
        0.97,
        f"bias {bias:+.1f} W · MAE {mae:.1f} W · RMSE {rmse:.1f} W · |Δ|≤15 W {agree:.0%}",
        transform=ax2.transAxes,
        va="top",
        fontsize=8,
        color="#cbd5e1",
        bbox={"boxstyle": "round,pad=0.3", "facecolor": "#0f1419", "edgecolor": "#3d4f66", "alpha": 0.85},
    )

    # --- Zoom around death ---
    ax3 = fig.add_subplot(grid[2, 1])
    win = 10.0
    t0 = max(0.0, death_ref / 60.0 - win)
    t1 = min(float(t_min[-1]), death_ref / 60.0 + win)
    m = (t_min >= t0) & (t_min <= t1)
    ax3.axvspan(death_ref / 60.0, t1, color="#f87171", alpha=0.08, zorder=0)
    ax3.axvline(death_ref / 60.0, color="#f87171", lw=1.2, ls="--")
    ax3.plot(t_min[m], before[m], color="#fbbf24", lw=1.0, alpha=0.9, label="Before method")
    ax3.plot(t_min[m], after[m], color="#38bdf8", lw=1.0, alpha=0.95, label="After method")
    ax3.set_xlim(t0, t1)
    ax3.set_ylim(0, max(float(np.nanpercentile(np.concatenate([before[m], after[m]]), 99)) * 1.12, 350))
    ax3.set_xlabel("Time (min)")
    ax3.set_ylabel("Power (W)")
    ax3.set_title(f"Zoom ±{win:.0f} min around death", loc="left")
    ax3.grid(True, ls="-", lw=0.5)
    ax3.legend(loc="upper right", fontsize=8, framealpha=0.9)

    # --- Scatter: before vs after on dual-estimated samples ---
    ax4 = fig.add_subplot(grid[3, 0])
    if both_est.any():
        # subsample for scatter density
        idx = np.flatnonzero(both_est)
        if len(idx) > 4000:
            rng = np.random.default_rng(0)
            idx = rng.choice(idx, size=4000, replace=False)
        xb, ya = before[idx], after[idx]
        ax4.scatter(xb, ya, s=4, alpha=0.25, c="#38bdf8", edgecolors="none")
        lim = max(float(np.percentile(np.concatenate([xb, ya]), 99.5)), 50)
        ax4.plot([0, lim], [0, lim], color="#94a3b8", lw=1.0, ls="--", label="1:1")
        # linear fit
        if len(xb) > 10:
            coef = np.polyfit(xb, ya, 1)
            xx = np.linspace(0, lim, 50)
            ax4.plot(xx, coef[0] * xx + coef[1], color="#c4b5fd", lw=1.2, label=f"fit y={coef[0]:.2f}x{coef[1]:+.0f}")
        ax4.set_xlim(0, lim)
        ax4.set_ylim(0, lim)
        r = float(np.corrcoef(xb, ya)[0, 1]) if len(xb) > 2 else 0.0
        ax4.set_title(f"Estimated samples: before vs after (r={r:.3f})", loc="left")
    else:
        ax4.text(0.5, 0.5, "No overlapping estimated samples", ha="center", va="center")
        ax4.set_title("Estimated samples: before vs after", loc="left")
    ax4.set_xlabel("Before method (W)")
    ax4.set_ylabel("After method (W)")
    ax4.grid(True, ls="-", lw=0.5)
    ax4.legend(loc="upper left", fontsize=8, framealpha=0.9)

    # --- Delta histogram on estimated region ---
    ax5 = fig.add_subplot(grid[3, 1])
    if delta_est.size:
        # clip display for readability
        lo, hi = np.percentile(delta_est, [1, 99])
        bins = np.linspace(lo, hi, 50)
        ax5.hist(delta_est, bins=bins, color="#818cf8", alpha=0.85, edgecolor="#1a2332")
        ax5.axvline(0, color="#e2e8f0", lw=1.0)
        ax5.axvline(bias, color="#f472b6", lw=1.2, ls="--", label=f"mean bias {bias:+.1f} W")
    ax5.set_xlabel("After − Before (W)")
    ax5.set_ylabel("Count (samples)")
    ax5.set_title("How much the newer method shifts estimated watts", loc="left")
    ax5.grid(True, ls="-", lw=0.5)
    ax5.legend(loc="upper right", fontsize=8, framealpha=0.9)

    fig.text(
        0.07,
        0.01,
        "Both series are complete filled watts (not device zeros). "
        f"Contested region: {int(either_est.sum())}s estimated by either method · "
        f"max |Δ| {max_abs:.0f} W · gold=earlier fill higher, cyan=later fill higher on delta chart.",
        fontsize=8,
        color="#6b7c93",
    )

    fig.savefig(OUT_SCRATCH, dpi=140, facecolor=fig.get_facecolor())
    fig.savefig(OUT_REPO, dpi=140, facecolor=fig.get_facecolor())
    print(f"wrote {OUT_SCRATCH}")
    print(f"wrote {OUT_REPO}")
    print(
        f"bias={bias:+.2f} mae={mae:.2f} rmse={rmse:.2f} agree15={agree:.1%} "
        f"avg {mb['avgWatts']:.1f}->{ma['avgWatts']:.1f} "
        f"estSec {fb.get('estimatedSeconds')}->{fa.get('estimatedSeconds')}"
    )


if __name__ == "__main__":
    main()
