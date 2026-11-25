package seed

import (
	"hash/fnv"
)

// seedHash hashes a seed to 64 well-spread bits: FNV-1a, then the 64-bit finalizer from
// MurmurHash3. FNV alone mixes the last bytes of its input poorly into the high bits, and
// seeds that differ only in a trailing counter — the same listing on successive visits —
// would all land in the same corner of the range.
func Hash(seed string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(seed))
	x := h.Sum64()

	x ^= x >> 33
	x *= 0xff51afd7ed558ccd
	x ^= x >> 33
	x *= 0xc4ceb9fe1a85ec53
	x ^= x >> 33

	return x
}
