package mock

import (
	"sort"
	"strings"
)

// FixtureDir is the directory the generated wire-format fixtures live in, relative to
// this package.
const FixtureDir = "testdata"

// Fixtures returns every mock payload keyed by the file it is written to. It is the
// single source of truth shared by the generator and the golden test, so a new mock
// cannot be added without its fixture following along.
func Fixtures() map[string]any {
	out := map[string]any{
		// single event, fully typed envelopes
		"view_item_list.mpv2.json":              ViewItemListPayload(),
		"view_item_list_filtered.mpv2.json":     FilteredItemListPayload(),
		"view_item_list_personalized.mpv2.json": PersonalizedItemListPayload(PersonaBusinessKate),
		"select_item.mpv2.json":                 SelectItemPayload(),
		"view_item.mpv2.json":                   ViewItemPayload(),
		"add_to_cart.mpv2.json":                 AddToCartPayload(),

		// a shopper who browses and leaves
		"journey_browse_kate.mpv2.json": BrowseJourney(PersonaBusinessKate),
	}

	// one converting session per persona, each with its own client and session id so
	// they read as separate shoppers rather than one person changing their mind
	for _, p := range Personas() {
		name := strings.ToLower(p.Name)
		out["journey_"+name+".mpv2.json"] = Journey(p,
			WithClientID(DefaultClientID),
			WithUserID("U-"+strings.ToUpper(p.Name)),
			WithSessionID(DefaultSessionID),
		)

		// and the month that shopper came back over, one payload per visit
		out["history_"+name+".mpv2.json"] = History(Month(p),
			WithClientID(DefaultClientID),
			WithUserID("U-"+strings.ToUpper(p.Name)),
		)
	}

	return out
}

// FixtureNames returns the fixture file names in a stable order.
func FixtureNames() []string {
	fixtures := Fixtures()
	names := make([]string, 0, len(fixtures))

	for name := range fixtures {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}
