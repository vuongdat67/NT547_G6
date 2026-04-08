// Package channel contains CRAB-He transaction generation.
package channel

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

type Output struct {
	ValueSat  int64
	Condition string
	Script    string
}

type Tx struct {
	Label    string
	SizeVB   int
	Inputs   []string
	Outputs  []Output
	TimeLock int64
	Note     string
}

func (t *Tx) String() string {
	s := fmt.Sprintf("[%s] ~%d vB", t.Label, t.SizeVB)
	if t.TimeLock > 0 {
		s += fmt.Sprintf(" CSV+%d", t.TimeLock)
	}
	s += "\n"
	for i, o := range t.Outputs {
		s += fmt.Sprintf("  out[%d] %d sat | %s\n", i, o.ValueSat, o.Condition)
	}
	if t.Note != "" {
		s += "  >> " + t.Note + "\n"
	}
	return s
}

type SerializedMeasurement struct {
	Name         string `json:"name"`
	BaseBytes    int    `json:"baseBytes"`
	WitnessBytes int    `json:"witnessBytes"`
	TotalBytes   int    `json:"totalBytes"`
	Weight       int    `json:"weight"`
	VBytes       int    `json:"vbytes"`
	InputCount   int    `json:"inputCount"`
	OutputCount  int    `json:"outputCount"`
	WitnessItems int    `json:"witnessItems"`
	TxHex        string `json:"txHex"`
}

func MakeFunding(p *Params, alicePK, bobPK string) *Tx {
	total := sat(p.V) + 2*sat(p.CStar)
	return &Tx{
		Label:  "tx_fund",
		SizeVB: 338,
		Inputs: []string{
			fmt.Sprintf("Alice UTXO %d sat (v+c*)", sat(p.V)+sat(p.CStar)),
			fmt.Sprintf("Bob   UTXO %d sat (c*)", sat(p.CStar)),
		},
		Outputs: []Output{{
			ValueSat:  total,
			Condition: fmt.Sprintf("2-of-2 multisig(%s|%s)", short(alicePK), short(bobPK)),
			Script:    fmt.Sprintf("OP_2 %s %s OP_2 OP_CHECKMULTISIG", short(alicePK), short(bobPK)),
		}},
		Note: fmt.Sprintf("total=%d sat  v=%d  c*=%d", total, sat(p.V), sat(p.CStar)),
	}
}

func MakeCommitA(p *Params, j int, vA, vB int64,
	rjA *RevocationSecret, htlc *HTLCSecrets,
	alicePK, bobPK string) *Tx {

	outputs := []Output{
		{
			ValueSat:  vB + sat(p.CStar),
			Condition: fmt.Sprintf("B immediately (pk=%s)", short(bobPK)),
			Script:    fmt.Sprintf("%s OP_CHECKSIG", short(bobPK)),
		},
		{
			ValueSat: vA + sat(p.CStar),
			Condition: fmt.Sprintf(
				"A after +%d CSV  |  B with r^%d_a (revoke)", p.T, j),
			Script: fmt.Sprintf(
				"OP_IF %s OP_CHECKSIG OP_ELSE %d OP_CSV OP_DROP %s OP_CHECKSIG OP_ENDIF",
				short(bobPK), p.T, short(alicePK)),
		},
		{
			ValueSat:  sat(p.CStar),
			Condition: fmt.Sprintf("[CRAB ACS] any miner + r^%d_a", j),
			Script: fmt.Sprintf(
				"OP_SHA256 %s OP_EQUALVERIFY OP_TRUE",
				hx(rjA.Hash)),
		},
	}

	sizeVB := estimateCommitVBytes(p, false, rjA.Hash, nil)
	note := fmt.Sprintf("j=%d vA=%d vB=%d c*=%d", j, vA, vB, sat(p.CStar))

	if htlc != nil {
		outputs = append(outputs, Output{
			ValueSat: sat(p.CStar),
			Condition: fmt.Sprintf(
				"[CRAB-He linked ACS] 2-of-2(A|B) + r^%d_a + pre_b; presigned split(miner=v_col, residual=burn)  H(pre_b)=%s",
				j, hx(htlc.HashPreB)),
			Script: fmt.Sprintf(
				"OP_SHA256 %s OP_EQUALVERIFY OP_SHA256 %s OP_EQUALVERIFY OP_2 %s %s OP_2 OP_CHECKMULTISIG",
				hx(rjA.Hash),
				hx(htlc.HashPreB),
				short(alicePK),
				short(bobPK)),
		})
		sizeVB = estimateCommitVBytes(p, true, rjA.Hash, htlc.HashPreB)
		note += " | HTLC linked ACS active"
	}

	return &Tx{
		Label:   fmt.Sprintf("tx_commit_A[%d]", j),
		SizeVB:  sizeVB,
		Inputs:  []string{"tx_fund out[0]"},
		Outputs: outputs,
		Note:    note,
	}
}

func estimateCommitVBytes(p *Params, withLinkedACS bool, hashRjA, hashPreB []byte) int {
	measure, err := MeasureCommitTemplateSerialized(p, withLinkedACS, hashRjA, hashPreB)
	if err != nil {
		panic(fmt.Sprintf("failed to build commit template tx for vsize measurement: %v", err))
	}

	return measure.VBytes
}

func MeasureCommitTemplateSerialized(p *Params, withLinkedACS bool, hashRjA, hashPreB []byte) (SerializedMeasurement, error) {
	tx, err := buildCommitTemplateTx(p, withLinkedACS, hashRjA, hashPreB)
	if err != nil {
		return SerializedMeasurement{}, err
	}

	base := tx.SerializeSizeStripped()
	total := tx.SerializeSize()
	weight := (base * 3) + total
	vsize := (weight + 3) / 4
	witnessBytes := total - base

	var witItems int
	if len(tx.TxIn) > 0 {
		witItems = len(tx.TxIn[0].Witness)
	}

	var buf bytes.Buffer
	if err := tx.Serialize(&buf); err != nil {
		return SerializedMeasurement{}, err
	}

	name := "tx_commit_A (no HTLC)"
	if withLinkedACS {
		name = "tx_commit_A (HTLC+linked ACS)"
	}

	return SerializedMeasurement{
		Name:         name,
		BaseBytes:    base,
		WitnessBytes: witnessBytes,
		TotalBytes:   total,
		Weight:       weight,
		VBytes:       vsize,
		InputCount:   len(tx.TxIn),
		OutputCount:  len(tx.TxOut),
		WitnessItems: witItems,
		TxHex:        hex.EncodeToString(buf.Bytes()),
	}, nil
}

func buildCommitTemplateTx(p *Params, withLinkedACS bool, hashRjA, hashPreB []byte) (*wire.MsgTx, error) {
	aliceKey := dummyCompressedKey(0x02, 0x11)
	bobKey := dummyCompressedKey(0x03, 0x22)
	revHash := normalizedHash32(hashRjA, 0x55)
	preBHash := normalizedHash32(hashPreB, 0x66)

	fundingRedeem, err := txscript.NewScriptBuilder().
		AddOp(txscript.OP_2).
		AddData(aliceKey).
		AddData(bobKey).
		AddOp(txscript.OP_2).
		AddOp(txscript.OP_CHECKMULTISIG).
		Script()
	if err != nil {
		return nil, err
	}

	out0, err := txscript.NewScriptBuilder().
		AddData(bobKey).
		AddOp(txscript.OP_CHECKSIG).
		Script()
	if err != nil {
		return nil, err
	}

	out1, err := txscript.NewScriptBuilder().
		AddOp(txscript.OP_IF).
		AddData(bobKey).
		AddOp(txscript.OP_CHECKSIG).
		AddOp(txscript.OP_ELSE).
		AddInt64(p.T).
		AddOp(txscript.OP_CHECKSEQUENCEVERIFY).
		AddOp(txscript.OP_DROP).
		AddData(aliceKey).
		AddOp(txscript.OP_CHECKSIG).
		AddOp(txscript.OP_ENDIF).
		Script()
	if err != nil {
		return nil, err
	}

	out2, err := txscript.NewScriptBuilder().
		AddOp(txscript.OP_SHA256).
		AddData(revHash).
		AddOp(txscript.OP_EQUALVERIFY).
		AddOp(txscript.OP_TRUE).
		Script()
	if err != nil {
		return nil, err
	}

	tx := wire.NewMsgTx(wire.TxVersion)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{},
		Sequence:         wire.MaxTxInSequenceNum,
	})

	cStar := sat(p.CStar)
	tx.AddTxOut(&wire.TxOut{Value: cStar, PkScript: out0})
	tx.AddTxOut(&wire.TxOut{Value: sat(p.V) + cStar, PkScript: out1})
	tx.AddTxOut(&wire.TxOut{Value: cStar, PkScript: out2})

	if withLinkedACS {
		out3, buildErr := txscript.NewScriptBuilder().
			AddOp(txscript.OP_SHA256).
			AddData(revHash).
			AddOp(txscript.OP_EQUALVERIFY).
			AddOp(txscript.OP_SHA256).
			AddData(preBHash).
			AddOp(txscript.OP_EQUALVERIFY).
			AddOp(txscript.OP_2).
			AddData(aliceKey).
			AddData(bobKey).
			AddOp(txscript.OP_2).
			AddOp(txscript.OP_CHECKMULTISIG).
			Script()
		if buildErr != nil {
			return nil, buildErr
		}
		tx.AddTxOut(&wire.TxOut{Value: cStar, PkScript: out3})
	}

	// Model the 2-of-2 funding-spend witness used by tx_commit_A.
	tx.TxIn[0].Witness = wire.TxWitness{
		[]byte{},
		bytes.Repeat([]byte{0x30}, 73),
		bytes.Repeat([]byte{0x31}, 73),
		fundingRedeem,
	}

	return tx, nil
}

func dummyCompressedKey(prefix, fill byte) []byte {
	key := make([]byte, 33)
	key[0] = prefix
	for i := 1; i < len(key); i++ {
		key[i] = fill
	}
	return key
}

func normalizedHash32(src []byte, fill byte) []byte {
	out := bytes.Repeat([]byte{fill}, 32)
	copy(out, src)
	return out
}

func MakeSpendA(p *Params, j int, vA int64, alicePK string) *Tx {
	return &Tx{
		Label:    fmt.Sprintf("tx_spend_A[%d]", j),
		SizeVB:   418,
		TimeLock: p.T,
		Inputs:   []string{fmt.Sprintf("tx_commit_A[%d] out[1]", j)},
		Outputs: []Output{{
			ValueSat:  vA + sat(p.CStar),
			Condition: fmt.Sprintf("A (pk=%s)", short(alicePK)),
			Script:    fmt.Sprintf("%s OP_CHECKSIG", short(alicePK)),
		}},
		Note: fmt.Sprintf("A honest close j=%d after +%d CSV", j, p.T),
	}
}

func MakeRevokeB(j int, rjA *RevocationSecret, vB, cStar int64, bobPK string) *Tx {
	return &Tx{
		Label:  fmt.Sprintf("tx_revoke_B[%d]", j),
		SizeVB: 192,
		Inputs: []string{fmt.Sprintf("tx_commit_A[%d] out[1]", j)},
		Outputs: []Output{{
			ValueSat:  vB + cStar,
			Condition: fmt.Sprintf("B (pk=%s)", short(bobPK)),
			Script:    fmt.Sprintf("%s OP_CHECKSIG", short(bobPK)),
		}},
		Note: fmt.Sprintf("B punishes state %d | witness: <sig_B> <r^%d_a=%s>",
			j, j, hx(rjA.Hash)),
	}
}

func MakeRevokeACSStd(j int, rjA *RevocationSecret, cStar int64) *Tx {
	return &Tx{
		Label:  fmt.Sprintf("tx_revoke_ACS_std[%d]", j),
		SizeVB: 192,
		Inputs: []string{fmt.Sprintf("tx_commit_A[%d] out[2]", j)},
		Outputs: []Output{{
			ValueSat:  cStar,
			Condition: "any miner (ACS) knowing r^j_a",
			Script: fmt.Sprintf(
				"OP_SHA256 %s OP_EQUALVERIFY OP_TRUE",
				hx(rjA.Hash)),
		}},
		Note: fmt.Sprintf("std CRAB ACS j=%d | witness: <r^%d_a>", j, j),
	}
}

func MakeRevokeACSLinked(j int, rjA *RevocationSecret, htlc *HTLCSecrets, cStar int64, vCol int64) *Tx {
	minerFee := vCol
	if minerFee > cStar {
		minerFee = cStar
	}
	burnValue := cStar - minerFee
	if burnValue < 0 {
		burnValue = 0
	}

	return &Tx{
		Label:  fmt.Sprintf("tx_revoke_ACS_linked[%d]", j),
		SizeVB: 246,
		Inputs: []string{fmt.Sprintf("tx_commit_A[%d] out[3]", j)},
		Outputs: []Output{{
			ValueSat:  burnValue,
			Condition: "provable burn output; miner reward is transaction fee",
			Script:    "OP_RETURN <burn-commitment>",
		}},
		Note: fmt.Sprintf(
			"CRAB-He linked ACS j=%d\n"+
				"  witness: <pre_b> <r^%d_a> + 2-of-2 presign\n"+
				"  trigger: Bob broadcasts dep-B on-chain\n"+
				"  r^%d_a from: PBB or Alice revoke tx\n"+
				"  payout split: miner fee=%d sat, burn output=%d sat\n"+
				"  Bob has no standalone spend path for out[3]",
			j, j, j, minerFee, burnValue),
	}
}

type Channel struct {
	P       *Params
	J       int
	BalA    int64
	BalB    int64
	RevA    *RevocationSecret
	RevB    *RevocationSecret
	HTLC    *HTLCSecrets
	AlicePK string
	BobPK   string
}

func NewChannel(p *Params, initBalA, initBalB int64,
	rev0A, rev0B *RevocationSecret, alicePK, bobPK string) *Channel {
	return &Channel{
		P: p, J: 0,
		BalA: initBalA, BalB: initBalB,
		RevA: rev0A, RevB: rev0B,
		AlicePK: alicePK, BobPK: bobPK,
	}
}

func (ch *Channel) AttachHTLC(htlc *HTLCSecrets) { ch.HTLC = htlc }
func (ch *Channel) DetachHTLC()                  { ch.HTLC = nil }

func (ch *Channel) GenerateCommitA() *Tx {
	return MakeCommitA(ch.P, ch.J, ch.BalA, ch.BalB, ch.RevA, ch.HTLC, ch.AlicePK, ch.BobPK)
}

func (ch *Channel) GeneratePunishmentBundle(fraudJ int, fraudRevA *RevocationSecret) []*Tx {
	bundle := []*Tx{
		MakeRevokeB(fraudJ, fraudRevA, ch.BalB, sat(ch.P.CStar), ch.BobPK),
		MakeRevokeACSStd(fraudJ, fraudRevA, sat(ch.P.CStar)),
	}
	if ch.HTLC != nil {
		bundle = append(bundle,
			MakeRevokeACSLinked(fraudJ, fraudRevA, ch.HTLC, sat(ch.P.CStar), sat(ch.P.VCol)))
	}
	return bundle
}

func (ch *Channel) Update(newBalA, newBalB int64, newRevA, newRevB *RevocationSecret) (old *RevocationSecret) {
	old = ch.RevA
	ch.RevA, ch.RevB = newRevA, newRevB
	ch.BalA, ch.BalB = newBalA, newBalB
	ch.J++
	return old
}

func (ch *Channel) PBBSecrets(upToJ int) []string {
	out := make([]string, 0, upToJ+1)
	for j := 0; j <= upToJ; j++ {
		out = append(out, fmt.Sprintf(
			"PBB[j=%d]: r^%d_a (OP_RETURN tx, ~600 bytes via RSA trapdoor)", j, j))
	}
	return out
}
