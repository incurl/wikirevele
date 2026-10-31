package etl

import (
	"context"
	"io"
	"os"
	"testing"
)

// BenchmarkPipeline measures the pass over a real slice of the dump.
//
// The file is the one the benchmark harness wrote; the test skips when
// it is not there.
func BenchmarkPipeline(b *testing.B) {
	const path = "/tmp/bench/slice.json"

	if _, err := os.Stat(path); err != nil {
		b.Skip("no slice to read")
	}

	b.SetBytes(fileSize(b, path))
	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		file, err := os.Open(path)
		if err != nil {
			b.Fatalf("open: %v", err)
		}

		pipeline := NewPipeline(Options{
			Targets: []*Target{
				NewTarget(ParisQID, "", "", NewEntityWriter(io.Discard), false),
			},
			Properties:      io.Discard,
			WriteProperties: true,
		})
		if _, err := pipeline.Run(context.Background(), file); err != nil {
			b.Fatalf("run: %v", err)
		}
		_ = file.Close()
	}
}

// fileSize is the size of a file.
func fileSize(b *testing.B, path string) int64 {
	b.Helper()
	info, err := os.Stat(path)
	if err != nil {
		b.Fatalf("stat: %v", err)
	}
	return info.Size()
}
