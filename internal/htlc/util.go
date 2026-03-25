// Package htlc contains shared helpers for script formatting output.
package htlc

import "encoding/hex"

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

func shortID(txID string) string {
	if len(txID) == 0 {
		return "pending"
	}
	if len(txID) < 8 {
		return txID
	}
	return txID[:8]
}
