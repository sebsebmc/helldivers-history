#!/usr/bin/env python3

import dataclasses
from typing import Dict, List, Optional
import git
import json
import collections
import datetime
import subprocess
import os

from pydantic import RootModel, TypeAdapter, ValidationError
from pydantic.dataclasses import dataclass

PLANET_INDEXES = range(261)

CACHE_DIR = '_cache'
# Bump this with any changes to `fetch_all_records`
CACHE_VERSION = 0

# TODO: Use the hash to memoize Planets, if the Hash does change we can track the histories then
@dataclass
class PlanetRecord:
    disabled: bool
    hash: int
    index: int
    initial_owner: str
    max_health: int
    name: str
    position: Dict[str, float]
    sector: str
    waypoints: List[int]

@dataclass
class PlanetStatus:
    # timestamp:
    health: int
    owner: str
    planet: PlanetRecord
    players: int
    regen_per_second: float
    _liberation: Optional[float] = None
    @property
    def liberation(self) -> float:
        if self._liberation is not None and self._liberation >= 0:
            return self._liberation
        if self.owner == "Humans":
            self._liberation = (self.health/self.planet.max_health)*100
        else:
            self._liberation = (1.0-(self.health/self.planet.max_health))*100
        return self._liberation
    
    @liberation.setter
    def liberation(self, val:float) -> None:
        self._liberation = val


@dataclass
class Campaign:
    count: int
    id: int
    planet: PlanetRecord
    type: int

@dataclass
class Attacks:
    source: PlanetRecord
    target: PlanetRecord

@dataclass
class FullStatus:
    campaigns: List[Campaign]
    impact_multiplier: float
    planet_attacks: List[Attacks]
    planet_status: List[PlanetStatus]
    # We'll just override this with the commit timestamp
    snapshot_at: str
    war_id: int
    version: Optional[int] = dataclasses.field(default=None)


def git_commits_for(path):
    return subprocess.check_output(['git', 'log', "--format=%H", path]).strip().decode().splitlines()

def git_show(ref, name, repo_client):
    commit_tree = repo_client.commit(ref).tree

    return commit_tree[name].data_stream.read()

def fetch_all_records():
    commits = git_commits_for("helldivers.json")

    repo = git.Repo('.', odbt=git.db.GitCmdObjectDB)

    out: List[FullStatus] = []

    for ref in commits:
        cache_path = os.path.join(CACHE_DIR, ref[:2], ref[2:] + ".json")

        if os.path.exists(cache_path):
            with open(cache_path) as fh:
                try:
                    record = TypeAdapter(FullStatus).validate_json(fh.read())
                except ValidationError as exc:
                    print(f"Bad cached data {exc}")
                    continue
                if record.version == CACHE_VERSION:
                    out.append(record)
                    continue
        try:
            record = TypeAdapter(FullStatus).validate_json(git_show(ref, 'helldivers.json', repo))
        except ValidationError as exc:
            continue
        timestamp = repo.commit(ref).committed_datetime.isoformat()
        record.snapshot_at = timestamp
        record.version = CACHE_VERSION
        
        out.append(record)
            
        try:
            os.makedirs(os.path.dirname(cache_path))
        except FileExistsError:
            pass
        with open(cache_path, 'w') as fh:
            fh.write(RootModel[FullStatus](record).model_dump_json())


    out.sort(key=lambda row: row.snapshot_at)
    return out

RECENCY = 6 * 24 

def create_agg_stats():
    records = fetch_all_records()
    players = [0]*len(records)
    timestamps = []
    impact = []
    active = [planet.target.index for planet in records[len(records)-1].planet_attacks]
    active_sum = {p:0 for p in active}
    active_planet_hist = []


    recent_start = len(records) - (RECENCY)
    for (step, record) in enumerate(records):
        active_step = {}
        for status in record.planet_status:
            players[step] += status.players
            if status.planet.index in active:
                active_step[status.planet.index] = {'players': status.players, 'liberation': status.liberation}
                if step > recent_start:
                    active_sum[status.planet.index] += status.players
        active_planet_hist.append(active_step)

    most_active = sorted(active_sum.items(), key=lambda x: x[1], reverse=True)

    for step in records:
        timestamps.append(step.snapshot_at)
        impact.append(step.impact_multiplier)

    with open('./docs/data/aggregates.json', 'w') as fh:
        json.dump([{'timestamp':v1, 'players': v2, 'impact': v3, 'attacks': v4} for v1, v2, v3, v4 in zip(timestamps, players, impact, active_planet_hist)], fh)
    with open('./docs/data/recent_attacks.json', 'w') as fh:
        json.dump(most_active, fh)


create_agg_stats()

# TODO:what is the best way to chart the data we care about? Planet positions we should just fetch() for live results
# (and then fallback if we err). Planet liberation we can chart. Player count can be a simple stat

# NB: Turns out planet positions got updated to use values that are [-100,100] from [-1,1] so we probably should
# use the current values from the API and ignore tracking planet positions long term

# We should track campaigns by id and then create small graphs of the current campaigns
# in each step
#   look for campaigns, if new id, set its start time
#   if the id is missing the campaign ended
#   for each ongoing campaign 
#      use the planet status to graph players and planet health/liberation

# Attacks would be good for plotting on the map as arrows
# Campaigns seems to list currently active planets players can choose from