package main

import (
	"math"
	"testing"
)

// Expected strings were produced by Python 3 (repr) and pydantic 2
// (model_dump_json) for the same values.
var point1, point2 = 0.1, 0.2

var floatCases = []struct {
	in         float64
	pyRepr     string
	pydanticJS string
}{
	{100.0, "100.0", "100.0"},
	{1e-5, "1e-05", "0.00001"},
	{0.0001, "0.0001", "0.0001"},
	{1e15, "1000000000000000.0", "1000000000000000.0"},
	{1e16, "1e+16", "1e16"},
	{1.5e16, "1.5e+16", "1.5e16"},
	{1e20, "1e+20", "1e20"},
	{1e22, "1e+22", "1e22"},
	{123456.789, "123456.789", "123456.789"},
	{4.1666665, "4.1666665", "4.1666665"},
	{point1 + point2, "0.30000000000000004", "0.30000000000000004"},
	{math.Copysign(0, -1), "-0.0", "-0.0"},
	{0, "0.0", "0.0"},
	{5e-324, "5e-324", "5e-324"},
	{1.7976931348623157e308, "1.7976931348623157e+308", "1.7976931348623157e308"},
	{12345678901234567.0, "1.2345678901234568e+16", "1.2345678901234568e16"},
	{0.00012345, "0.00012345", "0.00012345"},
	{1.2345e-7, "1.2345e-07", "1.2345e-7"},
	{-42.5, "-42.5", "-42.5"},
}

func TestPyFloatRepr(t *testing.T) {
	for _, c := range floatCases {
		if got := PyFloatRepr(c.in); got != c.pyRepr {
			t.Errorf("PyFloatRepr(%v) = %q, want %q", c.in, got, c.pyRepr)
		}
	}
}

func TestRyuFloat(t *testing.T) {
	for _, c := range floatCases {
		if got := RyuFloat(c.in); got != c.pydanticJS {
			t.Errorf("RyuFloat(%v) = %q, want %q", c.in, got, c.pydanticJS)
		}
	}
}

func TestTimestampMillis(t *testing.T) {
	// int(dt.timestamp()*1000) from Python for pydantic-parsed datetimes.
	cases := []struct {
		in   string
		want int64
	}{
		{"2024-03-01T12:34:56.0943119Z", 1709296496094},
		{"2024-03-01T12:34:56.9999999Z", 1709296496999},
		{"2024-03-01T12:34:56.123Z", 1709296496123},
		{"2024-03-01T12:34:56+02:00", 1709289296000},
		{"1700000000", 1700000000000},
	}
	for _, c := range cases {
		micros, err := parseDatetime(c.in)
		if err != nil {
			t.Errorf("parseDatetime(%q): %v", c.in, err)
			continue
		}
		if got := pyTimestampMillis(micros); got != c.want {
			t.Errorf("millis(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
