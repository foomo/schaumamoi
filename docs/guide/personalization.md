# Personalization

`pkg/personalization` is a sparse vector space over taste dimensions:

```go
type (
    Dimension string
    Value     float32
    Map       map[Dimension]Value
)

func (m Map) Score(other Map) Value                        // sparse dot product
func (m Map) WeightedScore(other, weights Map) Value       // … seen through a context
```

A map only names the dimensions it cares about; a dimension missing from either side
contributes nothing. `WeightedScore` adds a third map, the weights of a context: each
shared dimension counts `weights[d]` times, and a dimension the weights don't name
counts for nothing. Both sum in sorted dimension order: float addition is not
associative and Go randomises map iteration, so summing in map order would wobble in the
last bits and flip the ranking of near-tied products.


## Weights per category

How much each part of the vector counts is defined per category (`pkg/mock/weights.go`).
Each department states what it looks at: fashion leads with the occasion, home with how
a room should feel (cozy, natural, industrial). Each category overrides only where it
differs. Dresses weigh femininity and romance up and tailoring down, trousers the
reverse. Cushions are about comfort and texture, lamps about industrial, modern or
classic design, vases about colour, material and statement. Every product is scored
with its own category's weights (`mock.Score`): by the shop when it ranks, by the
personas when they choose, and by the corpus when it reads a category. A category's
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

