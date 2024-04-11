#!/usr/bin/env python3

import dataclasses
from typing import Dict, List, Optional, Union
import git
import json
import collections
import datetime
import subprocess
import os

from pydantic import RootModel, TypeAdapter, ValidationError
from models import frontend, v0, v1

PLANET_INDEXES = range(261)

CACHE_DIR = '_cache'
# Bump this with any changes to `fetch_all_records`
CACHE_VERSION = 2


def git_commits_for(path):
    return subprocess.check_output(['git', 'log', "--format=%H", path]).strip().decode().splitlines()

def git_show(ref, name, repo_client):
    commit_tree = repo_client.commit(ref).tree

    return commit_tree[name].data_stream.read()

def fetch_all_records_v0():
    commits = git_commits_for("helldivers.json")[:1440]

    repo = git.Repo('.', odbt=git.db.GitCmdObjectDB)

    out: List[v0.FullStatus] = []

    for ref in commits:
        cache_path = os.path.join(CACHE_DIR, ref[:2], ref[2:] + ".json")

        if os.path.exists(cache_path):
            with open(cache_path) as fh:
                try:
                    record = TypeAdapter(v0.FullStatus).validate_json(fh.read())
                except ValidationError as exc:
                    print(f"Bad cached data {exc}")
                    continue
                if record.version == CACHE_VERSION:
                    out.append(record)
                    continue
        try:
            record = TypeAdapter(v0.FullStatus).validate_json(git_show(ref, 'helldivers.json', repo))
        except ValidationError as exc:
            res = json.loads(git_show(ref, 'helldivers.json', repo))
            if 'error' in res.keys() or 'errors' in res.keys():
                continue
            print(f"Bad committed data {exc.errors()[0]}")
        timestamp = repo.commit(ref).committed_datetime.astimezone(datetime.timezone.utc)
        record.snapshot_at = timestamp
        record.version = CACHE_VERSION
        
        out.append(record)
            
        try:
            os.makedirs(os.path.dirname(cache_path))
        except FileExistsError:
            pass
        with open(cache_path, 'w') as fh:
            fh.write(RootModel[v0.FullStatus](record).model_dump_json())


    out.sort(key=lambda row: row.snapshot_at)
    return out

def fetch_all_records_v1():
    commits = git_commits_for("801_full_v1.json")[:1440]

    repo = git.Repo('.', odbt=git.db.GitCmdObjectDB)

    out: List[v1.FullStatus] = []

    for ref in commits:
        cache_path = os.path.join(CACHE_DIR, ref[:2], ref[2:] + ".json")

        if os.path.exists(cache_path):
            with open(cache_path) as fh:
                try:
                    record = v1.FullStatus.model_validate_json(fh.read())
                except ValidationError as exc:
                    print(f"Bad cached data {exc}")
                    continue
                if record.version == CACHE_VERSION:
                    out.append(record)
                    continue
        try:
            record = TypeAdapter(v0.FullStatus).validate_json(git_show(ref, '801_full_v1.json', repo))
        except ValidationError as exc:
            res = json.loads(git_show(ref, '801_full_v1.json', repo))
            if 'error' in res.keys() or 'errors' in res.keys():
                continue
            print(f"Bad committed data {exc.errors()[0]}")
        timestamp = repo.commit(ref).committed_datetime.astimezone(datetime.timezone.utc).isoformat()
        record.snapshot_at = timestamp
        record.version = CACHE_VERSION
        
        out.append(record)
            
        try:
            os.makedirs(os.path.dirname(cache_path))
        except FileExistsError:
            pass
        with open(cache_path, 'w') as fh:
            fh.write(record.model_dump_json())


    out.sort(key=lambda row: row.snapshot_at)
    return out

RECENCY = 6 * 24 

def create_agg_stats():
    records = [v0_to_frontend(rec) for rec in fetch_all_records_v0()] # + [v1_to_frontend(rec) for rec in fetch_all_records_v1()]
    players = [0]*len(records)
    timestamps = []
    impact = []
    active = set([campaign.planet.index for campaign in records[len(records)-1].active])
    active_sum = {p:0 for p in active}
    active_planet_hist = []


    recent_start = len(records) - (RECENCY)
    for (step, record) in enumerate(records):
        active_step = {}
        for status in record.planets:
            players[step] += status.statistics.player_count
            if status.index in active:
                active_step[status.index] = {'players': status.statistics.player_count, 'liberation': status.liberation}
                if step > recent_start:
                    active_sum[status.index] += status.statistics.player_count
        for event in record.events:
            planet = record.planets[event.planet.index]
            active_step[event.planet.index] = {'players': planet.statistics.player_count, 'liberation': event.liberation}
        active_planet_hist.append(active_step)

    most_active = sorted(active_sum.items(), key=lambda x: x[1], reverse=True)

    for step in records:
        timestamps.append(step.war.now)
        impact.append(step.war.impact_multiplier)

    with open('./docs/data/aggregates.json', 'w') as fh:
        json.dump([{'timestamp':v1, 'players': v2, 'impact': v3, 'attacks': v4} for v1, v2, v3, v4 in zip(timestamps, players, impact, active_planet_hist)], fh)
    with open('./docs/data/recent_attacks.json', 'w') as fh:
        json.dump(most_active, fh)
    with open('./docs/data/current_status.json', 'w') as fh:
        fh.write(records[-1].model_dump_json())

def v1_to_frontend(v1_rec: v1.FullStatus) -> frontend.CurrentStatus:
    planets = []
    events = []
    for planet in v1_rec.planets:
        planets.append(frontend.Planet.model_validate(planet.model_dump()))
        if planet.event is not None:
            events.append(frontend.Defense.model_validate({
                'id': planet.event.id,
                'faction':planet.event.faction,
                'type':planet.event.event_type,
                'start_time':int(planet.event.start_time.timestamp()*1000),
                'end_time':int(planet.event.end_time.timestamp()*1000),
                'health':planet.event.health,
                'max_health':planet.event.max_health,
                'joint_operation_ids': planet.event.joint_operation_ids,
                'planet': planet.model_dump(),
            }))

    assignments = []
    for assignment in v1_rec.assignments:
        assignments.append(frontend.Assignment.model_validate({
            'id': assignment.id,
            'title': {'en-us':assignment.title},
            'briefing': {'en-us':assignment.briefing},
            'description': {'en-us':assignment.description},
            'tasks': [task.model_dump() for task in assignment.tasks],
            'reward': assignment.reward.model_dump(),
        }))
    stats = v1_rec.war.statistics
    war_details = frontend.WarDetails.model_validate({
        'start_time': int(v1_rec.war.started.timestamp()*1000),
        'end_time': int(v1_rec.war.ended.timestamp()*1000),
        'now': int(v1_rec.war.now.timestamp()*1000),
        'factions': v1_rec.war.factions,
        'impact_multiplier': v1_rec.war.impact_multiplier,
        'statistics': frontend.Statistics.model_validate(v1_rec.war.statistics.model_dump()),
    })

    campaigns = []
    for campaign in v1_rec.campaigns:
        campaigns.append(frontend.Campaign.model_validate(campaign.model_dump()))

    return frontend.CurrentStatus.model_validate({
        'events': events,
        'planets': planets,
        'assignments': assignments,
        'war': war_details,
        'active': campaigns,
    })

def v0_to_frontend(v0_rec: v0.FullStatus) -> frontend.CurrentStatus:
    events = []
    planets: List[frontend.Planet] = []
    total_players = 0
    for planet_status in v0_rec.planet_status:
        planets.append(frontend.Planet.model_validate({
            'position': planet_status.planet.position,
            'index': planet_status.planet.index,
            'name': planet_status.planet.name,
            'sector': planet_status.planet.sector,
            'waypoints': planet_status.planet.waypoints,
            'disabled': planet_status.planet.disabled,
            'regen_per_second': planet_status.regen_per_second,
            'current_owner': planet_status.owner,
            'initial_owner': planet_status.planet.initial_owner,
            'health': planet_status.health,
            'max_health': planet_status.planet.max_health,
            'statistics': frontend.Statistics.model_validate({'player_count':planet_status.players}),
            'attacking': [target for (source, target) in v0_rec.planet_attacks if source == planet_status.planet.index]
        }))
        total_players += planet_status.players
    
    for event in v0_rec.planet_events:
        if event.event_type == 1:
            events.append(frontend.Defense.model_validate({
                'id':event.id,
                'faction':event.race,
                'type':event.event_type,
                'start_time':int(event.start_time.timestamp()*1000),
                'end_time':int(event.expire_time.timestamp()*1000),
                'health':event.health,
                'max_health':event.max_health,
                'joint_operation_ids':[j['id'] for j in event.joint_operations],
                'planet': next(filter(lambda p: p.index == event.planet.index, planets)),
            }))

    war_details = frontend.WarDetails.model_validate({
        'start_time': int(v0_rec.started_at.timestamp()*1000),
        'end_time': None,
        'now': int(v0_rec.snapshot_at.timestamp()*1000),
        'factions': ['Humans', 'Automatons', 'Terminids'],
        'impact_multiplier': v0_rec.impact_multiplier,
        'statistics': frontend.Statistics.model_validate({'player_count': total_players}),
    })

    campaigns = []
    for campaign in v0_rec.campaigns:
        campaigns.append(frontend.Campaign.model_validate({
            'count':campaign.count,
            'id':campaign.id,
            'planet':next(filter(lambda p: p.index == campaign.planet.index, planets)),
            'type':campaign.type,
        }))

    return frontend.CurrentStatus.model_validate({
        'events': events,
        'planets': planets,
        'assignments': [],
        'war': war_details,
        'active': campaigns,
        'dispatches':[dataclasses.asdict(d) for d in v0_rec.global_events],
    })

create_agg_stats()
# Plotting recent attacks based solely on player count is a bit boring sometimes. Maybe we should use variance of liberation?

# intial_owner's do change, such as when we lost the defense of Angel's Venture. We should keep track of these and add them to the message logs