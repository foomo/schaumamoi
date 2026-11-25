# The vector

The vector the shop ranks with is `affinity.Fade.Vector`, derived from the shopper's
choices (selections, views and carts; listings never enter the corpus). Recency is
counted in events, not time: a shopper who comes back after three weeks is not a
stranger. Three rules, applied in event order:

- every event fades everything before it by a factor (0.95: about 60% of the vector
  survives a visit);
- only the last 1000 events count at all, far more than the mock data reach;
- a strong change of intent forgets faster. When the last 3 choices point clearly away
  from the rest of the vector (cosine below 0.5, over the dimensions both name), the
  rest collapses to a fifth. Moving into a new category's own dimensions (clothes to
  cushions) is no change of intent and never triggers it.

`TestIntentPicksUpQuickly` checks this on long histories (a visit every two days, the
intent changing every six visits, every direction of change between the personas):

- **Strong change** to a taste that doesn't overlap the old one: the corpus reads it on
  the first visit, and the shop serves it by the second.
- **Gentle change** between overlapping tastes (Mira and Lena are both minimal and
  modern): the vector only fades into it, within three or four visits.

With the per-event fade alone, strong changes take as long as gentle ones. With no fade
at all, which was the shop's old behaviour, the shop stops following new intents once
the history is long enough. [The vector](/reports/affinity) report charts the fade and every collapse choice by
choice, and [Changing intent](/reports/affinity#intent) shows the test's histories visit
by visit.

