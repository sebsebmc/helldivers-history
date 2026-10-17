#!/usr/bin/env python3
"""Regenerate cases.json, the parity fixture for the Go tests.

Each case mutates a small snapshot and records what the original pydantic
pipeline does with it: rejects it at validation (the snapshot is skipped),
crashes during conversion, or produces a given current_status JSON.

Run from the repository root with the Python requirements installed:

    python3 buildstats/testdata/gen_cases.py
"""

import copy
import datetime
import json
import os
import sys

sys.path.insert(0, os.getcwd())

from pydantic import ValidationError  # noqa: E402

from build_stats import v1_to_frontend  # noqa: E402
from models import v1  # noqa: E402

HERE = os.path.dirname(os.path.abspath(__file__))
SNAPSHOT_AT = datetime.datetime(2026, 10, 17, 16, 0, 0, tzinfo=datetime.timezone.utc)


def base():
    with open(os.path.join(HERE, "base_snapshot.json")) as fh:
        return json.load(fh)


def mutate(fn):
    doc = base()
    fn(doc)
    return doc


def setp(path, value):
    def fn(doc):
        obj = doc
        for k in path[:-1]:
            obj = obj[k]
        obj[path[-1]] = value
    return fn


def delp(path):
    def fn(doc):
        obj = doc
        for k in path[:-1]:
            obj = obj[k]
        del obj[path[-1]]
    return fn


P0 = ["planets", 0]
EV = ["planets", 1, "event"]
CP = ["campaigns", 0, "planet"]

CASES = {
    "base": lambda d: None,
    "war null": setp(["war"], None),
    "planets null": setp(["planets"], None),
    "extra top-level key ignored": setp(["extra"], 1),
    "extra war key": setp(["war", "newField"], 1),
    "extra war statistics key": setp(["war", "statistics", "newStat"], 1),
    "extra planet key": setp(P0 + ["newField"], 1),
    "extra position key": setp(P0 + ["position", "z"], 1),
    "extra region key": setp(P0 + ["regions", 0, "newField"], 1),
    "extra event key ignored": setp(EV + ["newField"], 1),
    "extra biome key ignored": setp(P0 + ["biome", "newField"], 1),
    "extra hazard key ignored": setp(P0 + ["hazards", 0, "newField"], 1),
    "extra campaign key ignored": setp(["campaigns", 0, "newField"], 1),
    "extra campaign planet key": setp(CP + ["newField"], 1),
    "extra assignment key": setp(["assignments", 0, "newField"], 1),
    "extra task key": setp(["assignments", 0, "tasks", 0, "newField"], 1),
    "extra reward key": setp(["assignments", 0, "reward", "newField"], 1),
    "extra dispatch key": setp(["dispatches", 0, "newField"], 1),
    "planet health null": setp(P0 + ["health"], None),
    "planet health missing": delp(P0 + ["health"]),
    "planet health float": setp(P0 + ["health"], 1000000.0),
    "planet health fractional": setp(P0 + ["health"], 1000000.5),
    "planet health string": setp(P0 + ["health"], "1000000"),
    "planet health bool": setp(P0 + ["health"], True),
    "planet name string": setp(P0 + ["name"], "PLAIN NAME"),
    "planet name null value": setp(P0 + ["name", "de-DE"], None),
    "planet name number": setp(P0 + ["name"], 5),
    "planet sector null": setp(P0 + ["sector"], None),
    "planet regen null": setp(P0 + ["regenPerSecond"], None),
    "planet regen missing": delp(P0 + ["regenPerSecond"]),
    "planet regen int": setp(P0 + ["regenPerSecond"], 3),
    "planet regen string": setp(P0 + ["regenPerSecond"], "1.5"),
    "planet disabled string": setp(P0 + ["disabled"], "yes"),
    "planet disabled int": setp(P0 + ["disabled"], 1),
    "planet disabled null": setp(P0 + ["disabled"], None),
    "planet waypoints null": setp(P0 + ["waypoints"], None),
    "planet waypoint null element": setp(P0 + ["waypoints"], [1, None]),
    "planet attacking null": setp(P0 + ["attacking"], None),
    "planet statistics null": setp(P0 + ["statistics"], None),
    "planet player count null": setp(P0 + ["statistics", "playerCount"], None),
    "planet position x null": setp(P0 + ["position", "x"], None),
    "planet position small": setp(P0 + ["position"], {"x": 1.234e-7, "y": -0.00001}),
    "planet position large": setp(P0 + ["position"], {"x": 1.5e15, "y": 123456789012345678}),
    "planet initial owner null": setp(P0 + ["initialOwner"], None),
    "planet current owner not humans": setp(P0 + ["currentOwner"], "Automaton"),
    "planet string escapes": setp(P0 + ["sector"], "a\"b\\c/d\n\t\u0001\u007f <>&Ü"),
    "region size invalid": setp(P0 + ["regions", 0, "size"], "Village"),
    "region max health null": setp(P0 + ["regions", 0, "maxHealth"], None),
    "region max health missing": delp(P0 + ["regions", 0, "maxHealth"]),
    "region is available null": setp(P0 + ["regions", 0, "isAvailable"], None),
    "region players missing": delp(P0 + ["regions", 0, "players"]),
    "biome name null": setp(P0 + ["biome", "name"], None),
    "event start time 7 digits": setp(EV + ["startTime"], "2026-10-17T12:34:56.9999999Z"),
    "event start time offset": setp(EV + ["startTime"], "2026-10-17T12:34:56.123+02:00"),
    "event start time unix seconds": setp(EV + ["startTime"], 1760000000),
    "event start time unix millis": setp(EV + ["startTime"], 1760000000123),
    "event start time garbage": setp(EV + ["startTime"], "not a date"),
    "event start time null": setp(EV + ["startTime"], None),
    "event faction null": setp(EV + ["faction"], None),
    "event joint ops null": setp(EV + ["jointOperationIds"], None),
    "event health string": setp(EV + ["health"], "12"),
    "campaign planet null": setp(["campaigns", 0, "planet"], None),
    "campaign count null": setp(["campaigns", 0, "count"], None),
    "assignment title string": setp(["assignments", 0, "title"], "MAJOR ORDER"),
    "assignment title null value": setp(["assignments", 0, "title", "de-DE"], None),
    "assignment briefing null": setp(["assignments", 0, "briefing"], None),
    "assignment tasks null": setp(["assignments", 0, "tasks"], None),
    "assignment reward null": setp(["assignments", 0, "reward"], None),
    "assignment progress null": setp(["assignments", 0, "progress"], None),
    "assignment expiration missing": delp(["assignments", 0, "expiration"]),
    "task values null": setp(["assignments", 0, "tasks", 0, "values"], None),
    "dispatch message null": setp(["dispatches", 0, "message"], None),
    "dispatch message string": setp(["dispatches", 0, "message"], "hello"),
    "dispatch published garbage": setp(["dispatches", 0, "published"], "soon"),
    "dispatch id null": setp(["dispatches", 0, "id"], None),
    "war statistics null": setp(["war", "statistics"], None),
    "war impact int": setp(["war", "impactMultiplier"], 0),
    "war factions null": setp(["war", "factions"], None),
    "war now null": setp(["war", "now"], None),
    "duplicate planet index": lambda d: d["planets"].append(copy.deepcopy(d["planets"][0])),
}


def run_case(doc):
    raw = json.dumps(doc, ensure_ascii=False)
    try:
        rec = v1.FullStatus.model_validate_json(raw)
    except ValidationError:
        return {"result": "invalid"}
    rec.snapshot_at = SNAPSHOT_AT
    try:
        status = v1_to_frontend(rec)
        out = status.model_dump_json()
    except Exception as exc:  # the Python script would crash here
        return {"result": "error", "python_error": type(exc).__name__}
    return {"result": "ok", "current_status": out}


def main():
    cases = []
    for name, fn in CASES.items():
        doc = mutate(fn)
        cases.append({"name": name, "input": json.dumps(doc, ensure_ascii=False), **run_case(doc)})
    with open(os.path.join(HERE, "cases.json"), "w") as fh:
        json.dump({"snapshot_at_ms": int(SNAPSHOT_AT.timestamp() * 1000), "cases": cases}, fh, ensure_ascii=False, indent=1)
    print(f"wrote {len(cases)} cases")


if __name__ == "__main__":
    main()
