// Command gen writes the mock payloads out as MPv2 wire-format JSON fixtures, and the
// catalog and persona images out as SVG.
//
// The mocks are deterministic, so regenerating is a no-op unless a mock actually
// changed. Run it via `go generate ./...` from the repository root.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/foomo/schaumamoi/pkg/internal/mock"
	"github.com/foomo/schaumamoi/pkg/internal/mock/catalog"
)

func main() {
	out := flag.String("out", mock.FixtureDir, "directory to write the fixtures to")
	flag.Parse()

	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	fixtures := mock.Fixtures()

	for _, name := range mock.FixtureNames() {
		body, err := json.MarshalIndent(fixtures[name], "", "  ")
		if err != nil {
			return fmt.Errorf("marshal %s: %w", name, err)
		}

		if err := os.WriteFile(filepath.Join(dir, name), append(body, '\n'), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	if err := os.MkdirAll(filepath.Join(dir, catalog.ImageDir), 0o755); err != nil {
		return err
	}

	for name, svg := range mock.Images() {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(svg), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	return nil
}
