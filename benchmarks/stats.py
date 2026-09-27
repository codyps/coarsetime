"""Shared, dependency-free benchmark analysis; no historical samples in PR tests."""
import math
import random
import statistics

NAMES = ("NowInstant", "Since", "Now", "UnixNano", "TimeNow", "TimeSince", "TimeUnixNano")
ALTERNATIVES = {"NowInstant": "TimeNow", "Since": "TimeSince", "Now": "TimeNow", "UnixNano": "TimeUnixNano"}


def median(values):
    return statistics.median(values)


def interval(values, config):
    """Percentile bootstrap of the median; deterministic and paired by caller."""
    rng = random.Random(0)
    draws = sorted(median(rng.choices(values, k=len(values)))
                   for _ in range(config["bootstrap_iterations"]))
    tail = (1 - config["bootstrap_confidence"]) / 2
    return [draws[int(tail * (len(draws) - 1))], draws[math.ceil((1 - tail) * (len(draws) - 1))]]


def compare(before, after, config):
    if len(before) != len(after) or len(before) < 10:
        raise ValueError("Comparisons require at least ten paired samples")
    changes = [(b / a - 1) * 100 for a, b in zip(before, after)]
    change = median(changes)
    bounds = interval(changes, config)
    delta = median([b - a for a, b in zip(before, after)])
    alert = (abs(change) >= config["threshold_percent"]
             and abs(delta) >= config["threshold_ns"]
             and (bounds[0] > 0 or bounds[1] < 0))
    return {"before": median(before), "after": median(after), "percent": change,
            "interval": bounds, "delta_ns": delta, "alert": alert}


def summary(record):
    rows = {}
    for name in NAMES:
        samples = record["samples"]["head"][name]
        row = {"ns": median([s["ns"] for s in samples]),
               "bytes": median([s["bytes"] for s in samples]),
               "allocs": median([s["allocs"] for s in samples])}
        if name in ALTERNATIVES:
            std = record["samples"]["head"][ALTERNATIVES[name]]
            row["speedup"] = median([s["ns"] / c["ns"] for s, c in zip(std, samples)])
        rows[name] = row
    return rows
