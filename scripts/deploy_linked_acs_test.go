package main

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func TestBurnSplitPresignedLinkedACSSpendValid(t *testing.T) {
	fundValueSat := int64(10000)
	feeSat := int64(1000)

	rjA := bytes.Repeat([]byte{0x11}, 32)
	preB := bytes.Repeat([]byte{0x22}, 32)
	hR := sha256.Sum256(rjA)
	hP := sha256.Sum256(preB)

	alicePriv, alicePub := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{0x33}, 32))
	bobPriv, bobPub := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{0x44}, 32))
	redeemScript, err := buildBurnSplitLinkedACSScript(hR[:], hP[:], alicePub.SerializeCompressed(), bobPub.SerializeCompressed())
	if err != nil {
		t.Fatalf("buildBurnSplitLinkedACSScript: %v", err)
	}

	prevHash := chainhash.DoubleHashH([]byte("funding-outpoint"))
	tx, burnValue, err := buildBurnSplitSpendTx(prevHash, 0, fundValueSat, feeSat, hR[:], hP[:])
	if err != nil {
		t.Fatalf("buildBurnSplitSpendTx: %v", err)
	}
	if burnValue != 9000 {
		t.Fatalf("burnValue=%d, want 9000", burnValue)
	}

	witness, err := signPresignedBurnSplitWitness(tx, 0, fundValueSat, redeemScript, preB, rjA, alicePriv, bobPriv)
	if err != nil {
		t.Fatalf("signPresignedBurnSplitWitness: %v", err)
	}
	tx.TxIn[0].Witness = witness

	prevPkScript, err := p2WSHScriptPubKey(redeemScript)
	if err != nil {
		t.Fatalf("p2WSHScriptPubKey: %v", err)
	}
	if err := verifyP2WSHSpend(tx, 0, prevPkScript, fundValueSat); err != nil {
		t.Fatalf("verifyP2WSHSpend(valid): %v", err)
	}

	if len(tx.TxOut) != 1 {
		t.Fatalf("tx outputs=%d, want 1 burn output", len(tx.TxOut))
	}
	if tx.TxOut[0].Value != burnValue {
		t.Fatalf("burn output value=%d, want %d", tx.TxOut[0].Value, burnValue)
	}
	if !txscript.IsUnspendable(tx.TxOut[0].PkScript) {
		t.Fatal("burn output must be provably unspendable (OP_RETURN)")
	}
}

func TestBobCannotTakeSurplusFromLinkedOutput(t *testing.T) {
	fundValueSat := int64(10000)
	feeSat := int64(5000)
	spendValueSat := fundValueSat - feeSat

	rjA := bytes.Repeat([]byte{0x61}, 32)
	preB := bytes.Repeat([]byte{0x62}, 32)
	hR := sha256.Sum256(rjA)
	hP := sha256.Sum256(preB)

	alicePriv, alicePub := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{0x63}, 32))
	bobPriv, bobPub := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{0x64}, 32))

	redeemScript, err := buildBurnSplitLinkedACSScript(hR[:], hP[:], alicePub.SerializeCompressed(), bobPub.SerializeCompressed())
	if err != nil {
		t.Fatalf("buildBurnSplitLinkedACSScript: %v", err)
	}

	prevHash := chainhash.DoubleHashH([]byte("funding-outpoint-surplus"))
	canonicalTx, _, err := buildBurnSplitSpendTx(prevHash, 1, fundValueSat, feeSat, hR[:], hP[:])
	if err != nil {
		t.Fatalf("build canonical tx: %v", err)
	}
	canonicalWitness, err := signPresignedBurnSplitWitness(canonicalTx, 0, fundValueSat, redeemScript, preB, rjA, alicePriv, bobPriv)
	if err != nil {
		t.Fatalf("sign canonical witness: %v", err)
	}
	canonicalTx.TxIn[0].Witness = canonicalWitness

	prevPkScript, err := p2WSHScriptPubKey(redeemScript)
	if err != nil {
		t.Fatalf("p2WSHScriptPubKey: %v", err)
	}
	if err := verifyP2WSHSpend(canonicalTx, 0, prevPkScript, fundValueSat); err != nil {
		t.Fatalf("canonical tx should verify: %v", err)
	}

	bobAddr, err := btcutil.NewAddressWitnessPubKeyHash(btcutil.Hash160(bobPub.SerializeCompressed()), &chaincfg.RegressionNetParams)
	if err != nil {
		t.Fatalf("bob addr: %v", err)
	}
	bobPkScript, err := txscript.PayToAddrScript(bobAddr)
	if err != nil {
		t.Fatalf("bob pkscript: %v", err)
	}

	mutatedTx := cloneTx(canonicalTx)
	mutatedTx.TxOut = []*wire.TxOut{{
		Value:    spendValueSat,
		PkScript: bobPkScript,
	}}

	prevFetcher := txscript.NewCannedPrevOutputFetcher(prevPkScript, fundValueSat)
	hashes := txscript.NewTxSigHashes(mutatedTx, prevFetcher)
	bobSig, err := txscript.RawTxInWitnessSignature(mutatedTx, hashes, 0, fundValueSat, redeemScript, txscript.SigHashAll, bobPriv)
	if err != nil {
		t.Fatalf("bob signature: %v", err)
	}

	candidates := []wire.TxWitness{
		{[]byte{}, bobSig, preB, rjA, redeemScript},
		{[]byte{}, bobSig, bobSig, preB, rjA, redeemScript},
		{[]byte{}, canonicalWitness[1], canonicalWitness[2], preB, rjA, redeemScript},
	}

	for i, w := range candidates {
		mutatedTx.TxIn[0].Witness = w
		if err := verifyP2WSHSpend(mutatedTx, 0, prevPkScript, fundValueSat); err == nil {
			t.Fatalf("candidate %d unexpectedly verifies; Bob should not obtain surplus path", i)
		}
	}
}

func TestLinkedACSRejectsWrongPreimages(t *testing.T) {
	fundValueSat := int64(10000)
	feeSat := int64(1000)

	rjA := bytes.Repeat([]byte{0x71}, 32)
	preB := bytes.Repeat([]byte{0x72}, 32)
	hR := sha256.Sum256(rjA)
	hP := sha256.Sum256(preB)

	alicePriv, alicePub := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{0x73}, 32))
	bobPriv, bobPub := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{0x74}, 32))
	redeemScript, err := buildBurnSplitLinkedACSScript(hR[:], hP[:], alicePub.SerializeCompressed(), bobPub.SerializeCompressed())
	if err != nil {
		t.Fatalf("buildBurnSplitLinkedACSScript: %v", err)
	}

	prevHash := chainhash.DoubleHashH([]byte("funding-outpoint-wrong-preimage"))
	tx, _, err := buildBurnSplitSpendTx(prevHash, 2, fundValueSat, feeSat, hR[:], hP[:])
	if err != nil {
		t.Fatalf("build tx: %v", err)
	}

	witness, err := signPresignedBurnSplitWitness(tx, 0, fundValueSat, redeemScript, preB, rjA, alicePriv, bobPriv)
	if err != nil {
		t.Fatalf("sign witness: %v", err)
	}
	prevPkScript, err := p2WSHScriptPubKey(redeemScript)
	if err != nil {
		t.Fatalf("p2WSHScriptPubKey: %v", err)
	}

	wrongPreB := bytes.Repeat([]byte{0x7A}, 32)
	tx.TxIn[0].Witness = wire.TxWitness{witness[0], witness[1], witness[2], wrongPreB, rjA, redeemScript}
	if err := verifyP2WSHSpend(tx, 0, prevPkScript, fundValueSat); err == nil {
		t.Fatal("verification unexpectedly succeeded with wrong pre_b")
	}
}

func cloneTx(src *wire.MsgTx) *wire.MsgTx {
	var buf bytes.Buffer
	_ = src.Serialize(&buf)
	clone := wire.NewMsgTx(src.Version)
	_ = clone.Deserialize(bytes.NewReader(buf.Bytes()))
	return clone
}
