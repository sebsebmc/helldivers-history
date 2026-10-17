package main

// Aggregation from create_agg_stats in build_stats.py.
//
// The Python version keeps every converted snapshot in memory. Only a handful
// of numbers per planet feed the aggregates, so each snapshot is reduced to a
// Snapshot as soon as it's converted.

import (
	"errors"
	"fmt"
	"sort"
)

// RecencyMillis is the window used to rank the most active planets. Time-based
// because GitHub's scheduler fires far less often than the configured 10
// minute cron.
const RecencyMillis = 24 * 60 * 60 * 1000

type planetSample struct {
	Index   int64
	Players int64
	planet  FPlanet // health/owner fields only, for Liberation
}

type eventSample struct {
	PlanetIndex int64
	defense     FDefense // health fields only, for Liberation
}

// Snapshot is the part of a converted record that the aggregates need.
type Snapshot struct {
	Commit   string
	LogOrder int   // position in `git log` output, newest first
	At       int64 // snapshot_at in ms
	Impact   float64
	Planets  []planetSample
	Events   []eventSample
	Active   []int64 // planet index of each campaign
}

func Summarize(commit string, logOrder int, s *CurrentStatus) (Snapshot, error) {
	snap := Snapshot{
		Commit:   commit,
		LogOrder: logOrder,
		At:       s.SnapshotAt,
		Impact:   s.War.ImpactMultiplier,
		Planets:  make([]planetSample, len(s.Planets)),
		Events:   make([]eventSample, len(s.Events)),
		Active:   make([]int64, len(s.Active)),
	}
	for i := range s.Planets {
		p := &s.Planets[i]
		if p.Statistics == nil {
			// status.statistics.player_count on None raises AttributeError.
			return Snapshot{}, fmt.Errorf("planet %d: statistics is None", p.Index)
		}
		snap.Planets[i] = planetSample{
			Index:   p.Index,
			Players: p.Statistics.PlayerCount,
			planet:  FPlanet{Index: p.Index, Health: p.Health, MaxHealth: p.MaxHealth, CurrentOwner: p.CurrentOwner},
		}
	}
	for i := range s.Events {
		d := &s.Events[i]
		snap.Events[i] = eventSample{
			PlanetIndex: d.Planet.Index,
			defense:     FDefense{ID: d.ID, Health: d.Health, MaxHealth: d.MaxHealth},
		}
	}
	for i := range s.Active {
		snap.Active[i] = s.Active[i].Planet.Index
	}
	return snap, nil
}

// SortSnapshots reproduces `sorted(records, key=snapshot_at)` applied to the
// newest-first git log order: ties keep git log order.
func SortSnapshots(snaps []Snapshot) {
	sort.SliceStable(snaps, func(i, j int) bool {
		if snaps[i].At != snaps[j].At {
			return snaps[i].At < snaps[j].At
		}
		return snaps[i].LogOrder < snaps[j].LogOrder
	})
}

type Attack struct {
	Index      int64
	Players    int64
	Liberation float64
}

type AggregateStep struct {
	Timestamp int64
	Players   int64
	Impact    float64
	Attacks   []Attack // ordered like the Python dict
}

type PlanetTotal struct {
	Index   int64
	Players int64
}

// Aggregate computes aggregates.json and recent_attacks.json from snapshots
// already sorted with SortSnapshots.
func Aggregate(snaps []Snapshot) ([]AggregateStep, []PlanetTotal, error) {
	if len(snaps) == 0 {
		return nil, nil, errors.New("no valid snapshots")
	}
	last := snaps[len(snaps)-1]

	active := map[int64]bool{}
	activeSum := map[int64]int64{}
	var activeOrder []int64
	for _, idx := range last.Active {
		if !active[idx] {
			active[idx] = true
			activeSum[idx] = 0
			activeOrder = append(activeOrder, idx)
		}
	}

	recentStart := last.At - RecencyMillis
	steps := make([]AggregateStep, len(snaps))
	for n := range snaps {
		snap := &snaps[n]
		step := AggregateStep{Timestamp: snap.At, Impact: snap.Impact}
		pos := map[int64]int{}
		set := func(a Attack) {
			if i, ok := pos[a.Index]; ok {
				step.Attacks[i] = a
				return
			}
			pos[a.Index] = len(step.Attacks)
			step.Attacks = append(step.Attacks, a)
		}
		players := map[int64]int64{}
		for i := range snap.Planets {
			p := &snap.Planets[i]
			players[p.Index] = p.Players
			step.Players += p.Players
			if !active[p.Index] {
				continue
			}
			lib, err := p.planet.Liberation()
			if err != nil {
				return nil, nil, fmt.Errorf("commit %s: %w", snap.Commit, err)
			}
			set(Attack{Index: p.Index, Players: p.Players, Liberation: lib})
			if snap.At >= recentStart {
				activeSum[p.Index] += p.Players
			}
		}
		for i := range snap.Events {
			ev := &snap.Events[i]
			pp, ok := players[ev.PlanetIndex]
			if !ok {
				return nil, nil, fmt.Errorf("commit %s: event planet %d missing", snap.Commit, ev.PlanetIndex)
			}
			lib, err := ev.defense.Liberation()
			if err != nil {
				return nil, nil, fmt.Errorf("commit %s: %w", snap.Commit, err)
			}
			set(Attack{Index: ev.PlanetIndex, Players: pp, Liberation: lib})
		}
		steps[n] = step
	}

	// Python iterates a set here, whose order isn't meaningful; ties are
	// broken by planet index so the output is deterministic.
	sort.Slice(activeOrder, func(i, j int) bool { return activeOrder[i] < activeOrder[j] })
	ranked := make([]PlanetTotal, len(activeOrder))
	for i, idx := range activeOrder {
		ranked[i] = PlanetTotal{Index: idx, Players: activeSum[idx]}
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].Players > ranked[j].Players })
	return steps, ranked, nil
}
