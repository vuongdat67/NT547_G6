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
	leafCRABScript, err := buildCRABTaprootLeafScript(hR[:])
	if err != nil {
		t.Fatalf("buildCRABTaprootLeafScript: %v", err)
	}
	redeemScript, err := buildBurnSplitLinkedACSScript(hR[:], hP[:], alicePub.SerializeCompressed(), bobPub.SerializeCompressed())
	if err != nil {
		t.Fatalf("buildBurnSplitLinkedACSScript: %v", err)
	}
	linkedLeaf := txscript.NewBaseTapLeaf(redeemScript)
	tapTree := txscript.AssembleTaprootScriptTree(txscript.NewBaseTapLeaf(leafCRABScript), linkedLeaf)
	internalKey := alicePub
	rootHash := tapTree.RootNode.TapHash()
	p2trScript, err := txscript.PayToTaprootScript(txscript.ComputeTaprootOutputKey(internalKey, rootHash[:]))
	if err != nil {
		t.Fatalf("PayToTaprootScript: %v", err)
	}

	prevHash := chainhash.DoubleHashH([]byte("funding-outpoint"))
	tx, burnValue, err := buildBurnSplitSpendTx(prevHash, 0, fundValueSat, feeSat, hR[:], hP[:])
	if err != nil {
		t.Fatalf("buildBurnSplitSpendTx: %v", err)
	}
	if burnValue != 9000 {
		t.Fatalf("burnValue=%d, want 9000", burnValue)
	}

	witness, err := signPresignedBurnSplitWitness(tx, 0, fundValueSat, p2trScript, linkedLeaf, tapTree, internalKey, preB, rjA, alicePriv, bobPriv)
	if err != nil {
		t.Fatalf("signPresignedBurnSplitWitness: %v", err)
	}
	tx.TxIn[0].Witness = witness

	if err := verifyTaprootScriptSpend(tx, 0, p2trScript, fundValueSat); err != nil {
		t.Fatalf("verifyTaprootScriptSpend(valid): %v", err)
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
	leafCRABScript, err := buildCRABTaprootLeafScript(hR[:])
	if err != nil {
		t.Fatalf("buildCRABTaprootLeafScript: %v", err)
	}

	redeemScript, err := buildBurnSplitLinkedACSScript(hR[:], hP[:], alicePub.SerializeCompressed(), bobPub.SerializeCompressed())
	if err != nil {
		t.Fatalf("buildBurnSplitLinkedACSScript: %v", err)
	}
	linkedLeaf := txscript.NewBaseTapLeaf(redeemScript)
	tapTree := txscript.AssembleTaprootScriptTree(txscript.NewBaseTapLeaf(leafCRABScript), linkedLeaf)
	internalKey := alicePub
	rootHash := tapTree.RootNode.TapHash()
	p2trScript, err := txscript.PayToTaprootScript(txscript.ComputeTaprootOutputKey(internalKey, rootHash[:]))
	if err != nil {
		t.Fatalf("PayToTaprootScript: %v", err)
	}

	prevHash := chainhash.DoubleHashH([]byte("funding-outpoint-surplus"))
	canonicalTx, _, err := buildBurnSplitSpendTx(prevHash, 1, fundValueSat, feeSat, hR[:], hP[:])
	if err != nil {
		t.Fatalf("build canonical tx: %v", err)
	}
	canonicalWitness, err := signPresignedBurnSplitWitness(canonicalTx, 0, fundValueSat, p2trScript, linkedLeaf, tapTree, internalKey, preB, rjA, alicePriv, bobPriv)
	if err != nil {
		t.Fatalf("sign canonical witness: %v", err)
	}
	canonicalTx.TxIn[0].Witness = canonicalWitness
	if err := verifyTaprootScriptSpend(canonicalTx, 0, p2trScript, fundValueSat); err != nil {
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
	proofIndex, ok := tapTree.LeafProofIndex[linkedLeaf.TapHash()]
	if !ok {
		t.Fatal("linked leaf proof not found")
	}
	ctrl := tapTree.LeafMerkleProofs[proofIndex].ToControlBlock(internalKey)
	ctrlBytes, err := ctrl.ToBytes()
	if err != nil {
		t.Fatalf("control block bytes: %v", err)
	}

	prevFetcher := txscript.NewCannedPrevOutputFetcher(p2trScript, fundValueSat)
	hashes := txscript.NewTxSigHashes(mutatedTx, prevFetcher)
	bobSig, err := txscript.RawTxInTapscriptSignature(mutatedTx, hashes, 0, fundValueSat, p2trScript, linkedLeaf, txscript.SigHashDefault, bobPriv)
	if err != nil {
		t.Fatalf("bob signature: %v", err)
	}

	candidates := []wire.TxWitness{
		{bobSig, preB, rjA, redeemScript, ctrlBytes},
		{bobSig, bobSig, preB, rjA, redeemScript, ctrlBytes},
		{canonicalWitness[0], canonicalWitness[1], preB, rjA, redeemScript, ctrlBytes},
	}

	for i, w := range candidates {
		mutatedTx.TxIn[0].Witness = w
		if err := verifyTaprootScriptSpend(mutatedTx, 0, p2trScript, fundValueSat); err == nil {
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
	leafCRABScript, err := buildCRABTaprootLeafScript(hR[:])
	if err != nil {
		t.Fatalf("buildCRABTaprootLeafScript: %v", err)
	}
	redeemScript, err := buildBurnSplitLinkedACSScript(hR[:], hP[:], alicePub.SerializeCompressed(), bobPub.SerializeCompressed())
	if err != nil {
		t.Fatalf("buildBurnSplitLinkedACSScript: %v", err)
	}
	linkedLeaf := txscript.NewBaseTapLeaf(redeemScript)
	tapTree := txscript.AssembleTaprootScriptTree(txscript.NewBaseTapLeaf(leafCRABScript), linkedLeaf)
	internalKey := alicePub
	rootHash := tapTree.RootNode.TapHash()
	p2trScript, err := txscript.PayToTaprootScript(txscript.ComputeTaprootOutputKey(internalKey, rootHash[:]))
	if err != nil {
		t.Fatalf("PayToTaprootScript: %v", err)
	}

	prevHash := chainhash.DoubleHashH([]byte("funding-outpoint-wrong-preimage"))
	tx, _, err := buildBurnSplitSpendTx(prevHash, 2, fundValueSat, feeSat, hR[:], hP[:])
	if err != nil {
		t.Fatalf("build tx: %v", err)
	}

	witness, err := signPresignedBurnSplitWitness(tx, 0, fundValueSat, p2trScript, linkedLeaf, tapTree, internalKey, preB, rjA, alicePriv, bobPriv)
	if err != nil {
		t.Fatalf("sign witness: %v", err)
	}

	wrongPreB := bytes.Repeat([]byte{0x7A}, 32)
	tx.TxIn[0].Witness = wire.TxWitness{witness[0], witness[1], wrongPreB, rjA, redeemScript, witness[5]}
	if err := verifyTaprootScriptSpend(tx, 0, p2trScript, fundValueSat); err == nil {
		t.Fatal("verification unexpectedly succeeded with wrong pre_b")
	}
}

func TestLinkedACSRejectsReplayAcrossDifferentOutpoint(t *testing.T) {
	fundValueSat := int64(10000)
	feeSat := int64(1000)

	rjA := bytes.Repeat([]byte{0x81}, 32)
	preB := bytes.Repeat([]byte{0x82}, 32)
	hR := sha256.Sum256(rjA)
	hP := sha256.Sum256(preB)

	alicePriv, alicePub := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{0x83}, 32))
	bobPriv, bobPub := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{0x84}, 32))
	leafCRABScript, err := buildCRABTaprootLeafScript(hR[:])
	if err != nil {
		t.Fatalf("buildCRABTaprootLeafScript: %v", err)
	}
	redeemScript, err := buildBurnSplitLinkedACSScript(hR[:], hP[:], alicePub.SerializeCompressed(), bobPub.SerializeCompressed())
	if err != nil {
		t.Fatalf("buildBurnSplitLinkedACSScript: %v", err)
	}
	linkedLeaf := txscript.NewBaseTapLeaf(redeemScript)
	tapTree := txscript.AssembleTaprootScriptTree(txscript.NewBaseTapLeaf(leafCRABScript), linkedLeaf)
	internalKey := alicePub
	rootHash := tapTree.RootNode.TapHash()
	p2trScript, err := txscript.PayToTaprootScript(txscript.ComputeTaprootOutputKey(internalKey, rootHash[:]))
	if err != nil {
		t.Fatalf("PayToTaprootScript: %v", err)
	}

	stateJPrevHash := chainhash.DoubleHashH([]byte("funding-state-j"))
	txStateJ, _, err := buildBurnSplitSpendTx(stateJPrevHash, 0, fundValueSat, feeSat, hR[:], hP[:])
	if err != nil {
		t.Fatalf("build state-j tx: %v", err)
	}
	stateJWitness, err := signPresignedBurnSplitWitness(txStateJ, 0, fundValueSat, p2trScript, linkedLeaf, tapTree, internalKey, preB, rjA, alicePriv, bobPriv)
	if err != nil {
		t.Fatalf("sign state-j witness: %v", err)
	}
	txStateJ.TxIn[0].Witness = stateJWitness
	if err := verifyTaprootScriptSpend(txStateJ, 0, p2trScript, fundValueSat); err != nil {
		t.Fatalf("state-j tx should verify: %v", err)
	}

	stateJ1PrevHash := chainhash.DoubleHashH([]byte("funding-state-j-plus-1"))
	txStateJ1, _, err := buildBurnSplitSpendTx(stateJ1PrevHash, 0, fundValueSat, feeSat, hR[:], hP[:])
	if err != nil {
		t.Fatalf("build state-j+1 tx: %v", err)
	}
	// Replaying a witness pre-signed for a different prevout must fail.
	txStateJ1.TxIn[0].Witness = stateJWitness
	if err := verifyTaprootScriptSpend(txStateJ1, 0, p2trScript, fundValueSat); err == nil {
		t.Fatal("replayed witness unexpectedly verified on a different outpoint/state")
	}
}

func TestLinkedACSRejectsReplayAcrossDifferentStateSameOutpoint(t *testing.T) {
	fundValueSat := int64(10000)
	feeSat := int64(1000)

	rjStateJ := bytes.Repeat([]byte{0x91}, 32)
	preStateJ := bytes.Repeat([]byte{0x92}, 32)
	hRj := sha256.Sum256(rjStateJ)
	hPj := sha256.Sum256(preStateJ)

	rjStateJ1 := bytes.Repeat([]byte{0x93}, 32)
	preStateJ1 := bytes.Repeat([]byte{0x94}, 32)
	hRj1 := sha256.Sum256(rjStateJ1)
	hPj1 := sha256.Sum256(preStateJ1)

	alicePriv, alicePub := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{0x95}, 32))
	bobPriv, bobPub := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{0x96}, 32))

	// State j linked leaf / taproot commitment.
	leafCRABJ, err := buildCRABTaprootLeafScript(hRj[:])
	if err != nil {
		t.Fatalf("buildCRABTaprootLeafScript(state j): %v", err)
	}
	redeemJ, err := buildBurnSplitLinkedACSScript(hRj[:], hPj[:], alicePub.SerializeCompressed(), bobPub.SerializeCompressed())
	if err != nil {
		t.Fatalf("buildBurnSplitLinkedACSScript(state j): %v", err)
	}
	leafJ := txscript.NewBaseTapLeaf(redeemJ)
	treeJ := txscript.AssembleTaprootScriptTree(txscript.NewBaseTapLeaf(leafCRABJ), leafJ)
	rootJ := treeJ.RootNode.TapHash()
	p2trJ, err := txscript.PayToTaprootScript(txscript.ComputeTaprootOutputKey(alicePub, rootJ[:]))
	if err != nil {
		t.Fatalf("PayToTaprootScript(state j): %v", err)
	}

	// State j+1 linked leaf / taproot commitment.
	leafCRABJ1, err := buildCRABTaprootLeafScript(hRj1[:])
	if err != nil {
		t.Fatalf("buildCRABTaprootLeafScript(state j+1): %v", err)
	}
	redeemJ1, err := buildBurnSplitLinkedACSScript(hRj1[:], hPj1[:], alicePub.SerializeCompressed(), bobPub.SerializeCompressed())
	if err != nil {
		t.Fatalf("buildBurnSplitLinkedACSScript(state j+1): %v", err)
	}
	leafJ1 := txscript.NewBaseTapLeaf(redeemJ1)
	treeJ1 := txscript.AssembleTaprootScriptTree(txscript.NewBaseTapLeaf(leafCRABJ1), leafJ1)
	rootJ1 := treeJ1.RootNode.TapHash()
	p2trJ1, err := txscript.PayToTaprootScript(txscript.ComputeTaprootOutputKey(alicePub, rootJ1[:]))
	if err != nil {
		t.Fatalf("PayToTaprootScript(state j+1): %v", err)
	}

	// Same prevout, but witness/signature bound to state-j scripts must fail on state-j+1 script commitment.
	sharedPrevHash := chainhash.DoubleHashH([]byte("funding-shared-outpoint-state-rotation"))
	txStateJ, _, err := buildBurnSplitSpendTx(sharedPrevHash, 0, fundValueSat, feeSat, hRj[:], hPj[:])
	if err != nil {
		t.Fatalf("build state-j tx: %v", err)
	}
	wJ, err := signPresignedBurnSplitWitness(txStateJ, 0, fundValueSat, p2trJ, leafJ, treeJ, alicePub, preStateJ, rjStateJ, alicePriv, bobPriv)
	if err != nil {
		t.Fatalf("sign state-j witness: %v", err)
	}
	txStateJ.TxIn[0].Witness = wJ
	if err := verifyTaprootScriptSpend(txStateJ, 0, p2trJ, fundValueSat); err != nil {
		t.Fatalf("state-j tx should verify: %v", err)
	}

	txStateJ1, _, err := buildBurnSplitSpendTx(sharedPrevHash, 0, fundValueSat, feeSat, hRj1[:], hPj1[:])
	if err != nil {
		t.Fatalf("build state-j+1 tx: %v", err)
	}
	txStateJ1.TxIn[0].Witness = wJ
	if err := verifyTaprootScriptSpend(txStateJ1, 0, p2trJ1, fundValueSat); err == nil {
		t.Fatal("state-j witness unexpectedly verified on state-j+1 commitment for same outpoint")
	}
}

func cloneTx(src *wire.MsgTx) *wire.MsgTx {
	var buf bytes.Buffer
	_ = src.Serialize(&buf)
	clone := wire.NewMsgTx(src.Version)
	_ = clone.Deserialize(bytes.NewReader(buf.Bytes()))
	return clone
}
