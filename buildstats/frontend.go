package main

// Frontend models (models/frontend.py) and the v1 -> frontend conversion
// (v1_to_frontend in build_stats.py).
//
// The Python conversion re-validates every value against the frontend models,
// and any failure there crashes the script. Convert returns an error in those
// cases instead.

import (
	"errors"
	"fmt"
)

type FStatistics struct {
	MissionsWon        *int64
	MissionsLost       *int64
	MissionTime        *int64
	TerminidKills      *int64
	AutomatonKills     *int64
	IlluminateKills    *int64
	BulletsFired       *int64
	BulletsHit         *int64
	TimePlayed         *int64
	Deaths             *int64
	Revives            *int64
	Friendlies         *int64
	MissionSuccessRate *int64
	Accuracy           *int64
	PlayerCount        int64
}

type FPlanet struct {
	X, Y           float64
	Index          int64
	Name           []LangEntry
	Sector         string
	Waypoints      []int64
	Disabled       bool
	RegenPerSecond float64
	CurrentOwner   string
	InitialOwner   string
	Health         int64
	MaxHealth      int64
	Statistics     *FStatistics
	Attacking      []int64 // nil means null
}

// Liberation is Planet.liberation from models/frontend.py.
func (p *FPlanet) Liberation() (float64, error) {
	if p.MaxHealth == 0 {
		return 0, fmt.Errorf("planet %d: division by zero computing liberation", p.Index)
	}
	pct := float64(p.Health) / float64(p.MaxHealth) * 100
	if p.CurrentOwner == "Humans" {
		return pct, nil
	}
	return 100.0 - pct, nil
}

type FDefense struct {
	ID                int64
	Faction           string
	Type              int64
	StartTime         int64
	EndTime           int64
	Health            int64
	MaxHealth         int64
	JointOperationIDs []int64
	Planet            FPlanet
}

// Liberation is Defense.liberation from models/frontend.py.
func (d *FDefense) Liberation() (float64, error) {
	if d.MaxHealth == 0 {
		return 0, fmt.Errorf("event %d: division by zero computing liberation", d.ID)
	}
	return 100.0 - (float64(d.Health) / float64(d.MaxHealth) * 100), nil
}

type FTask struct {
	Type       int64
	Values     []int64
	ValueTypes []int64
}

type FReward struct {
	Type   int64
	Amount int64
}

type FAssignment struct {
	ID          int64
	Title       []LangEntry
	Briefing    []LangEntry
	Description []LangEntry
	Tasks       []FTask
	Progress    []int64
	Expiration  int64
	Reward      *FReward
}

type FWarDetails struct {
	StartTime        int64
	EndTime          int64
	Now              int64
	Factions         []string
	ImpactMultiplier float64
	Statistics       *FStatistics
}

type FCampaign struct {
	ID     int64
	Planet FPlanet
	Type   int64
	Count  int64
}

type FDispatch struct {
	ID      int64
	Message []LangEntry
}

type CurrentStatus struct {
	Events      []FDefense
	Planets     []FPlanet // deduplicated by index, in first-seen order
	Assignments []FAssignment
	War         FWarDetails
	Active      []FCampaign
	Dispatches  []FDispatch
	SnapshotAt  int64
}

// PlanetByIndex mirrors `record.planets[index]`.
func (s *CurrentStatus) PlanetByIndex(index int64) (*FPlanet, bool) {
	for i := range s.Planets {
		if s.Planets[i].Index == index {
			return &s.Planets[i], true
		}
	}
	return nil, false
}

// Convert ports v1_to_frontend. snapshotMillis is the commit timestamp in ms.
func Convert(rec *FullStatus, snapshotMillis int64) (*CurrentStatus, error) {
	out := &CurrentStatus{SnapshotAt: snapshotMillis}

	planetPos := map[int64]int{}
	for i := range rec.Planets {
		p := &rec.Planets[i]
		fp, err := convertPlanet(p)
		if err != nil {
			return nil, prefixed(fmt.Sprintf("planets.%d", i), err)
		}
		// {p.index: p for p in planets}: first position, last value.
		if j, ok := planetPos[fp.Index]; ok {
			out.Planets[j] = fp
		} else {
			planetPos[fp.Index] = len(out.Planets)
			out.Planets = append(out.Planets, fp)
		}
		if p.Event != nil {
			d, err := convertEvent(p.Event, fp)
			if err != nil {
				return nil, prefixed(fmt.Sprintf("planets.%d.event", i), err)
			}
			out.Events = append(out.Events, d)
		}
	}

	for i := range rec.Assignments {
		a, err := convertAssignment(&rec.Assignments[i])
		if err != nil {
			return nil, prefixed(fmt.Sprintf("assignments.%d", i), err)
		}
		out.Assignments = append(out.Assignments, a)
	}

	war, err := convertWar(rec.War)
	if err != nil {
		return nil, prefixed("war", err)
	}
	out.War = war

	for i := range rec.Campaigns {
		c := &rec.Campaigns[i]
		fp, err := convertPlanet(c.Planet)
		if err != nil {
			return nil, prefixed(fmt.Sprintf("campaigns.%d.planet", i), err)
		}
		var fc FCampaign
		var e errs
		e.add(need(&fc.ID, c.ID, "id"))
		e.add(need(&fc.Type, c.Type, "type"))
		e.add(need(&fc.Count, c.Count, "count"))
		if err := e.join(); err != nil {
			return nil, prefixed(fmt.Sprintf("campaigns.%d", i), err)
		}
		fc.Planet = fp
		out.Active = append(out.Active, fc)
	}

	for i := range rec.Dispatches {
		d := &rec.Dispatches[i]
		if !d.Message.Has() {
			continue
		}
		var fd FDispatch
		if err := need(&fd.ID, d.ID, "id"); err != nil {
			return nil, prefixed(fmt.Sprintf("dispatches.%d", i), err)
		}
		fd.Message = d.Message.Entries
		out.Dispatches = append(out.Dispatches, fd)
	}

	return out, nil
}

func need(dst *int64, x Int, name string) error {
	if !x.Has() {
		return fmt.Errorf("%s: input should be a valid integer", name)
	}
	*dst = x.V
	return nil
}

func needList(name string, xs []Int) ([]int64, error) {
	if xs == nil {
		return nil, fmt.Errorf("%s: input should be a valid list", name)
	}
	return ints(xs), nil
}

func ints(xs []Int) []int64 {
	out := make([]int64, len(xs))
	for i, x := range xs {
		out[i] = x.V
	}
	return out
}

func convertStatistics(s *Statistics) (*FStatistics, error) {
	opt := func(x Int) *int64 {
		if !x.Has() {
			return nil
		}
		v := x.V
		return &v
	}
	if !s.PlayerCount.Has() {
		return nil, errors.New("statistics.player_count: input should be a valid integer")
	}
	return &FStatistics{
		MissionsWon:        opt(s.MissionsWon),
		MissionsLost:       opt(s.MissionsLost),
		MissionTime:        opt(s.MissionTime),
		TerminidKills:      opt(s.TerminidKills),
		AutomatonKills:     opt(s.AutomatonKills),
		IlluminateKills:    opt(s.IlluminateKills),
		BulletsFired:       opt(s.BulletsFired),
		BulletsHit:         opt(s.BulletsHit),
		TimePlayed:         opt(s.TimePlayed),
		Deaths:             opt(s.Deaths),
		Revives:            opt(s.Revives),
		Friendlies:         opt(s.Friendlies),
		MissionSuccessRate: opt(s.MissionSuccessRate),
		Accuracy:           opt(s.Accuracy),
		PlayerCount:        s.PlayerCount.V,
	}, nil
}

func convertPlanet(p *Planet) (FPlanet, error) {
	if !p.Position.X.Has() || !p.Position.Y.Has() {
		// `planet.position.x *= 100` on None raises TypeError.
		return FPlanet{}, errors.New("position: unsupported operand None")
	}
	fp := FPlanet{
		X:            p.Position.X.V * 100,
		Y:            p.Position.Y.V * 100,
		Index:        p.Index.V,
		Name:         p.Name.Entries,
		Health:       p.Health.V,
		MaxHealth:    p.MaxHealth.V,
		CurrentOwner: p.CurrentOwner.V,
	}
	var e errs
	if !p.Sector.Has() {
		e.add(errors.New("sector: input should be a valid string"))
	}
	if p.Waypoints == nil {
		e.add(errors.New("waypoints: input should be a valid list"))
	}
	if !p.Disabled.Has() {
		e.add(errors.New("disabled: input should be a valid boolean"))
	}
	if !p.RegenPerSecond.Has() {
		e.add(errors.New("regen_per_second: input should be a valid number"))
	}
	if !p.InitialOwner.Has() {
		e.add(errors.New("initial_owner: input should be a valid string"))
	}
	if p.Statistics != nil {
		st, err := convertStatistics(p.Statistics)
		e.add(err)
		fp.Statistics = st
	}
	if err := e.join(); err != nil {
		return FPlanet{}, err
	}
	fp.Sector = p.Sector.V
	fp.Waypoints = ints(p.Waypoints)
	fp.Disabled = p.Disabled.V
	fp.RegenPerSecond = p.RegenPerSecond.V
	fp.InitialOwner = p.InitialOwner.V
	if p.Attacking != nil {
		fp.Attacking = ints(p.Attacking)
	}
	return fp, nil
}

func convertEvent(ev *Event, planet FPlanet) (FDefense, error) {
	d := FDefense{Planet: planet}
	var e errs
	e.add(need(&d.ID, ev.ID, "id"))
	if !ev.Faction.Has() {
		e.add(errors.New("faction: input should be a valid string"))
	}
	e.add(need(&d.Type, ev.EventType, "type"))
	if !ev.StartTime.Has() || !ev.EndTime.Has() {
		e.add(errors.New("start_time/end_time: None has no attribute timestamp"))
	}
	e.add(need(&d.Health, ev.Health, "health"))
	e.add(need(&d.MaxHealth, ev.MaxHealth, "max_health"))
	jo, err := needList("joint_operation_ids", ev.JointOperationIDs)
	e.add(err)
	if err := e.join(); err != nil {
		return FDefense{}, err
	}
	d.Faction = ev.Faction.V
	d.StartTime = ev.StartTime.UnixMillis()
	d.EndTime = ev.EndTime.UnixMillis()
	d.JointOperationIDs = jo
	return d, nil
}

func convertAssignment(a *Assignment2) (FAssignment, error) {
	fa := FAssignment{
		ID:         a.ID.V,
		Title:      a.Title.Entries,
		Progress:   ints(a.Progress),
		Expiration: a.Expiration.UnixMillis(),
	}
	var e errs
	if !a.Briefing.Has() {
		e.add(errors.New("briefing: input should be a valid dictionary"))
	}
	if !a.Description.Has() {
		e.add(errors.New("description: input should be a valid dictionary"))
	}
	if a.Tasks == nil {
		e.add(errors.New("tasks: 'NoneType' object is not iterable"))
	}
	for i, t := range a.Tasks {
		var ft FTask
		var te errs
		te.add(need(&ft.Type, t.Type, "type"))
		v, err := needList("values", t.Values)
		te.add(err)
		vt, err := needList("value_types", t.ValueTypes)
		te.add(err)
		if err := te.join(); err != nil {
			e.add(prefixed(fmt.Sprintf("tasks.%d", i), err))
			continue
		}
		ft.Values, ft.ValueTypes = v, vt
		fa.Tasks = append(fa.Tasks, ft)
	}
	if a.Reward != nil {
		var r FReward
		e.add(prefixed("reward", need(&r.Type, a.Reward.Type, "type")))
		e.add(prefixed("reward", need(&r.Amount, a.Reward.Amount, "amount")))
		fa.Reward = &r
	}
	if err := e.join(); err != nil {
		return FAssignment{}, err
	}
	fa.Briefing = a.Briefing.Entries
	fa.Description = a.Description.Entries
	return fa, nil
}

func convertWar(w *War) (FWarDetails, error) {
	var e errs
	if !w.Started.Has() || !w.Ended.Has() || !w.Now.Has() {
		e.add(errors.New("started/ended/now: None has no attribute timestamp"))
	}
	if w.Factions == nil {
		e.add(errors.New("factions: input should be a valid list"))
	}
	if !w.ImpactMultiplier.Has() {
		e.add(errors.New("impact_multiplier: input should be a valid number"))
	}
	var stats *FStatistics
	if w.Statistics == nil {
		e.add(errors.New("statistics: None has no attribute model_dump"))
	} else {
		var err error
		stats, err = convertStatistics(w.Statistics)
		e.add(err)
	}
	if err := e.join(); err != nil {
		return FWarDetails{}, err
	}
	factions := make([]string, len(w.Factions))
	for i, f := range w.Factions {
		factions[i] = f.V
	}
	return FWarDetails{
		StartTime:        w.Started.UnixMillis(),
		EndTime:          w.Ended.UnixMillis(),
		Now:              w.Now.UnixMillis(),
		Factions:         factions,
		ImpactMultiplier: w.ImpactMultiplier.V,
		Statistics:       stats,
	}, nil
}
