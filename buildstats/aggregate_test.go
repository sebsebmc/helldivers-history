package main

import (
	"bufio"
	"bytes"
	"testing"
)

func planet(index, players, health, maxHealth int64, owner string) planetSample {
	return planetSample{
		Index:   index,
		Players: players,
		planet:  FPlanet{Index: index, Health: health, MaxHealth: maxHealth, CurrentOwner: owner},
	}
}

func TestAggregate(t *testing.T) {
	const hour = 60 * 60 * 1000
	snaps := []Snapshot{
		// Newest first, as git log returns them.
		{Commit: "c", LogOrder: 0, At: 30 * hour, Impact: 0.5,
			Planets: []planetSample{planet(1, 10, 500, 1000, "Automaton"), planet(2, 20, 250, 1000, "Humans"), planet(3, 5, 1000, 1000, "Humans")},
			Events:  []eventSample{{PlanetIndex: 3, defense: FDefense{Health: 400, MaxHealth: 1000}}},
			Active:  []int64{2, 1, 2}},
		{Commit: "b", LogOrder: 1, At: 10 * hour, Impact: 0.25,
			Planets: []planetSample{planet(1, 7, 1000, 1000, "Automaton"), planet(2, 3, 1000, 1000, "Humans")}},
		{Commit: "a", LogOrder: 2, At: 1 * hour, Impact: 1,
			Planets: []planetSample{planet(1, 100, 0, 1000, "Automaton")}},
	}
	SortSnapshots(snaps)
	steps, ranked, err := Aggregate(snaps)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	WriteAggregates(w, steps)
	w.Flush()
	wantAgg := `[{"timestamp": 3600000, "players": 100, "impact": 1.0, "attacks": {"1": {"players": 100, "liberation": 100.0}}}, ` +
		`{"timestamp": 36000000, "players": 10, "impact": 0.25, "attacks": {"1": {"players": 7, "liberation": 0.0}, "2": {"players": 3, "liberation": 100.0}}}, ` +
		`{"timestamp": 108000000, "players": 35, "impact": 0.5, "attacks": {"1": {"players": 10, "liberation": 50.0}, "2": {"players": 20, "liberation": 25.0}, "3": {"players": 5, "liberation": 60.0}}}]`
	if got := buf.String(); got != wantAgg {
		t.Errorf("aggregates\n got: %s\nwant: %s", got, wantAgg)
	}

	// Only snapshots in the last 24h (at >= 6h) count towards the ranking.
	buf.Reset()
	WriteRecentAttacks(w, ranked)
	w.Flush()
	if got, want := buf.String(), "[[2, 23], [1, 17]]"; got != want {
		t.Errorf("recent attacks = %s, want %s", got, want)
	}
}

func TestSortSnapshotsKeepsGitLogOrderForTies(t *testing.T) {
	snaps := []Snapshot{{Commit: "new", LogOrder: 0, At: 5}, {Commit: "old", LogOrder: 1, At: 5}, {Commit: "first", LogOrder: 2, At: 1}}
	SortSnapshots(snaps)
	got := []string{snaps[0].Commit, snaps[1].Commit, snaps[2].Commit}
	if got[0] != "first" || got[1] != "new" || got[2] != "old" {
		t.Errorf("order = %v, want [first new old]", got)
	}
}

func TestAggregateZeroMaxHealthFails(t *testing.T) {
	snaps := []Snapshot{{Commit: "a", At: 1, Planets: []planetSample{planet(1, 1, 0, 0, "Humans")}, Active: []int64{1}}}
	if _, _, err := Aggregate(snaps); err == nil {
		t.Error("expected an error for max_health 0, Python raises ZeroDivisionError")
	}
}
