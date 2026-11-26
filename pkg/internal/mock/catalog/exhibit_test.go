package catalog_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/foomo/schaumamoi/pkg/internal/docs"
	"github.com/foomo/schaumamoi/pkg/internal/mock/catalog"
	"github.com/foomo/schaumamoi/pkg/personalization"
	"github.com/stretchr/testify/require"
)

type (
	// category is one category of the tree, with its products and the weights they are
	// scored with.
	category struct {
		Name    string
		Path    string
		Weights personalization.Vector
		Items   []catalog.Item
	}

	// department is a department of the tree: the base weights its categories start
	// from, and its categories.
	department struct {
		Name       string
		ID         string
		Count      int
		Weights    personalization.Vector
		Categories []category
	}

	// dimensions are the taste dimensions sorted by which department speaks them.
	dimensions struct {
		Shared, Fashion, Home []personalization.Dimension
	}
)

// TestExhibit writes the catalog exhibit: the whole category tree from All Products down,
// the dimensions each department speaks, the weights each department and category scores
// with, and every product with its facets, copy and attractor. It also writes the product
// images every page shows.
func TestExhibit(t *testing.T) {
	t.Parallel()

	var departments []department

	for _, name := range categories {
		var items []catalog.Item

		for _, item := range catalog.All() {
			if item.Category() == name {
				items = append(items, item)
			}
		}

		require.NotEmpty(t, items, name)

		parent := catalog.Parent(name)
		if len(departments) == 0 || departments[len(departments)-1].Name != parent {
			departments = append(departments, department{Name: parent, ID: slug(parent), Weights: catalog.DepartmentWeights(parent)})
		}

		d := &departments[len(departments)-1]
		d.Count += len(items)
		d.Categories = append(d.Categories, category{
			Name:    name,
			Path:    path(items[0]),
			Weights: catalog.CategoryWeights(name),
			Items:   items,
		})
	}

	for _, item := range catalog.All() {
		require.NoError(t, docs.Image(item.ImagePath(), []byte(item.SVG())))
	}

	require.NoError(t, docs.Exhibit("catalog", map[string]any{
		"Departments": departments,
		"Dimensions":  spoken(),
		"Count":       len(catalog.All()),
	}, nil))

	t.Logf("wrote %s", filepath.Join(docs.Dir(), docs.FragmentDir, "catalog.md"))
}

// spoken sorts the dimensions the products carry by department: those both departments'
// products carry, and those only fashion or only home decoration does.
func spoken() dimensions {
	in := func(items []catalog.Item) map[personalization.Dimension]bool {
		out := map[personalization.Dimension]bool{}
		for _, item := range items {
			for d := range item.Attractor {
				out[d] = true
			}
		}

		return out
	}

	fashion, home := in(catalog.Fashion()), in(catalog.HomeDecoration())

	var out dimensions

	for d := range fashion {
		if home[d] {
			out.Shared = append(out.Shared, d)
		} else {
			out.Fashion = append(out.Fashion, d)
		}
	}

	for d := range home {
		if !fashion[d] {
			out.Home = append(out.Home, d)
		}
	}

	slices.Sort(out.Shared)
	slices.Sort(out.Fashion)
	slices.Sort(out.Home)

	return out
}

// path is an item's GA4 category path, e.g. Women's Fashion > Dresses > Business > Midi.
func path(item catalog.Item) string {
	var levels []string

	for _, level := range []string{item.ItemCategory, item.ItemCategory2, item.ItemCategory3, item.ItemCategory4} {
		if level != "" {
			levels = append(levels, level)
		}
	}

	return strings.Join(levels, " > ")
}

// slug is a heading id for a name: lower case, words joined by dashes, as the shop's list
// ids are (Women's Fashion is womens-fashion).
func slug(name string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.ReplaceAll(name, "'", ""))), "-")
}
