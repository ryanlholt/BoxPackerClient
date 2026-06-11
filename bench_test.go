package main

// Benchmarks for the pure packing computation, with the HTTP and JSON layers
// stripped out. This is the theoretical per-request ceiling: compare its
// timings against the latency k6 reports to attribute cost to compute vs.
// network/serialisation overhead.
//
//	go test -bench=BenchmarkPack -benchmem -benchtime=5s

import (
	"testing"
)

// benchBoxes mirrors the box catalog used by the k6 load test (loadtest/pack.js).
func benchBoxes() []BoxInput {
	return []BoxInput{
		{Reference: "small mailer", OuterWidth: 230, OuterLength: 300, OuterDepth: 240, EmptyWeight: 160, InnerWidth: 220, InnerLength: 290, InnerDepth: 230, MaxWeight: 15000},
		{Reference: "large mailer", OuterWidth: 370, OuterLength: 375, OuterDepth: 380, EmptyWeight: 410, InnerWidth: 360, InnerLength: 365, InnerDepth: 370, MaxWeight: 15000},
		{Reference: "xl box", OuterWidth: 500, OuterLength: 500, OuterDepth: 500, EmptyWeight: 800, InnerWidth: 490, InnerLength: 490, InnerDepth: 490, MaxWeight: 30000},
	}
}

// benchItemTemplates are the four item shapes the load test draws from.
func benchItemTemplates() []ItemInput {
	return []ItemInput{
		{Description: "mug", Width: 110, Length: 110, Depth: 105, Weight: 350, Rotation: "never"},
		{Description: "book", Width: 210, Length: 130, Depth: 30, Weight: 450, Rotation: "keepFlat"},
		{Description: "toy", Width: 80, Length: 60, Depth: 60, Weight: 150, Rotation: "best"},
		{Description: "cable", Width: 40, Length: 40, Depth: 120, Weight: 80, Rotation: "best"},
	}
}

// benchRequest builds a deterministic problem with the given number of item
// lines (cycling through the templates), each with quantity 3.
func benchRequest(lines int) *Request {
	templates := benchItemTemplates()
	items := make([]ItemInput, lines)
	for i := range items {
		it := templates[i%len(templates)]
		it.Quantity = 3
		items[i] = it
	}
	return &Request{
		Boxes:   benchBoxes(),
		Items:   items,
		Options: Options{AllowPartialResults: true},
	}
}

// BenchmarkPack measures Pack across the small/medium/large problem sizes that
// the load test weights its traffic toward. Per-request cost is dominated by
// item count, so each size is its own sub-benchmark.
func BenchmarkPack(b *testing.B) {
	sizes := []struct {
		name  string
		lines int
	}{
		{"small_3", 3},
		{"medium_15", 15},
		{"large_60", 60},
	}

	for _, s := range sizes {
		req := benchRequest(s.lines)
		b.Run(s.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				resp, err := Pack(req)
				if err != nil {
					b.Fatalf("Pack returned validation error: %v", err)
				}
				if resp.Error != "" {
					b.Fatalf("unexpected packing error: %s", resp.Error)
				}
			}
		})
	}
}

// BenchmarkPackParallel runs Pack concurrently to gauge how throughput scales
// with cores — the closest in-process analogue to the HTTP service under load.
// Set parallelism with -cpu, e.g. `go test -bench=Parallel -cpu=1,4,10`.
func BenchmarkPackParallel(b *testing.B) {
	req := benchRequest(15)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := Pack(req); err != nil {
				b.Fatalf("Pack returned validation error: %v", err)
			}
		}
	})
}
