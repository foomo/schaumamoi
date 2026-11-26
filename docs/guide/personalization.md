# Personalization

`pkg/personalization` is a sparse vector space over taste dimensions:

```go
type (
    Dimension string
    Value     float32
    Vector    map[Dimension]Value
)

func (v Vector) Score(other Vector) Value                   // sparse dot product
func (v Vector) WeightedScore(other, weights Vector) Value  // … seen through a context
func (v Vector) Cosine(other Vector) Value                  // similarity, regardless of magnitude
```

One `Vector` type serves every role: a shopper's taste, what a product appeals to (its
attractor), and what a category looks at (its weights).

A vector only names the dimensions it cares about; a dimension missing from either side
contributes nothing. `WeightedScore` adds a third vector, the weights of a context: each
shared dimension counts `weights[d]` times, and a dimension the weights don't name
counts for nothing. Both sum in sorted dimension order: float addition is not
associative and Go randomises map iteration, so summing in map order would wobble in the
last bits and flip the ranking of near-tied products.


## Weights per category

How much each part of the vector counts is defined per category in the test shop's catalog
(`pkg/internal/mock/catalog/weights.go`, see [Catalog](/guide/test-shop/catalog#weights-and-score)).
Each department states what it looks at: fashion leads with the occasion, home with how
a room should feel (cozy, natural, industrial). Each category overrides only where it
differs. Dresses weigh femininity and romance up and tailoring down, trousers the
reverse. Cushions are about comfort and texture, lamps about industrial, modern or
classic design, vases about colour, material and statement. Every product is scored
with its own category's weights (`catalog.Score`): by the shop when it ranks, by the
personas when they choose, and by the distillery's recipes when they read a category. A category's
weights are a complete statement (anything they don't name counts for nothing, and a
test keeps every dimension its products carry named). They're scaled to average 1, so
they set emphasis rather than scale, and a listing that mixes categories still ranks on
one scale.

| Category | Leads with |
|---|---|
| Dresses | occasion · feminine · romantic |
| Trousers | occasion · structured · edgy |
| Cushions | cozy · natural |
| Lamps | industrial · modern · classic |
| Vases | colourful · natural · bold |

