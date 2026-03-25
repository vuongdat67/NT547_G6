// Package htlc implements He-HTLC[ell] contracts.
package htlc

import (
	"fmt"
)

type HeDep struct {
	VDep       int64
	VCol       int64
	HashPreA   []byte
	HashPreB   []byte
	AbsTimeout int64
	AlicePK    string
	BobPK      string
	TxID       string
}

type HeCol struct {
	VDep     int64
	VCol     int64
	HashPreA []byte
	HashPreB []byte
	Ell      int64
	AlicePK  string
	BobPK    string
	TxID     string
}

func HeDepScript(alicePK, bobPK string, hashPreA, hashPreB []byte, absT int64) string {
	return fmt.Sprintf(`
He-Dep Script (UTXOdep):
  2 %s %s 2 OP_CHECKMULTISIGVERIFY
  OP_HASH160 %s OP_EQUAL
  OP_IF
    OP_TRUE
  OP_ELSE
    %d OP_CHECKSEQUENCEVERIFY
    OP_DROP
    OP_HASH160 %s OP_EQUAL
  OP_ENDIF`,
		short(alicePK), short(bobPK),
		hx(hashPreA), absT, hx(hashPreB),
	)
}

func HeColScript(alicePK, bobPK string, hashPreA, hashPreB []byte, ell int64) string {
	return fmt.Sprintf(`
He-Col Script (UTXOcol):
  2 %s %s 2 OP_CHECKMULTISIGVERIFY
  OP_HASH160 %s OP_EQUAL
  OP_IF
    OP_HASH160 %s OP_EQUAL
  OP_ELSE
    %d OP_CHECKSEQUENCEVERIFY
    OP_DROP
    OP_TRUE
  OP_ENDIF`,
		short(alicePK), short(bobPK),
		hx(hashPreA), hx(hashPreB), ell,
	)
}

func DepA(dep *HeDep, preA []byte) *HeTx {
	return &HeTx{
		Label:  "tx_dep_A",
		SizeVB: 190,
		Path:   "dep-A",
		Inputs: []string{fmt.Sprintf("UTXOdep(%s)", shortID(dep.TxID))},
		Outputs: []HeTxOutput{
			{ValueSat: dep.VDep, Recipient: "Alice", Note: "v_dep payment"},
			{ValueSat: dep.VCol, Recipient: "Bob", Note: "v_col collateral return"},
		},
		Witness: fmt.Sprintf("<sig_A> <pre_a=%s>", hx(preA)),
		Note:    "Honest execution: Alice reveals pre_a before timeout T",
	}
}

func DepB(dep *HeDep, preB []byte) *HeTx {
	return &HeTx{
		Label:  "tx_dep_B",
		SizeVB: 172,
		Path:   "dep-B",
		Inputs: []string{fmt.Sprintf("UTXOdep(%s)", shortID(dep.TxID))},
		Outputs: []HeTxOutput{
			{
				ValueSat:  dep.VDep + dep.VCol,
				Recipient: "UTXOcol (He-Col)",
				Note:      "Creates He-Col with v_dep+v_col",
			},
		},
		Witness: fmt.Sprintf("<sig_B> <pre_b=%s>", hx(preB)),
		Note: "Bob refund path: pre_b revealed on-chain; this is the linked-ACS trigger in CRAB-He",
	}
}

func ColB(col *HeCol) *HeTx {
	return &HeTx{
		Label:  "tx_col_B",
		SizeVB: 152,
		Path:   "col-B",
		Inputs: []string{fmt.Sprintf("UTXOcol(%s)", shortID(col.TxID))},
		Outputs: []HeTxOutput{
			{ValueSat: col.VDep + col.VCol, Recipient: "Bob", Note: "v_dep+v_col refund"},
		},
		Witness:  "<sig_B>",
		TimeLock: col.Ell,
		Note:     fmt.Sprintf("Bob refund after ell=%d blocks CSV", col.Ell),
	}
}

func ColM(col *HeCol, preA, preB []byte, minerAddr string) *HeTx {
	return &HeTx{
		Label:  "tx_col_M",
		SizeVB: 168,
		Path:   "col-M",
		Inputs: []string{fmt.Sprintf("UTXOcol(%s)", shortID(col.TxID))},
		Outputs: []HeTxOutput{
			{ValueSat: col.VCol, Recipient: fmt.Sprintf("Miner (%s)", short(minerAddr)), Note: "v_col to miner"},
			{ValueSat: col.VDep, Recipient: "OP_RETURN (BURN)", Note: "v_dep permanently unspendable"},
		},
		Witness: fmt.Sprintf("<pre_a=%s> <pre_b=%s>", hx(preA), hx(preB)),
		Note:    "He-HTLC enforcement path",
	}
}

type HeTxOutput struct {
	ValueSat  int64
	Recipient string
	Note      string
}

type HeTx struct {
	Label    string
	SizeVB   int
	Path     string
	Inputs   []string
	Outputs  []HeTxOutput
	Witness  string
	TimeLock int64
	Note     string
}

func (t *HeTx) String() string {
	s := fmt.Sprintf("[%s] path=%s ~%d vB", t.Label, t.Path, t.SizeVB)
	if t.TimeLock > 0 {
		s += fmt.Sprintf(" CSV+%d", t.TimeLock)
	}
	s += "\n"
	s += fmt.Sprintf("  witness: %s\n", t.Witness)
	for i, o := range t.Outputs {
		s += fmt.Sprintf("  out[%d] %d sat -> %s (%s)\n", i, o.ValueSat, o.Recipient, o.Note)
	}
	if t.Note != "" {
		s += "  >> " + t.Note + "\n"
	}
	return s
}

type HTLC struct {
	Dep *HeDep
	Col *HeCol
}

func NewHTLC(vDep, vCol int64, hashPreA, hashPreB []byte,
	absT, ell int64, alicePK, bobPK string) *HTLC {
	return &HTLC{
		Dep: &HeDep{VDep: vDep, VCol: vCol, HashPreA: hashPreA, HashPreB: hashPreB, AbsTimeout: absT, AlicePK: alicePK, BobPK: bobPK},
		Col: &HeCol{VDep: vDep, VCol: vCol, HashPreA: hashPreA, HashPreB: hashPreB, Ell: ell, AlicePK: alicePK, BobPK: bobPK},
	}
}

func (h *HTLC) Scripts() string {
	return HeDepScript(h.Dep.AlicePK, h.Dep.BobPK, h.Dep.HashPreA, h.Dep.HashPreB, h.Dep.AbsTimeout) +
		"\n" +
		HeColScript(h.Col.AlicePK, h.Col.BobPK, h.Col.HashPreA, h.Col.HashPreB, h.Col.Ell)
}

func (h *HTLC) DepBTriggerNote() string {
	return fmt.Sprintf(`
CRAB-He Linked Revocation Trigger (dep-B broadcast):
  1. Bob broadcasts tx_dep_B with pre_b in witness
  2. pre_b = %s is now public on-chain
  3. Miners with r^j_a (from PBB or Alice revoke tx) can spend linked ACS
  4. Bob loses c* automatically, making CLBA self-defeating`,
		hx(h.Dep.HashPreB),
	)
}
