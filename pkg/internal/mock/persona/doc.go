// Package persona is the four shoppers the traffic is generated for: Kate, Sofia, Lena
// and Mira, each a taste vector chosen so that each one's favourite product is a
// different one.
//
// A persona decides what a shopper picks. It scans the first few products of a listing
// and opens one it likes well enough, the earlier positions more likely, seeded so the
// traffic stays reproducible. The shop never sees a persona: how well its listings
// anticipate the picks is up to what it learns from them.
package persona
