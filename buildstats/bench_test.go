package main

import (
	"os"
	"testing"
)

func BenchmarkParseConvert(b *testing.B) {
	data, err := os.ReadFile("../801_full_v1.json")
	if err != nil {
		b.Skip("no snapshot at repo root")
	}
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		rec, err := ParseFullStatus(data)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := Convert(rec, 0); err != nil {
			b.Fatal(err)
		}
	}
}
