package main

// Input models for 801_full_v1.json, ported from models/v1.py.
//
// The pydantic models use extra='forbid' on most classes, which means a
// snapshot is rejected when the API adds a field. To match, documents are
// decoded with DisallowUnknownFields, and the classes that use the default
// extra='ignore' (Event, Biome, Hazard, Campaign2) implement UnmarshalJSON
// with a non-strict decoder.
//
// Go's decoder can't express "required" or "not null", so each struct has a
// validate method that applies the field rules from models/v1.py.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

type FullStatus struct {
	War         *War          `json:"war"`
	Planets     []Planet      `json:"planets"`
	Assignments []Assignment2 `json:"assignments"`
	Campaigns   []Campaign2   `json:"campaigns"`
	Dispatches  []Dispatch    `json:"dispatches"`
	SnapshotAt  Time          `json:"snapshot_at"`
	Version     Int           `json:"version"`
}

type War struct {
	Started          Time        `json:"started"`
	Ended            Time        `json:"ended"`
	Now              Time        `json:"now"`
	ClientVersion    Str         `json:"clientVersion"`
	Factions         []Str       `json:"factions"`
	ImpactMultiplier Float       `json:"impactMultiplier"`
	Statistics       *Statistics `json:"statistics"`
}

type Statistics struct {
	MissionsWon        Int `json:"missionsWon"`
	MissionsLost       Int `json:"missionsLost"`
	MissionTime        Int `json:"missionTime"`
	TerminidKills      Int `json:"terminidKills"`
	AutomatonKills     Int `json:"automatonKills"`
	IlluminateKills    Int `json:"illuminateKills"`
	BulletsFired       Int `json:"bulletsFired"`
	BulletsHit         Int `json:"bulletsHit"`
	TimePlayed         Int `json:"timePlayed"`
	Deaths             Int `json:"deaths"`
	Revives            Int `json:"revives"`
	Friendlies         Int `json:"friendlies"`
	MissionSuccessRate Int `json:"missionSuccessRate"`
	Accuracy           Int `json:"accuracy"`
	PlayerCount        Int `json:"playerCount"`
}

type Position struct {
	X Float `json:"x"`
	Y Float `json:"y"`
}

type Planet struct {
	Index          Int         `json:"index"`
	Name           LangText    `json:"name"`
	Sector         Str         `json:"sector"`
	Hash           Int         `json:"hash"`
	Position       *Position   `json:"position"`
	Waypoints      []Int       `json:"waypoints"`
	MaxHealth      Int         `json:"maxHealth"`
	Health         Int         `json:"health"`
	Disabled       Bool        `json:"disabled"`
	InitialOwner   Str         `json:"initialOwner"`
	CurrentOwner   Str         `json:"currentOwner"`
	RegenPerSecond Float       `json:"regenPerSecond"`
	Event          *Event      `json:"event"`
	Statistics     *Statistics `json:"statistics"`
	Attacking      []Int       `json:"attacking"`
	Biome          *Biome      `json:"biome"`
	Hazards        []Hazard    `json:"hazards"`
	Regions        []Region    `json:"regions"`
}

// Event uses extra='ignore'.
type Event struct {
	ID                Int   `json:"id"`
	EventType         Int   `json:"eventType"`
	Faction           Str   `json:"faction"`
	Health            Int   `json:"health"`
	MaxHealth         Int   `json:"maxHealth"`
	StartTime         Time  `json:"startTime"`
	EndTime           Time  `json:"endTime"`
	CampaignID        Int   `json:"campaignId"`
	JointOperationIDs []Int `json:"jointOperationIds"`
}

func (e *Event) UnmarshalJSON(b []byte) error {
	type plain Event
	return json.Unmarshal(b, (*plain)(e))
}

// Biome uses extra='ignore'.
type Biome struct {
	Name        Str `json:"name"`
	Description Str `json:"description"`
}

func (x *Biome) UnmarshalJSON(b []byte) error {
	type plain Biome
	return json.Unmarshal(b, (*plain)(x))
}

// Hazard uses extra='ignore'.
type Hazard struct {
	Name        Str `json:"name"`
	Description Str `json:"description"`
}

func (x *Hazard) UnmarshalJSON(b []byte) error {
	type plain Hazard
	return json.Unmarshal(b, (*plain)(x))
}

type Region struct {
	ID                 Int   `json:"id"`
	Hash               Int   `json:"hash"`
	Name               Str   `json:"name"`
	Description        Str   `json:"description"`
	Health             Int   `json:"health"`
	MaxHealth          Int   `json:"maxHealth"`
	Size               Str   `json:"size"`
	RegenPerSecond     Float `json:"regenPerSecond"`
	AvailabilityFactor Float `json:"availabilityFactor"`
	IsAvailable        Bool  `json:"isAvailable"`
	Players            Int   `json:"players"`
}

type Task2 struct {
	Type       Int   `json:"type"`
	Values     []Int `json:"values"`
	ValueTypes []Int `json:"valueTypes"`
}

type Reward2 struct {
	Type   Int `json:"type"`
	Amount Int `json:"amount"`
}

type Assignment2 struct {
	ID          Int       `json:"id"`
	Title       LangText  `json:"title"`
	Briefing    LangText  `json:"briefing"`
	Description LangText  `json:"description"`
	Tasks       []Task2   `json:"tasks"`
	Reward      *Reward2  `json:"reward"`
	Rewards     []Reward2 `json:"rewards"`
	Progress    []Int     `json:"progress"`
	Expiration  Time      `json:"expiration"`
	Flags       Int       `json:"flags"`
}

// Campaign2 uses extra='ignore', but its nested Planet forbids extras, so the
// planet is captured raw and decoded strictly.
type Campaign2 struct {
	ID        Int
	Planet    *Planet
	Type      Int
	Count     Int
	Attacking []Int
	Faction   Str
}

func (c *Campaign2) UnmarshalJSON(b []byte) error {
	var raw struct {
		ID        Int             `json:"id"`
		Planet    json.RawMessage `json:"planet"`
		Type      Int             `json:"type"`
		Count     Int             `json:"count"`
		Attacking []Int           `json:"attacking"`
		Faction   Str             `json:"faction"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*c = Campaign2{ID: raw.ID, Type: raw.Type, Count: raw.Count, Attacking: raw.Attacking, Faction: raw.Faction}
	if raw.Planet == nil || isNull(raw.Planet) {
		return nil // reported by validate
	}
	c.Planet = &Planet{}
	return decodeStrict(raw.Planet, c.Planet)
}

type Dispatch struct {
	ID        Int      `json:"id"`
	Published Time     `json:"published"`
	Type      Int      `json:"type"`
	Message   LangText `json:"message"`
}

func decodeStrict(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("trailing data after JSON document")
	}
	return nil
}

// ParseFullStatus decodes and validates one snapshot the way
// `v1.FullStatus.model_validate_json` does.
func ParseFullStatus(b []byte) (*FullStatus, error) {
	// FullStatus itself uses extra='ignore', so only its fields are strict.
	var top struct {
		War         json.RawMessage `json:"war"`
		Planets     json.RawMessage `json:"planets"`
		Assignments json.RawMessage `json:"assignments"`
		Campaigns   json.RawMessage `json:"campaigns"`
		Dispatches  json.RawMessage `json:"dispatches"`
		SnapshotAt  Time            `json:"snapshot_at"`
		Version     Int             `json:"version"`
	}
	if err := json.Unmarshal(b, &top); err != nil {
		return nil, err
	}
	fs := FullStatus{SnapshotAt: top.SnapshotAt, Version: top.Version}
	for _, f := range []struct {
		raw json.RawMessage
		dst any
	}{
		{top.War, &fs.War},
		{top.Planets, &fs.Planets},
		{top.Assignments, &fs.Assignments},
		{top.Campaigns, &fs.Campaigns},
		{top.Dispatches, &fs.Dispatches},
	} {
		if f.raw == nil {
			continue // reported by validate
		}
		if err := decodeStrict(f.raw, f.dst); err != nil {
			return nil, err
		}
	}
	if err := fs.validate(); err != nil {
		return nil, err
	}
	return &fs, nil
}

// --- validation -----------------------------------------------------------

type errs []error

func (e *errs) add(err error) {
	if err != nil {
		*e = append(*e, err)
	}
}

func (e errs) join() error { return errors.Join(e...) }

func prefixed(prefix string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s.%w", prefix, err)
}

func requireIntList(name string, xs []Int) error {
	if xs == nil {
		return fmt.Errorf("%s: input should be a valid list", name)
	}
	return optionalIntList(name, xs)
}

func optionalIntList(name string, xs []Int) error {
	for i, x := range xs {
		if x.Null {
			return fmt.Errorf("%s.%d: input should be a valid integer", name, i)
		}
	}
	return nil
}

func (fs *FullStatus) validate() error {
	var e errs
	if fs.War == nil {
		e.add(errors.New("war: input should be an object"))
	} else {
		e.add(prefixed("war", fs.War.validate()))
	}
	if fs.Planets == nil {
		e.add(errors.New("planets: input should be a valid array"))
	}
	for i := range fs.Planets {
		e.add(prefixed(fmt.Sprintf("planets.%d", i), fs.Planets[i].validate()))
	}
	if fs.Assignments == nil {
		e.add(errors.New("assignments: input should be a valid array"))
	}
	for i := range fs.Assignments {
		e.add(prefixed(fmt.Sprintf("assignments.%d", i), fs.Assignments[i].validate()))
	}
	if fs.Campaigns == nil {
		e.add(errors.New("campaigns: input should be a valid array"))
	}
	for i := range fs.Campaigns {
		e.add(prefixed(fmt.Sprintf("campaigns.%d", i), fs.Campaigns[i].validate()))
	}
	if fs.Dispatches == nil {
		e.add(errors.New("dispatches: input should be a valid array"))
	}
	return e.join()
}

func (w *War) validate() error {
	for i, f := range w.Factions {
		if f.Null {
			return fmt.Errorf("factions.%d: input should be a valid string", i)
		}
	}
	return nil
}

func (p *Planet) validate() error {
	var e errs
	e.add(p.Index.checkRequired("index"))
	e.add(p.Name.checkRequired("name"))
	e.add(p.Name.checkNoNullValues("name"))
	if p.Position == nil {
		e.add(errors.New("position: input should be an object"))
	}
	e.add(optionalIntList("waypoints", p.Waypoints))
	e.add(p.MaxHealth.checkRequired("maxHealth"))
	e.add(p.Health.checkRequired("health"))
	e.add(p.CurrentOwner.checkRequired("currentOwner"))
	e.add(p.RegenPerSecond.checkDefaultNone("regenPerSecond"))
	if p.Event != nil {
		e.add(prefixed("event", optionalIntList("jointOperationIds", p.Event.JointOperationIDs)))
	}
	e.add(optionalIntList("attacking", p.Attacking))
	if p.Biome != nil {
		e.add(prefixed("biome", p.Biome.Name.checkRequired("name")))
		e.add(prefixed("biome", p.Biome.Description.checkRequired("description")))
	}
	for i, h := range p.Hazards {
		e.add(prefixed(fmt.Sprintf("hazards.%d", i), h.Name.checkRequired("name")))
		e.add(prefixed(fmt.Sprintf("hazards.%d", i), h.Description.checkRequired("description")))
	}
	for i := range p.Regions {
		e.add(prefixed(fmt.Sprintf("regions.%d", i), p.Regions[i].validate()))
	}
	return e.join()
}

var regionSizes = map[string]bool{"Settlement": true, "Town": true, "City": true, "MegaCity": true}

func (r *Region) validate() error {
	var e errs
	e.add(r.MaxHealth.checkDefaultNone("maxHealth"))
	e.add(r.Size.checkRequired("size"))
	if r.Size.Has() && !regionSizes[r.Size.V] {
		e.add(fmt.Errorf("size: input should be 'Settlement', 'Town', 'City' or 'MegaCity', got %q", r.Size.V))
	}
	e.add(r.IsAvailable.checkDefaultNone("isAvailable"))
	e.add(r.Players.checkRequired("players"))
	return e.join()
}

func (a *Assignment2) validate() error {
	var e errs
	e.add(a.ID.checkRequired("id"))
	e.add(a.Title.checkRequired("title"))
	for i, t := range a.Tasks {
		e.add(prefixed(fmt.Sprintf("tasks.%d", i), optionalIntList("values", t.Values)))
		e.add(prefixed(fmt.Sprintf("tasks.%d", i), optionalIntList("valueTypes", t.ValueTypes)))
	}
	e.add(requireIntList("progress", a.Progress))
	e.add(a.Expiration.checkRequired("expiration"))
	return e.join()
}

func (c *Campaign2) validate() error {
	if c.Planet == nil {
		return errors.New("planet: input should be an object")
	}
	return errors.Join(
		prefixed("planet", c.Planet.validate()),
		optionalIntList("attacking", c.Attacking),
	)
}
