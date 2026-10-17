package main

// Scalar field types that mirror pydantic's "lax" validation mode.
//
// build_stats.py drops any snapshot that fails pydantic validation, so the Go
// port has to accept and reject the same inputs. Each type records whether the
// key was present and whether it was null, because pydantic treats "missing",
// "null" and "wrong type" differently depending on how the field is declared.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// presence is embedded in every scalar type.
type presence struct {
	Set  bool // the key appeared in the object (possibly as null)
	Null bool // the key appeared with a JSON null
}

// Has reports whether the field holds a usable (non-null) value.
func (p presence) Has() bool { return p.Set && !p.Null }

// checkRequired validates a field declared as `x: T` with no default.
func (p presence) checkRequired(name string) error {
	if !p.Set {
		return fmt.Errorf("%s: field required", name)
	}
	if p.Null {
		return fmt.Errorf("%s: input should not be null", name)
	}
	return nil
}

// checkDefaultNone validates a field declared as `x: T = None` (not Optional):
// pydantic skips validation of the default when the key is missing, but an
// explicit null still fails.
func (p presence) checkDefaultNone(name string) error {
	if p.Null {
		return fmt.Errorf("%s: input should not be null", name)
	}
	return nil
}

func isNull(b []byte) bool { return len(b) == 4 && string(b) == "null" }

// Int is a lax pydantic int.
type Int struct {
	presence
	V int64
}

func (x *Int) UnmarshalJSON(b []byte) error {
	x.Set = true
	if isNull(b) {
		x.Null = true
		return nil
	}
	v, err := laxInt(b)
	if err != nil {
		return err
	}
	x.V = v
	return nil
}

func laxInt(b []byte) (int64, error) {
	switch b[0] {
	case 't':
		return 1, nil
	case 'f':
		return 0, nil
	case '"':
		s, err := decodeString(b)
		if err != nil {
			return 0, err
		}
		return intFromNumber(strings.TrimSpace(s))
	case '{', '[':
		return 0, fmt.Errorf("input should be a valid integer, got %.20s", b)
	}
	return intFromNumber(string(b))
}

func intFromNumber(s string) (int64, error) {
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v, nil
	}
	// Floats are accepted when they have no fractional part (e.g. 5.0).
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) || f != math.Trunc(f) {
		return 0, fmt.Errorf("input should be a valid integer, got %q", s)
	}
	if f >= 9.223372036854775807e18 || f < -9.223372036854775808e18 {
		return 0, fmt.Errorf("integer out of range: %q", s)
	}
	return int64(f), nil
}

// Float is a lax pydantic float.
type Float struct {
	presence
	V float64
}

func (x *Float) UnmarshalJSON(b []byte) error {
	x.Set = true
	if isNull(b) {
		x.Null = true
		return nil
	}
	switch b[0] {
	case 't':
		x.V = 1
		return nil
	case 'f':
		x.V = 0
		return nil
	case '"':
		s, err := decodeString(b)
		if err != nil {
			return err
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return fmt.Errorf("input should be a valid number, got %q", s)
		}
		x.V = v
		return nil
	case '{', '[':
		return fmt.Errorf("input should be a valid number, got %.20s", b)
	}
	v, err := strconv.ParseFloat(string(b), 64)
	if err != nil {
		return fmt.Errorf("input should be a valid number, got %s", b)
	}
	x.V = v
	return nil
}

// Str is a pydantic str. Lax mode does not coerce numbers to strings.
type Str struct {
	presence
	V string
}

func (x *Str) UnmarshalJSON(b []byte) error {
	x.Set = true
	if isNull(b) {
		x.Null = true
		return nil
	}
	if b[0] != '"' {
		return fmt.Errorf("input should be a valid string, got %.20s", b)
	}
	s, err := decodeString(b)
	x.V = s
	return err
}

// decodeString decodes a JSON string literal, skipping the slow path when the
// literal has no escapes.
func decodeString(b []byte) (string, error) {
	inner := b[1 : len(b)-1]
	if bytes.IndexByte(inner, '\\') < 0 && utf8.Valid(inner) {
		return string(inner), nil
	}
	var s string
	err := json.Unmarshal(b, &s)
	return s, err
}

// Bool is a lax pydantic bool.
type Bool struct {
	presence
	V bool
}

func (x *Bool) UnmarshalJSON(b []byte) error {
	x.Set = true
	switch {
	case isNull(b):
		x.Null = true
		return nil
	case string(b) == "true":
		x.V = true
		return nil
	case string(b) == "false":
		x.V = false
		return nil
	case b[0] == '"':
		s, err := decodeString(b)
		if err != nil {
			return err
		}
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "1", "on", "t", "true", "y", "yes":
			x.V = true
			return nil
		case "0", "off", "f", "false", "n", "no":
			x.V = false
			return nil
		}
	case string(b) == "0" || string(b) == "0.0":
		x.V = false
		return nil
	case string(b) == "1" || string(b) == "1.0":
		x.V = true
		return nil
	}
	return fmt.Errorf("input should be a valid boolean, got %.20s", b)
}

// Time is a pydantic datetime. Values are kept at microsecond precision, the
// same as Python's datetime.
type Time struct {
	presence
	Micros int64 // microseconds since the Unix epoch
}

func (x *Time) UnmarshalJSON(b []byte) error {
	x.Set = true
	if isNull(b) {
		x.Null = true
		return nil
	}
	switch b[0] {
	case '"':
		s, err := decodeString(b)
		if err != nil {
			return err
		}
		x.Micros, err = parseDatetime(s)
		return err
	case '{', '[', 't', 'f':
		return fmt.Errorf("input should be a valid datetime, got %.20s", b)
	}
	f, err := strconv.ParseFloat(string(b), 64)
	if err != nil {
		return fmt.Errorf("input should be a valid datetime, got %s", b)
	}
	x.Micros = unixNumberToMicros(f)
	return nil
}

// UnixMillis reproduces Python's `int(dt.timestamp() * 1000)`, including its
// floating point rounding.
func (x Time) UnixMillis() int64 {
	return pyTimestampMillis(x.Micros)
}

func pyTimestampMillis(micros int64) int64 {
	ts := float64(micros) / 1e6
	return int64(ts * 1000)
}

// unixNumberToMicros follows pydantic: numbers above 2e10 are milliseconds.
func unixNumberToMicros(f float64) int64 {
	if math.Abs(f) > 2e10 {
		f /= 1000
	}
	sec := math.Floor(f)
	return int64(sec)*1_000_000 + int64(math.Round((f-sec)*1e6))
}

func parseDatetime(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return unixNumberToMicros(f), nil
	}
	if len(s) > 10 && (s[10] == ' ' || s[10] == 't') {
		s = s[:10] + "T" + s[11:]
	}
	if strings.HasSuffix(s, "z") {
		s = s[:len(s)-1] + "Z"
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		// Naive datetimes. CI runs in UTC, which is what Python would assume.
		var err2 error
		t, err2 = time.Parse("2006-01-02T15:04:05.999999999", s)
		if err2 != nil {
			return 0, fmt.Errorf("input should be a valid datetime: %w", err)
		}
	}
	// Python datetimes truncate to microseconds.
	return t.Unix()*1_000_000 + int64(t.Nanosecond()/1000), nil
}

// LangText is pydantic's `Union[Dict[str, Optional[str]], str]` used for
// translated strings. Key order is preserved so output matches Python's dicts.
type LangText struct {
	presence
	IsStr   bool
	Entries []LangEntry
}

type LangEntry struct {
	Key   string
	Value *string // nil for JSON null
}

func (x *LangText) UnmarshalJSON(b []byte) error {
	x.Set = true
	if isNull(b) {
		x.Null = true
		return nil
	}
	switch b[0] {
	case '"':
		s, err := decodeString(b)
		if err != nil {
			return err
		}
		x.IsStr = true
		x.Entries = []LangEntry{{Key: "en-US", Value: &s}}
		return nil
	case '{':
		return x.parseObject(b)
	}
	return fmt.Errorf("input should be a valid dictionary or string, got %.20s", b)
}

func (x *LangText) parseObject(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	if _, err := dec.Token(); err != nil { // {
		return err
	}
	index := map[string]int{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		key := tok.(string)
		tok, err = dec.Token()
		if err != nil {
			return err
		}
		var val *string
		switch v := tok.(type) {
		case nil:
		case string:
			val = &v
		default:
			return fmt.Errorf("%s: input should be a valid string", key)
		}
		// Duplicate keys keep their first position and last value, like dict.
		if i, ok := index[key]; ok {
			x.Entries[i].Value = val
			continue
		}
		index[key] = len(x.Entries)
		x.Entries = append(x.Entries, LangEntry{Key: key, Value: val})
	}
	return nil
}

// checkNoNullValues enforces `Dict[str, str]` (used for planet names).
func (x LangText) checkNoNullValues(name string) error {
	if x.IsStr {
		return nil
	}
	for _, e := range x.Entries {
		if e.Value == nil {
			return fmt.Errorf("%s.%s: input should be a valid string", name, e.Key)
		}
	}
	return nil
}
