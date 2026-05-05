package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

type rpcCLI struct {
	binPath string
	network string
	wallet  string
}

func (c *rpcCLI) run(walletScoped bool, args ...string) (string, error) {
	full := make([]string, 0, len(args)+2)
	switch c.network {
	case "regtest":
		full = append(full, "-regtest")
	case "signet":
		full = append(full, "-signet")
	default:
		return "", fmt.Errorf("unsupported network %q", c.network)
	}
	if walletScoped {
		if strings.TrimSpace(c.wallet) == "" {
			return "", errors.New("wallet name is required for wallet-scoped calls")
		}
		full = append(full, "-rpcwallet="+c.wallet)
	}
	full = append(full, args...)

	cmd := exec.Command(c.binPath, full...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("bitcoin-cli %s failed: %w\n%s", strings.Join(full, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

type chainInfo struct {
	Chain  string `json:"chain"`
	Blocks int64  `json:"blocks"`
}

type walletDirEntry struct {
	Name string `json:"name"`
}

type walletDirResult struct {
	Wallets []walletDirEntry `json:"wallets"`
}

type outputInfo struct {
	Value        float64 `json:"value"`
	ScriptPubKey struct {
		Address string `json:"address"`
	} `json:"scriptPubKey"`
}

type rawTxVerbose struct {
	Vout []outputInfo `json:"vout"`
}

type walletTxDetail struct {
	Address  string  `json:"address"`
	Category string  `json:"category"`
	Amount   float64 `json:"amount"`
	Vout     uint32  `json:"vout"`
}

type walletTxResult struct {
	Hex     string           `json:"hex"`
	Details []walletTxDetail `json:"details"`
}

type addressInfo struct {
	ScriptPubKey string `json:"scriptPubKey"`
}

type mempoolResult struct {
	Allowed bool   `json:"allowed"`
	Reject  string `json:"reject-reason"`
}

type deployArtifact struct {
	Network            string `json:"network"`
	Wallet             string `json:"wallet"`
	LinkedACSAddress   string `json:"linkedACSAddress"`
	LinkedACSScriptHex string `json:"linkedACSScriptHex"`
	LeafCRABScriptHex  string `json:"leafCrabScriptHex"`
	TaprootOutputKey   string `json:"taprootOutputKeyXOnly"`
	AlicePubKeyHex     string `json:"alicePubKeyHex"`
	BobPubKeyHex       string `json:"bobPubKeyHex"`
	FundTxID           string `json:"fundTxid"`
	FundVout           uint32 `json:"fundVout"`
	FundValueSat       int64  `json:"fundValueSat"`
	SpendTxID          string `json:"spendTxid"`
	SpendValueSat      int64  `json:"spendValueSat"`
	MinerPayoutSat     int64  `json:"minerPayoutSat"`
	BurnValueSat       int64  `json:"burnValueSat"`
	FeeSat             int64  `json:"feeSat"`
	WitnessOrder       string `json:"witnessOrder"`
	HashRjA            string `json:"hashRjA"`
	HashPreB           string `json:"hashPreB"`
	CreatedAtUTC       string `json:"createdAtUtc"`
}

func main() {
	var (
		bitcoinCLI   = flag.String("bitcoin-cli", "bitcoin-cli", "Path to bitcoin-cli executable")
		network      = flag.String("network", "regtest", "Network to use: regtest or signet")
		wallet       = flag.String("wallet", "hehtlc_research", "Wallet name")
		tryLoad      = flag.Bool("try-load-wallet", true, "Try loading wallet before wallet-scoped RPC calls")
		createWallet = flag.Bool("create-wallet-if-missing", false, "Create wallet if missing when try-load-wallet is enabled")
		fundSat      = flag.Int64("fund-sat", 3000000, "Funding amount in satoshis sent to linked ACS output (default c*)")
		feeSat       = flag.Int64("fee-sat", 500000, "Miner reward carried as spend transaction fee in satoshis (default v_col)")
		maxBurnBTC   = flag.String("max-burn-btc", "", "Maximum BTC allowed as provably-unspendable burn output in sendrawtransaction; if empty, computed dynamically as (fund-sat - fee-sat) * 1.1 / 1e8")
		autoMine     = flag.Bool("auto-mine-regtest", true, "Mine 1 block automatically on regtest after fund and spend")
		pollSeconds  = flag.Int("poll-seconds", 5, "Polling interval while waiting for fund tx visibility")
		maxWaitSec   = flag.Int("max-wait-seconds", 120, "Max wait for tx visibility")
		artifactPath = flag.String("artifact", "", "Optional artifact path (default: artifacts/linked_acs_<network>.json)")
	)
	flag.Parse()

	if *network != "regtest" && *network != "signet" {
		die("invalid -network %q (expected regtest or signet)", *network)
	}
	if *fundSat <= 0 {
		die("-fund-sat must be > 0")
	}
	if *feeSat <= 0 {
		die("-fee-sat must be > 0")
	}
	if *fundSat <= *feeSat {
		die("-fund-sat (%d) must be greater than -fee-sat (%d)", *fundSat, *feeSat)
	}
	if strings.TrimSpace(*maxBurnBTC) == "" {
		// Compute dynamically with 10% safety margin so the flag works for any c* size.
		maxBurnFloat := float64(*fundSat-*feeSat) / 1e8 * 1.1
		s := strconv.FormatFloat(maxBurnFloat, 'f', 8, 64)
		maxBurnBTC = &s
	}

	cli := &rpcCLI{binPath: *bitcoinCLI, network: *network, wallet: *wallet}

	fmt.Println("[1/9] Checking node connectivity...")
	ciRaw, err := cli.run(false, "getblockchaininfo")
	must(err)
	var ci chainInfo
	must(json.Unmarshal([]byte(ciRaw), &ci))
	if ci.Chain != *network {
		die("connected chain is %q, expected %q", ci.Chain, *network)
	}

	if *tryLoad {
		fmt.Println("[1.5/9] Ensuring wallet is loaded...")
		must(ensureWalletReady(cli, *wallet, *createWallet))
	}

	fmt.Println("[1.6/9] Checking wallet balance...")
	balanceSat, err := walletBalanceSat(cli)
	must(err)
	requiredSat := *fundSat
	if balanceSat < requiredSat {
		die("insufficient funds in wallet %q on %s: balance=%d sat, required>=%d sat (fund-sat)", *wallet, *network, balanceSat, requiredSat)
	}

	fmt.Println("[2/9] Generating secrets, signer keys, and burn-split linked ACS script...")
	rjA := random32()
	preB := random32()
	hashRjA := sha256.Sum256(rjA)
	hashPreB := sha256.Sum256(preB)
	alicePriv, alicePub := btcec.PrivKeyFromBytes(random32())
	bobPriv, bobPub := btcec.PrivKeyFromBytes(random32())

	leafCRABScript, err := buildCRABTaprootLeafScript(hashRjA[:])
	must(err)
	linkedScript, err := buildBurnSplitLinkedACSScript(hashRjA[:], hashPreB[:], alicePub.SerializeCompressed(), bobPub.SerializeCompressed())
	must(err)

	leafCRAB := txscript.NewBaseTapLeaf(leafCRABScript)
	linkedLeaf := txscript.NewBaseTapLeaf(linkedScript)
	tapTree := txscript.AssembleTaprootScriptTree(leafCRAB, linkedLeaf)
	rootHash := tapTree.RootNode.TapHash()
	internalKey := alicePub
	outKey := txscript.ComputeTaprootOutputKey(internalKey, rootHash[:])
	p2trScript, err := txscript.PayToTaprootScript(outKey)
	must(err)

	params := &chaincfg.RegressionNetParams
	if *network == "signet" {
		params = &chaincfg.SigNetParams
	}
	addr, err := btcutil.NewAddressTaproot(schnorr.SerializePubKey(outKey), params)
	must(err)

	fmt.Printf("  linked ACS address: %s\n", addr.EncodeAddress())
	fmt.Printf("  alice signer pubkey: %s\n", hex.EncodeToString(alicePub.SerializeCompressed()))
	fmt.Printf("  bob signer pubkey:   %s\n", hex.EncodeToString(bobPub.SerializeCompressed()))
	fmt.Printf("  H(r^j_a): %s\n", hex.EncodeToString(hashRjA[:]))
	fmt.Printf("  H(pre_b): %s\n", hex.EncodeToString(hashPreB[:]))

	fmt.Println("[3/9] Funding linked ACS output...")
	fundBtc := satsToBTC(*fundSat)
	fundTxID, err := cli.run(true, "-named", "sendtoaddress", "address="+addr.EncodeAddress(), "amount="+fundBtc)
	must(err)
	fundTxID = strings.TrimSpace(fundTxID)
	fmt.Printf("  fund txid: %s\n", fundTxID)

	if *network == "regtest" && *autoMine {
		fmt.Println("[4/9] Mining 1 regtest block to confirm funding tx...")
		mineAddr, err := cli.run(true, "getnewaddress")
		must(err)
		_, err = cli.run(false, "generatetoaddress", "1", strings.TrimSpace(mineAddr))
		must(err)
	} else {
		fmt.Println("[4/9] Skipping mining step (signet/manual mode)...")
	}

	fundVout, fundValueSat := waitForFundOutput(cli, fundTxID, addr.EncodeAddress(), time.Duration(*pollSeconds)*time.Second, time.Duration(*maxWaitSec)*time.Second)
	fmt.Printf("  located funding outpoint: %s:%d (%d sat)\n", fundTxID, fundVout, fundValueSat)

	fmt.Println("[5/9] Building burn-split pre-signed spend transaction...")
	if fundValueSat <= *feeSat {
		die("funding value %d is not enough after fee %d", fundValueSat, *feeSat)
	}

	hash, err := chainhash.NewHashFromStr(fundTxID)
	must(err)
	spendTx, burnValue, err := buildBurnSplitSpendTx(*hash, fundVout, fundValueSat, *feeSat, hashRjA[:], hashPreB[:])
	must(err)

	witness, err := signPresignedBurnSplitWitness(
		spendTx,
		0,
		fundValueSat,
		p2trScript,
		linkedLeaf,
		tapTree,
		internalKey,
		preB,
		rjA,
		alicePriv,
		bobPriv,
	)
	must(err)
	spendTx.TxIn[0].Witness = witness

	must(verifyTaprootScriptSpend(spendTx, 0, p2trScript, fundValueSat))
	fmt.Printf("  burn-split outputs: burn=%d sat; miner reward via fee=%d sat\n", burnValue, *feeSat)

	var buf bytes.Buffer
	must(spendTx.Serialize(&buf))
	spendHex := hex.EncodeToString(buf.Bytes())

	spendValue := int64(0)
	for _, out := range spendTx.TxOut {
		spendValue += out.Value
	}

	fmt.Println("[6/9] Verifying mempool acceptance...")
	acceptRaw, err := cli.run(false, "testmempoolaccept", fmt.Sprintf("[\"%s\"]", spendHex))
	must(err)
	var accepts []mempoolResult
	must(json.Unmarshal([]byte(acceptRaw), &accepts))
	if len(accepts) == 0 || !accepts[0].Allowed {
		reason := "unknown"
		if len(accepts) > 0 && accepts[0].Reject != "" {
			reason = accepts[0].Reject
		}
		die("spend tx rejected by mempool: %s", reason)
	}

	fmt.Println("[7/9] Broadcasting spend transaction...")
	spendTxID, err := cli.run(false, "sendrawtransaction", spendHex, "0", *maxBurnBTC)
	must(err)
	spendTxID = strings.TrimSpace(spendTxID)
	fmt.Printf("  spend txid: %s\n", spendTxID)

	if *network == "regtest" && *autoMine {
		fmt.Println("[8/9] Mining 1 regtest block to confirm spend tx...")
		mineAddr, err := cli.run(true, "getnewaddress")
		must(err)
		_, err = cli.run(false, "generatetoaddress", "1", strings.TrimSpace(mineAddr))
		must(err)
	} else {
		fmt.Println("[8/9] No auto-mining for this network/mode.")
	}

	fmt.Println("[9/9] Writing artifact...")
	artifact := deployArtifact{
		Network:            *network,
		Wallet:             *wallet,
		LinkedACSAddress:   addr.EncodeAddress(),
		LinkedACSScriptHex: hex.EncodeToString(linkedScript),
		LeafCRABScriptHex:  hex.EncodeToString(leafCRABScript),
		TaprootOutputKey:   hex.EncodeToString(schnorr.SerializePubKey(outKey)),
		AlicePubKeyHex:     hex.EncodeToString(alicePub.SerializeCompressed()),
		BobPubKeyHex:       hex.EncodeToString(bobPub.SerializeCompressed()),
		FundTxID:           fundTxID,
		FundVout:           fundVout,
		FundValueSat:       fundValueSat,
		SpendTxID:          spendTxID,
		SpendValueSat:      spendValue,
		MinerPayoutSat:     *feeSat,
		BurnValueSat:       burnValue,
		FeeSat:             *feeSat,
		WitnessOrder:       "<sig_B> <sig_A> <pre_b> <r^j_a> <linkedLeafScript> <controlBlock>",
		HashRjA:            hex.EncodeToString(hashRjA[:]),
		HashPreB:           hex.EncodeToString(hashPreB[:]),
		CreatedAtUTC:       time.Now().UTC().Format(time.RFC3339),
	}

	path := *artifactPath
	if strings.TrimSpace(path) == "" {
		path = filepath.Join("artifacts", fmt.Sprintf("linked_acs_%s.json", *network))
	}
	must(os.MkdirAll(filepath.Dir(path), 0o755))
	jsonBytes, err := json.MarshalIndent(artifact, "", "  ")
	must(err)
	must(os.WriteFile(path, jsonBytes, 0o644))

	fmt.Println("\n=== Linked ACS deployment completed ===")
	fmt.Printf("Fund TxID : %s\n", fundTxID)
	fmt.Printf("Spend TxID: %s\n", spendTxID)
	fmt.Printf("Artifact  : %s\n", path)
}

func buildCRABTaprootLeafScript(hashRjA []byte) ([]byte, error) {
	b := txscript.NewScriptBuilder()
	b.AddOp(txscript.OP_SHA256)
	b.AddData(hashRjA)
	b.AddOp(txscript.OP_EQUAL)
	return b.Script()
}

func buildBurnSplitLinkedACSScript(hashRjA, hashPreB, alicePub, bobPub []byte) ([]byte, error) {
	aliceKey, err := btcec.ParsePubKey(alicePub)
	if err != nil {
		return nil, fmt.Errorf("parse alice pubkey: %w", err)
	}
	bobKey, err := btcec.ParsePubKey(bobPub)
	if err != nil {
		return nil, fmt.Errorf("parse bob pubkey: %w", err)
	}
	aliceXOnly := schnorr.SerializePubKey(aliceKey)
	bobXOnly := schnorr.SerializePubKey(bobKey)

	b := txscript.NewScriptBuilder()
	b.AddOp(txscript.OP_SHA256)
	b.AddData(hashRjA)
	b.AddOp(txscript.OP_EQUALVERIFY)
	b.AddOp(txscript.OP_SHA256)
	b.AddData(hashPreB)
	b.AddOp(txscript.OP_EQUALVERIFY)
	b.AddData(aliceXOnly)
	b.AddOp(txscript.OP_CHECKSIG)
	b.AddData(bobXOnly)
	b.AddOp(txscript.OP_CHECKSIGADD)
	b.AddOp(txscript.OP_2)
	b.AddOp(txscript.OP_EQUAL)
	return b.Script()
}

func buildBurnSplitSpendTx(prevHash chainhash.Hash, prevVout uint32, fundValueSat, feeSat int64, hashRjA, hashPreB []byte) (*wire.MsgTx, int64, error) {
	if feeSat <= 0 {
		return nil, 0, fmt.Errorf("feeSat must be > 0")
	}
	if fundValueSat <= feeSat {
		return nil, 0, fmt.Errorf("fundValueSat (%d) must be > feeSat (%d)", fundValueSat, feeSat)
	}
	// In CRAB-He notation, fundValueSat models c* on out[2] and feeSat models
	// v_col paid to the including miner, so the burned residual is c* - v_col.
	burnValue := fundValueSat - feeSat

	burnPkScript, err := buildBurnOutputScript(hashRjA, hashPreB)
	if err != nil {
		return nil, 0, err
	}

	tx := wire.NewMsgTx(wire.TxVersion)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: prevHash, Index: prevVout},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	tx.AddTxOut(&wire.TxOut{Value: burnValue, PkScript: burnPkScript})

	return tx, burnValue, nil
}

func buildBurnOutputScript(hashRjA, hashPreB []byte) ([]byte, error) {
	// Marker bytes are for auditability only. Replay protection is provided by
	// Taproot signatures committing to the concrete prevout (txid:vout) of the
	// funded out[2] UTXO, so a witness pre-signed for state j cannot authorize
	// spending a different state's outpoint.
	marker := append([]byte("crab-he-burn:"), hashRjA[:4]...)
	marker = append(marker, hashPreB[:4]...)
	b := txscript.NewScriptBuilder()
	b.AddOp(txscript.OP_RETURN)
	b.AddData(marker)
	return b.Script()
}

func signPresignedBurnSplitWitness(tx *wire.MsgTx, inputIndex int, prevValueSat int64, prevPkScript []byte, linkedLeaf txscript.TapLeaf, tapTree *txscript.IndexedTapScriptTree, internalKey *btcec.PublicKey, preB, rjA []byte, alicePriv, bobPriv *btcec.PrivateKey) (wire.TxWitness, error) {
	prevFetcher := txscript.NewCannedPrevOutputFetcher(prevPkScript, prevValueSat)
	hashes := txscript.NewTxSigHashes(tx, prevFetcher)

	sigA, err := txscript.RawTxInTapscriptSignature(tx, hashes, inputIndex, prevValueSat, prevPkScript, linkedLeaf, txscript.SigHashDefault, alicePriv)
	if err != nil {
		return nil, fmt.Errorf("alice signature: %w", err)
	}
	sigB, err := txscript.RawTxInTapscriptSignature(tx, hashes, inputIndex, prevValueSat, prevPkScript, linkedLeaf, txscript.SigHashDefault, bobPriv)
	if err != nil {
		return nil, fmt.Errorf("bob signature: %w", err)
	}
	proofIndex, ok := tapTree.LeafProofIndex[linkedLeaf.TapHash()]
	if !ok {
		return nil, errors.New("linked leaf proof not found in taproot tree")
	}
	ctrl := tapTree.LeafMerkleProofs[proofIndex].ToControlBlock(internalKey)
	ctrlBytes, err := ctrl.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("serialize control block: %w", err)
	}

	// Witness order is chosen so preimage checks consume rjA then preB before signature checks.
	w := wire.TxWitness{sigB, sigA, preB, rjA, linkedLeaf.Script, ctrlBytes}
	tx.TxIn[inputIndex].Witness = w
	if err := verifyTaprootScriptSpend(tx, inputIndex, prevPkScript, prevValueSat); err != nil {
		return nil, fmt.Errorf("deterministic witness ordering failed verification: %w", err)
	}

	return w, nil
}

func verifyTaprootScriptSpend(tx *wire.MsgTx, inputIndex int, prevPkScript []byte, prevValueSat int64) error {
	prevFetcher := txscript.NewCannedPrevOutputFetcher(prevPkScript, prevValueSat)
	hashes := txscript.NewTxSigHashes(tx, prevFetcher)
	vm, err := txscript.NewEngine(prevPkScript, tx, inputIndex, txscript.StandardVerifyFlags, nil, hashes, prevValueSat, prevFetcher)
	if err != nil {
		return err
	}
	return vm.Execute()
}

func waitForFundOutput(cli *rpcCLI, txID, expectedAddr string, poll, timeout time.Duration) (uint32, int64) {
	deadline := time.Now().Add(timeout)
	for {
		vout, sat, found, err := findOutput(cli, txID, expectedAddr)
		if err == nil && found {
			return vout, sat
		}
		if time.Now().After(deadline) {
			if err != nil {
				die("timed out waiting funding output: %v", err)
			}
			die("timed out waiting funding output for address %s in tx %s", expectedAddr, txID)
		}
		time.Sleep(poll)
	}
}

func findOutput(cli *rpcCLI, txID, expectedAddr string) (uint32, int64, bool, error) {
	// Prefer wallet-aware lookup so txindex is not required.
	txRaw, err := cli.run(true, "gettransaction", txID, "true")
	if err != nil {
		return 0, 0, false, err
	}
	var wtx walletTxResult
	if err := json.Unmarshal([]byte(txRaw), &wtx); err != nil {
		return 0, 0, false, err
	}
	for _, d := range wtx.Details {
		if d.Address == expectedAddr && d.Category == "send" {
			// Wallet "send" amount is negative; convert to positive sat value.
			sat := int64(math.Round(math.Abs(d.Amount) * 1e8))
			return d.Vout, sat, true, nil
		}
	}

	// Fallback: decode tx hex and match address in outputs.
	if strings.TrimSpace(wtx.Hex) == "" {
		return 0, 0, false, nil
	}
	raw, err := cli.run(false, "decoderawtransaction", wtx.Hex)
	if err != nil {
		return 0, 0, false, err
	}
	var tx rawTxVerbose
	if err := json.Unmarshal([]byte(raw), &tx); err != nil {
		return 0, 0, false, err
	}
	for i, out := range tx.Vout {
		if out.ScriptPubKey.Address == expectedAddr {
			sat := int64(math.Round(out.Value * 1e8))
			return uint32(i), sat, true, nil
		}
	}
	return 0, 0, false, nil
}

func satsToBTC(sat int64) string {
	r := new(big.Rat).SetFrac(big.NewInt(sat), big.NewInt(100_000_000))
	f, _ := r.Float64()
	return strconv.FormatFloat(f, 'f', 8, 64)
}

func walletBalanceSat(cli *rpcCLI) (int64, error) {
	raw, err := cli.run(true, "getbalance")
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, fmt.Errorf("parse getbalance output %q: %w", raw, err)
	}
	return int64(math.Round(v * 1e8)), nil
}

func ensureWalletReady(cli *rpcCLI, wallet string, createIfMissing bool) error {
	wallet = strings.TrimSpace(wallet)
	if wallet == "" {
		return errors.New("wallet name is empty")
	}

	loadedRaw, err := cli.run(false, "listwallets")
	if err != nil {
		return err
	}
	var loaded []string
	if err := json.Unmarshal([]byte(loadedRaw), &loaded); err != nil {
		return fmt.Errorf("parse listwallets: %w", err)
	}
	for _, w := range loaded {
		if w == wallet {
			return nil
		}
	}

	dirRaw, err := cli.run(false, "listwalletdir")
	if err != nil {
		return err
	}
	var dir walletDirResult
	if err := json.Unmarshal([]byte(dirRaw), &dir); err != nil {
		return fmt.Errorf("parse listwalletdir: %w", err)
	}

	exists := false
	for _, w := range dir.Wallets {
		if w.Name == wallet {
			exists = true
			break
		}
	}

	if !exists {
		if !createIfMissing {
			return fmt.Errorf("wallet %q not found. Use -create-wallet-if-missing or create it manually", wallet)
		}
		if _, err := cli.run(false, "createwallet", wallet); err != nil {
			return fmt.Errorf("createwallet %q failed: %w", wallet, err)
		}
		return nil
	}

	if _, err := cli.run(false, "loadwallet", wallet); err != nil {
		msg := err.Error()
		if strings.Contains(msg, "already loaded") {
			return nil
		}
		return fmt.Errorf("loadwallet %q failed: %w", wallet, err)
	}

	return nil
}

func random32() []byte {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		die("random generation failed: %v", err)
	}
	return b
}

func must(err error) {
	if err != nil {
		die("%v", err)
	}
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", args...)
	os.Exit(1)
}
