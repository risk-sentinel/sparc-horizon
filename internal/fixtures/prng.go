package fixtures

import "math"

// A deterministic pseudo-random source, in the repository rather than from
// math/rand.
//
// Two reasons, in order of weight. The generator's output has to be
// reproducible by the Ruby and Python ports of the key grammar if they ever
// generate fixtures of their own, and "math/rand with seed 11" does not mean
// the same sequence in three languages — splitmix64 does, in ten lines.
// Second, gosec flags math/rand (G404), and .golangci.yml keeps an empty
// exclusion set as its intended steady state; earning an exclusion for
// convenience is not the trade this repository makes.
//
// This is not a security primitive and must never be used as one.
type prng struct{ state uint64 }

// The conversion in intn is bounded by the caller's n; math.MaxInt is here to
// say so in the code rather than only in a comment.

// newPRNG seeds the sequence. The seed is a constant in gen.go: the fixtures
// are a fixture, and a fixture that changes between runs is not one.
func newPRNG(seed uint64) *prng { return &prng{state: seed} }

// next is splitmix64, as published: a 64-bit state, a fixed odd increment, and
// three xor-shift-multiply rounds.
func (p *prng) next() uint64 {
	p.state += 0x9E3779B97F4A7C15
	z := p.state
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// intn returns a value in [0, n). A non-positive n returns 0 rather than
// panicking: this is a fixture generator, and a panic in one is a worse
// failure than a degenerate draw.
func (p *prng) intn(n int) int {
	if n <= 0 {
		return 0
	}
	v := p.next() % uint64(n)
	// v is already below n, which is an int, so this cannot fire. It is here
	// because the conversion below is only safe for that reason, and a
	// reader — or a linter — should not have to reconstruct the argument.
	if v > math.MaxInt {
		return 0
	}
	return int(v)
}
