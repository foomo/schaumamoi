// Package personalization is the space taste lives in: sparse vectors over named
// dimensions. One Vector type serves every role — a shopper's taste, what a product
// appeals to (its attractor), what a category looks at (its weights) — and its
// operations compare them: Score, WeightedScore and Cosine.
package personalization

type (
	// Dimension names one direction of taste: classic, cozy, festive …
	Dimension string

	// Value is how strongly a vector points along a dimension. Negative values point
	// away from it.
	Value float32

	// Vector is a sparse vector over dimensions: it names only the dimensions it cares
	// about, and a dimension it does not name counts as zero.
	Vector map[Dimension]Value
)
