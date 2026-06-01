// Package channel contains shared helpers for formatting and satoshi conversion.
package channel

import (
	"encoding/hex"
	"math/big"
)

func sat(v *big.Int) int64 {
	return v.Int64()
}

func hx(b []byte) string {
	if len(b) == 0 {
		return "n/a"
	}
	h := hex.EncodeToString(b)
	if len(h) < 12 {
		return h
	}
	return h[:12] + "..."
}

func short(s string) string {
	if len(s) == 0 {
		return "n/a"
	}
	if len(s) < 8 {
		return s
	}
	return s[:8]
}
