package scanner_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/clarion-dev/clarion/internal/scanner"
)

// BenchmarkScanLargeRepo benchmarks scanning a synthetic 200k LOC repository
// (1000 files × 200 lines). The p99 latency must remain below 10 seconds.
func BenchmarkScanLargeRepo(b *testing.B) {
	dir := b.TempDir()
	if err := generateSyntheticRepo(dir, 1000, 200); err != nil {
		b.Fatalf("generate synthetic repo: %v", err)
	}

	s := scanner.New()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		iterStart := time.Now()
		if _, err := s.Scan(dir); err != nil {
			b.Fatal(err)
		}
		if elapsed := time.Since(iterStart); elapsed > 10*time.Second {
			b.Fatalf("scan exceeded 10s p99 limit: %v", elapsed)
		}
	}
}

// generateSyntheticRepo creates nFiles Go source files each with nLines lines in dir.
// Files are distributed across nFiles/10 packages (10 files each).
func generateSyntheticRepo(dir string, nFiles, nLines int) error {
	for i := 0; i < nFiles; i++ {
		pkgDir := filepath.Join(dir, fmt.Sprintf("pkg%d", i/10))
		if err := os.MkdirAll(pkgDir, 0o755); err != nil {
			return err
		}

		var sb strings.Builder
		pkgName := fmt.Sprintf("pkg%d", i/10)
		sb.WriteString(fmt.Sprintf("package %s\n\n", pkgName))
		for j := 0; j < nLines-1; j++ {
			sb.WriteString(fmt.Sprintf("// line%d in file%d\n", j, i))
		}

		fpath := filepath.Join(pkgDir, fmt.Sprintf("file%d.go", i))
		if err := os.WriteFile(fpath, []byte(sb.String()), 0o644); err != nil {
			return err
		}
	}
	return nil
}
