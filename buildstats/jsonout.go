package main

// Hand-rolled JSON writers that reproduce the bytes the Python script emits:
//
//   - current_status.json comes from pydantic's model_dump_json: compact
//     separators, raw UTF-8, and floats formatted like Rust's ryu.
//   - aggregates.json and recent_attacks.json come from json.dump: ", " and
//     ": " separators and floats formatted with Python's repr.

import (
	"bufio"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// shortestDigits returns the shortest round-trip decimal digits of |f| and
// the decimal point position, so |f| = 0.<digits> * 10^decpt.
func shortestDigits(f float64) (digits string, decpt int) {
	s := strconv.FormatFloat(math.Abs(f), 'e', -1, 64) // d.ddde±XX
	mant, exp, _ := strings.Cut(s, "e")
	e, _ := strconv.Atoi(exp)
	return strings.Replace(mant, ".", "", 1), e + 1
}

func sign(f float64) string {
	if math.Signbit(f) {
		return "-"
	}
	return ""
}

// PyFloatRepr formats f the way Python's float.__repr__ does.
func PyFloatRepr(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	}
	d, decpt := shortestDigits(f)
	var b strings.Builder
	b.WriteString(sign(f))
	switch {
	case decpt <= -4 || decpt > 16:
		b.WriteByte(d[0])
		if len(d) > 1 {
			b.WriteByte('.')
			b.WriteString(d[1:])
		}
		exp := decpt - 1
		if exp < 0 {
			fmt.Fprintf(&b, "e-%02d", -exp)
		} else {
			fmt.Fprintf(&b, "e+%02d", exp)
		}
	case decpt <= 0:
		b.WriteString("0.")
		b.WriteString(strings.Repeat("0", -decpt))
		b.WriteString(d)
	case decpt >= len(d):
		b.WriteString(d)
		b.WriteString(strings.Repeat("0", decpt-len(d)))
		b.WriteString(".0")
	default:
		b.WriteString(d[:decpt])
		b.WriteByte('.')
		b.WriteString(d[decpt:])
	}
	return b.String()
}

// RyuFloat formats f the way pydantic-core does (ryu's "pretty" format).
// Non-finite values serialize as null, pydantic's default.
func RyuFloat(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "null"
	}
	d, kk := shortestDigits(f)
	k := kk - len(d)
	var b strings.Builder
	b.WriteString(sign(f))
	switch {
	case k >= 0 && kk <= 16:
		b.WriteString(d)
		b.WriteString(strings.Repeat("0", k))
		b.WriteString(".0")
	case kk > 0 && kk <= 16:
		b.WriteString(d[:kk])
		b.WriteByte('.')
		b.WriteString(d[kk:])
	case kk > -5 && kk <= 0:
		b.WriteString("0.")
		b.WriteString(strings.Repeat("0", -kk))
		b.WriteString(d)
	default:
		b.WriteByte(d[0])
		if len(d) > 1 {
			b.WriteByte('.')
			b.WriteString(d[1:])
		}
		b.WriteByte('e')
		b.WriteString(strconv.Itoa(kk - 1))
	}
	return b.String()
}

const hexDigits = "0123456789abcdef"

// writeRustString writes s the way serde_json does: only quotes, backslashes
// and control characters are escaped.
func writeRustString(w *bufio.Writer, s string) {
	w.WriteByte('"')
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 0x20 && c != '"' && c != '\\' {
			continue
		}
		w.WriteString(s[start:i])
		switch c {
		case '"':
			w.WriteString(`\"`)
		case '\\':
			w.WriteString(`\\`)
		case '\n':
			w.WriteString(`\n`)
		case '\r':
			w.WriteString(`\r`)
		case '\t':
			w.WriteString(`\t`)
		case '\b':
			w.WriteString(`\b`)
		case '\f':
			w.WriteString(`\f`)
		default:
			w.WriteString(`\u00`)
			w.WriteByte(hexDigits[c>>4])
			w.WriteByte(hexDigits[c&0xf])
		}
		start = i + 1
	}
	w.WriteString(s[start:])
	w.WriteByte('"')
}

// pydanticWriter emits compact JSON for CurrentStatus.
type pydanticWriter struct {
	w   *bufio.Writer
	err error
}

func (p *pydanticWriter) raw(s string)    { p.w.WriteString(s) }
func (p *pydanticWriter) key(k string)    { writeRustString(p.w, k); p.w.WriteByte(':') }
func (p *pydanticWriter) str(s string)    { writeRustString(p.w, s) }
func (p *pydanticWriter) int(v int64)     { p.w.WriteString(strconv.FormatInt(v, 10)) }
func (p *pydanticWriter) float(v float64) { p.w.WriteString(RyuFloat(v)) }

func (p *pydanticWriter) bool(v bool) {
	if v {
		p.raw("true")
	} else {
		p.raw("false")
	}
}

func (p *pydanticWriter) optInt(v *int64) {
	if v == nil {
		p.raw("null")
		return
	}
	p.int(*v)
}

func (p *pydanticWriter) ints(xs []int64) {
	if xs == nil {
		p.raw("null")
		return
	}
	p.raw("[")
	for i, x := range xs {
		if i > 0 {
			p.raw(",")
		}
		p.int(x)
	}
	p.raw("]")
}

func (p *pydanticWriter) lang(entries []LangEntry) {
	p.raw("{")
	for i, e := range entries {
		if i > 0 {
			p.raw(",")
		}
		p.key(e.Key)
		if e.Value == nil {
			p.raw("null")
		} else {
			p.str(*e.Value)
		}
	}
	p.raw("}")
}

func (p *pydanticWriter) statistics(s *FStatistics) {
	if s == nil {
		p.raw("null")
		return
	}
	fields := []struct {
		name string
		v    *int64
	}{
		{"missions_won", s.MissionsWon},
		{"missions_lost", s.MissionsLost},
		{"mission_time", s.MissionTime},
		{"terminid_kills", s.TerminidKills},
		{"automaton_kills", s.AutomatonKills},
		{"illuminate_kills", s.IlluminateKills},
		{"bullets_fired", s.BulletsFired},
		{"bullets_hit", s.BulletsHit},
		{"time_played", s.TimePlayed},
		{"deaths", s.Deaths},
		{"revives", s.Revives},
		{"friendlies", s.Friendlies},
		{"mission_success_rate", s.MissionSuccessRate},
		{"accuracy", s.Accuracy},
	}
	p.raw("{")
	for _, f := range fields {
		p.key(f.name)
		p.optInt(f.v)
		p.raw(",")
	}
	p.key("player_count")
	p.int(s.PlayerCount)
	p.raw("}")
}

func (p *pydanticWriter) planet(pl *FPlanet) {
	p.raw(`{"position":{"x":`)
	p.float(pl.X)
	p.raw(`,"y":`)
	p.float(pl.Y)
	p.raw(`},"index":`)
	p.int(pl.Index)
	p.raw(`,"name":`)
	p.lang(pl.Name)
	p.raw(`,"sector":`)
	p.str(pl.Sector)
	p.raw(`,"waypoints":`)
	p.ints(pl.Waypoints)
	p.raw(`,"disabled":`)
	p.bool(pl.Disabled)
	p.raw(`,"regen_per_second":`)
	p.float(pl.RegenPerSecond)
	p.raw(`,"current_owner":`)
	p.str(pl.CurrentOwner)
	p.raw(`,"initial_owner":`)
	p.str(pl.InitialOwner)
	p.raw(`,"health":`)
	p.int(pl.Health)
	p.raw(`,"max_health":`)
	p.int(pl.MaxHealth)
	p.raw(`,"statistics":`)
	p.statistics(pl.Statistics)
	p.raw(`,"attacking":`)
	p.ints(pl.Attacking)
	p.raw(`,"liberation":`)
	lib, err := pl.Liberation()
	if err != nil && p.err == nil {
		p.err = err
	}
	p.float(lib)
	p.raw("}")
}

func (p *pydanticWriter) defense(d *FDefense) {
	p.raw(`{"id":`)
	p.int(d.ID)
	p.raw(`,"faction":`)
	p.str(d.Faction)
	p.raw(`,"type":`)
	p.int(d.Type)
	p.raw(`,"start_time":`)
	p.int(d.StartTime)
	p.raw(`,"end_time":`)
	p.int(d.EndTime)
	p.raw(`,"health":`)
	p.int(d.Health)
	p.raw(`,"max_health":`)
	p.int(d.MaxHealth)
	p.raw(`,"joint_operation_ids":`)
	p.ints(d.JointOperationIDs)
	p.raw(`,"planet":`)
	p.planet(&d.Planet)
	p.raw(`,"liberation":`)
	lib, err := d.Liberation()
	if err != nil && p.err == nil {
		p.err = err
	}
	p.float(lib)
	p.raw("}")
}

func (p *pydanticWriter) assignment(a *FAssignment) {
	p.raw(`{"id":`)
	p.int(a.ID)
	p.raw(`,"title":`)
	p.lang(a.Title)
	p.raw(`,"briefing":`)
	p.lang(a.Briefing)
	p.raw(`,"description":`)
	p.lang(a.Description)
	p.raw(`,"tasks":[`)
	for i, t := range a.Tasks {
		if i > 0 {
			p.raw(",")
		}
		p.raw(`{"type":`)
		p.int(t.Type)
		p.raw(`,"values":`)
		p.ints(t.Values)
		p.raw(`,"value_types":`)
		p.ints(t.ValueTypes)
		p.raw("}")
	}
	p.raw(`],"progress":`)
	p.ints(a.Progress)
	p.raw(`,"expiration":`)
	p.int(a.Expiration)
	p.raw(`,"reward":`)
	if a.Reward == nil {
		p.raw("null")
	} else {
		p.raw(`{"type":`)
		p.int(a.Reward.Type)
		p.raw(`,"amount":`)
		p.int(a.Reward.Amount)
		p.raw("}")
	}
	p.raw("}")
}

// WriteCurrentStatus writes s exactly as `records[-1].model_dump_json()`.
func WriteCurrentStatus(w *bufio.Writer, s *CurrentStatus) error {
	p := &pydanticWriter{w: w}
	p.raw(`{"events":[`)
	for i := range s.Events {
		if i > 0 {
			p.raw(",")
		}
		p.defense(&s.Events[i])
	}
	p.raw(`],"planets":{`)
	for i := range s.Planets {
		if i > 0 {
			p.raw(",")
		}
		p.key(strconv.FormatInt(s.Planets[i].Index, 10))
		p.planet(&s.Planets[i])
	}
	p.raw(`},"assignments":[`)
	for i := range s.Assignments {
		if i > 0 {
			p.raw(",")
		}
		p.assignment(&s.Assignments[i])
	}
	p.raw(`],"war":{"start_time":`)
	p.int(s.War.StartTime)
	p.raw(`,"end_time":`)
	p.int(s.War.EndTime)
	p.raw(`,"now":`)
	p.int(s.War.Now)
	p.raw(`,"factions":[`)
	for i, f := range s.War.Factions {
		if i > 0 {
			p.raw(",")
		}
		p.str(f)
	}
	p.raw(`],"impact_multiplier":`)
	p.float(s.War.ImpactMultiplier)
	p.raw(`,"statistics":`)
	p.statistics(s.War.Statistics)
	p.raw(`},"active":[`)
	for i := range s.Active {
		if i > 0 {
			p.raw(",")
		}
		c := &s.Active[i]
		p.raw(`{"id":`)
		p.int(c.ID)
		p.raw(`,"planet":`)
		p.planet(&c.Planet)
		p.raw(`,"type":`)
		p.int(c.Type)
		p.raw(`,"count":`)
		p.int(c.Count)
		p.raw("}")
	}
	p.raw(`],"dispatches":[`)
	for i := range s.Dispatches {
		if i > 0 {
			p.raw(",")
		}
		d := &s.Dispatches[i]
		p.raw(`{"id":`)
		p.int(d.ID)
		p.raw(`,"message":`)
		p.lang(d.Message)
		p.raw(`,"title":{"en-US":"Dispatch"},"faction":null}`)
	}
	p.raw(`],"snapshot_at":`)
	p.int(s.SnapshotAt)
	p.raw("}")
	return p.err
}

// WriteAggregates writes the aggregates.json list the way json.dump does.
func WriteAggregates(w *bufio.Writer, steps []AggregateStep) {
	w.WriteString("[")
	for i, s := range steps {
		if i > 0 {
			w.WriteString(", ")
		}
		fmt.Fprintf(w, `{"timestamp": %d, "players": %d, "impact": %s, "attacks": {`,
			s.Timestamp, s.Players, PyFloatRepr(s.Impact))
		for j, a := range s.Attacks {
			if j > 0 {
				w.WriteString(", ")
			}
			fmt.Fprintf(w, `"%d": {"players": %d, "liberation": %s}`,
				a.Index, a.Players, PyFloatRepr(a.Liberation))
		}
		w.WriteString("}}")
	}
	w.WriteString("]")
}

// WriteRecentAttacks writes the [[index, players], ...] list like json.dump.
func WriteRecentAttacks(w *bufio.Writer, ranked []PlanetTotal) {
	w.WriteString("[")
	for i, r := range ranked {
		if i > 0 {
			w.WriteString(", ")
		}
		fmt.Fprintf(w, "[%d, %d]", r.Index, r.Players)
	}
	w.WriteString("]")
}
