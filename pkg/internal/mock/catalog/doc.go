// Package catalog is what the mock shop sells: 54 products in a small category tree,
// dresses and trousers in women's fashion, cushions, lamps and vases in home decoration.
//
// Every product carries an attractor, a taste vector saying what it appeals to, and a
// description saying the same thing in prose. Score compares a taste to a product
// through the weights of the product's category: the one scorer both the shop ranking a
// listing and a persona choosing from it use. Each product also draws its own picture,
// from its facets, attractor and copy, never per id.
package catalog
