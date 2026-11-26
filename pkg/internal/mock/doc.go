// Package mock is the shop the tests run on: deterministic GA4 traffic from a shop with
// two departments, women's fashion and home decoration. It is internal: test fixtures
// for this module, free to change, not an API. The exhibits the tests write use it to
// show the module's packages at work.
//
// The shop is split the way a real one is:
//
//   - taste: the dimensions taste is described in.
//   - catalog: what the shop sells, each product with an attractor vector in those
//     dimensions, and the per-category weights every product is scored with.
//   - persona: the four shoppers' tastes, and how a shopper picks from a listing.
//   - shop: what the shop does: listings, filtered and ranked by what it has learned,
//     and the GA4 events it reports. The shop never sees a persona, and it does not
//     import the persona package.
//   - shopper: a persona going through the shop: journeys and histories, with every
//     event fed into an affinity still whose vector ranks the next listing.
//   - wire: the Measurement Protocol envelope, and a visit as requests on the wire.
//
// This package holds what is built from all of them: the wire-format fixtures in
// testdata and the images, both regenerated with go generate.
package mock
