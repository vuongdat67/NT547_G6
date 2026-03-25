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
	FundTxID           string `json:"fundTxid"`
	FundVout           uint32 `json:"fundVout"`
	FundValueSat       int64  `json:"fundValueSat"`
	SpendTxID          string `json:"spendTxid"`
	SpendValueSat      int64  `json:"spendValueSat"`
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
		fundSat      = flag.Int64("fund-sat", 10000, "Funding amount in satoshis sent to linked ACS output")
		feeSat       = flag.Int64("fee-sat", 1000, "Fee in satoshis for spending linked ACS output")
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
	requiredSat := *fundSat + *feeSat
	if balanceSat < requiredSat {
		die("insufficient funds in wallet %q on %s: balance=%d sat, required>=%d sat (fund-sat + fee-sat)", *wallet, *network, balanceSat, requiredSat)
	}

	fmt.Println("[2/9] Generating secrets and linked ACS script...")
	rjA := random32()
	preB := random32()
	hashRjA := sha256.Sum256(rjA)
	hashPreB := sha256.Sum256(preB)

	linkedScript, err := buildLinkedACSScript(hashRjA[:], hashPreB[:])
	must(err)

	params := &chaincfg.RegressionNetParams
	if *network == "signet" {
		params = &chaincfg.SigNetParams
	}
	scriptHash := sha256.Sum256(linkedScript)
	addr, err := btcutil.NewAddressWitnessScriptHash(scriptHash[:], params)
	must(err)

	fmt.Printf("  linked ACS address: %s\n", addr.EncodeAddress())
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

	fmt.Println("[5/9] Building spend transaction with corrected witness order...")
	spendValue := fundValueSat - *feeSat
	if spendValue <= 0 {
		die("funding value %d is not enough after fee %d", fundValueSat, *feeSat)
	}

	destAddr, err := cli.run(true, "getnewaddress", "linked_acs_test")
	must(err)
	destAddr = strings.TrimSpace(destAddr)
	aiRaw, err := cli.run(true, "getaddressinfo", destAddr)
	must(err)
	var ai addressInfo
	must(json.Unmarshal([]byte(aiRaw), &ai))
	if ai.ScriptPubKey == "" {
		die("empty scriptPubKey for destination address %s", destAddr)
	}
	destPkScript, err := hex.DecodeString(ai.ScriptPubKey)
	must(err)

	hash, err := chainhash.NewHashFromStr(fundTxID)
	must(err)
	spendTx := wire.NewMsgTx(wire.TxVersion)
	spendTx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: *hash, Index: fundVout},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	spendTx.AddTxOut(&wire.TxOut{Value: spendValue, PkScript: destPkScript})

	// Correct witness order for script:
	// OP_SHA256 <H(rjA)> OP_EQUALVERIFY OP_SHA256 <H(preB)> OP_EQUALVERIFY OP_TRUE
	// First OP_SHA256 consumes top stack item => rjA must be on top.
	// Witness stack items are pushed in listed order, so preB must come before rjA.
	spendTx.TxIn[0].Witness = wire.TxWitness{
		preB,
		rjA,
		linkedScript,
	}

	var buf bytes.Buffer
	must(spendTx.Serialize(&buf))
	spendHex := hex.EncodeToString(buf.Bytes())

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
	spendTxID, err := cli.run(false, "sendrawtransaction", spendHex)
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
		FundTxID:           fundTxID,
		FundVout:           fundVout,
		FundValueSat:       fundValueSat,
		SpendTxID:          spendTxID,
		SpendValueSat:      spendValue,
		FeeSat:             *feeSat,
		WitnessOrder:       "<pre_b> <r^j_a> <redeemScript>",
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

func buildLinkedACSScript(hashRjA, hashPreB []byte) ([]byte, error) {
	b := txscript.NewScriptBuilder()
	b.AddOp(txscript.OP_SHA256)
	b.AddData(hashRjA)
	b.AddOp(txscript.OP_EQUALVERIFY)
	b.AddOp(txscript.OP_SHA256)
	b.AddData(hashPreB)
	b.AddOp(txscript.OP_EQUALVERIFY)
	b.AddOp(txscript.OP_TRUE)
	return b.Script()
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
