package mock

import (
	"github.com/foomo/schaumamoi/pkg/internal/mock/catalog"
	"github.com/foomo/schaumamoi/pkg/internal/mock/persona"
)

// Images returns every generated image keyed by its path relative to FixtureDir: one
// product shot per dress and one avatar per persona. Like Fixtures, it is shared by the
// generator and the golden test.
func Images() map[string]string {
	out := map[string]string{}
	for _, item := range catalog.All() {
		out[item.ImagePath()] = item.SVG()
	}

	for _, p := range persona.All() {
		out[p.ImagePath()] = p.SVG()
	}

	return out
}
