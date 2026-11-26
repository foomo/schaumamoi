// Package shop is the shop's side of the traffic: the listings, as filters over the
// catalog ranked by a profile, and the GA4 events reporting them (view_item_list,
// select_item, view_item, add_to_cart).
//
// The shop ranks by a Profile, the personalization vector the affinity still distills from
// a shopper's events so far. It never sees a persona, and this package does not import
// one: what it knows about a shopper is what the shopper has done.
package shop
