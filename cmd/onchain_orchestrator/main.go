package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/crab-he/internal/experiments"
)

const defaultVSat int64 = 2_000_000

const mempoolChainPolicyMarker = "too-long-mempool-chain"

type deployArtifact struct {
	FundTxID     string `json:"fundTxid"`
	SpendTxID    string `json:"spendTxid"`
	CreatedAtUTC string `json:"createdAtUtc"`
}

type runResult struct {
	ConfigID     string `json:"configId"`
	Seed         int    `json:"seed"`
	Network      string `json:"network"`
	Success      bool   `json:"success"`
	DurationMs   int64  `json:"durationMs"`
	ArtifactPath string `json:"artifactPath"`
	FundTxID     string `json:"fundTxid,omitempty"`
	SpendTxID    string `json:"spendTxid,omitempty"`
	Command      string `json:"command"`
	DryRun       bool   `json:"dryRun"`
	Error        string `json:"error,omitempty"`
}

type networkSummary struct {
	Network     string  `json:"network"`
	TotalRuns   int     `json:"totalRuns"`
	SuccessRuns int     `json:"successRuns"`
	FailureRuns int     `json:"failureRuns"`
	SuccessRate float64 `json:"successRate"`
}

type orchestratorSummary struct {
	GeneratedAtUTC   string           `json:"generatedAtUtc"`
	DryRun           bool             `json:"dryRun"`
	Wallet           string           `json:"wallet"`
	Networks         []string         `json:"networks"`
	SeedRuns         int              `json:"seedRuns"`
	ConfigCount      int              `json:"configCount"`
	TotalRuns        int              `json:"totalRuns"`
	SuccessfulRuns   int              `json:"successfulRuns"`
	FailedRuns       int              `json:"failedRuns"`
	NetworkSummaries []networkSummary `json:"networkSummaries"`
	Results          []runResult      `json:"results"`
}

func main() {
	var (
		networksFlag     = flag.String("networks", "regtest,signet", "Comma-separated networks")
		wallet           = flag.String("wallet", "test", "Wallet name used by deploy script")
		seedRuns         = flag.Int("seed-runs", 30, "Number of seeds per config")
		maxConfigs       = flag.Int("max-configs", 0, "Limit number of configs (0 = all)")
		configOffset     = flag.Int("config-offset", 0, "Start offset in config list")
		fundSat          = flag.Int64("fund-sat", 10000, "Funding amount in satoshis")
		feeSat           = flag.Int64("fee-sat", 1000, "Fee amount in satoshis")
		dryRun           = flag.Bool("dry-run", true, "Plan commands without broadcasting transactions")
		continueOnError  = flag.Bool("continue-on-error", true, "Continue when one run fails")
		createIfMissing  = flag.Bool("create-wallet-if-missing", false, "Create wallet if missing")
		deployScriptPath = flag.String("deploy-script", "./scripts/deploy_linked_acs.go", "Path to linked ACS deploy script")
		artifactRoot     = flag.String("artifact-root", filepath.Join("artifacts", "onchain"), "Output folder for run artifacts")
		retryAttempts    = flag.Int("retry-attempts", 0, "Retry failed broadcast this many times when mempool-chain policy rejects the transaction")
		retryDelayMs     = flag.Int("retry-delay-ms", 0, "Delay between retry attempts in milliseconds")
	)
	flag.Parse()

	if *seedRuns <= 0 {
		die("seed-runs must be > 0")
	}
	if *fundSat <= 0 || *feeSat <= 0 || *fundSat <= *feeSat {
		die("invalid fund/fee values: require fund-sat > fee-sat > 0")
	}
	if *retryAttempts < 0 || *retryDelayMs < 0 {
		die("retry-attempts and retry-delay-ms must be >= 0")
	}

	networks := parseNetworks(*networksFlag)
	if len(networks) == 0 {
		die("no valid networks provided")
	}

	configs := experiments.BuildGridConfigs(defaultVSat)
	if *configOffset < 0 || *configOffset > len(configs) {
		die("config-offset out of range: %d", *configOffset)
	}
	configs = configs[*configOffset:]
	if *maxConfigs > 0 && *maxConfigs < len(configs) {
		configs = configs[:*maxConfigs]
	}

	must(os.MkdirAll(*artifactRoot, 0o755))
	results := make([]runResult, 0, len(configs)*len(networks)*(*seedRuns))

	fmt.Printf("Running on-chain orchestrator over %d configs, %d seeds, %d networks (dry-run=%t)\n", len(configs), *seedRuns, len(networks), *dryRun)

	for _, cfg := range configs {
		for seed := 1; seed <= *seedRuns; seed++ {
			for _, network := range networks {
				start := time.Now()
				artifactPath := filepath.Join(*artifactRoot, network, cfg.ID, fmt.Sprintf("seed_%03d.json", seed))
				must(os.MkdirAll(filepath.Dir(artifactPath), 0o755))

				args := []string{
					"run", *deployScriptPath,
					"-network", network,
					"-wallet", *wallet,
					"-fund-sat", fmt.Sprintf("%d", *fundSat),
					"-fee-sat", fmt.Sprintf("%d", *feeSat),
					"-artifact", artifactPath,
					"-try-load-wallet",
				}
				if *createIfMissing {
					args = append(args, "-create-wallet-if-missing")
				}
				if network == "regtest" {
					args = append(args, "-auto-mine-regtest")
				}
				commandDisplay := "go " + strings.Join(args, " ")

				if *dryRun {
					results = append(results, runResult{
						ConfigID:     cfg.ID,
						Seed:         seed,
						Network:      network,
						Success:      true,
						DurationMs:   time.Since(start).Milliseconds(),
						ArtifactPath: artifactPath,
						Command:      commandDisplay,
						DryRun:       true,
					})
					continue
				}

				out, err := runDeployWithRetry(args, *retryAttempts, *retryDelayMs)
				if err != nil {
					r := runResult{
						ConfigID:     cfg.ID,
						Seed:         seed,
						Network:      network,
						Success:      false,
						DurationMs:   time.Since(start).Milliseconds(),
						ArtifactPath: artifactPath,
						Command:      commandDisplay,
						DryRun:       false,
						Error:        strings.TrimSpace(string(out)),
					}
					results = append(results, r)
					if !*continueOnError {
						writeOutputs(*artifactRoot, summarize(*dryRun, *wallet, networks, *seedRuns, len(configs), results))
						die("orchestrator stopped on error (%s %s seed=%d)", cfg.ID, network, seed)
					}
					continue
				}

				artifact, parseErr := readDeployArtifact(artifactPath)
				if parseErr != nil {
					results = append(results, runResult{
						ConfigID:     cfg.ID,
						Seed:         seed,
						Network:      network,
						Success:      false,
						DurationMs:   time.Since(start).Milliseconds(),
						ArtifactPath: artifactPath,
						Command:      commandDisplay,
						DryRun:       false,
						Error:        parseErr.Error(),
					})
					if !*continueOnError {
						writeOutputs(*artifactRoot, summarize(*dryRun, *wallet, networks, *seedRuns, len(configs), results))
						die("orchestrator stopped due to artifact parse error")
					}
					continue
				}

				results = append(results, runResult{
					ConfigID:     cfg.ID,
					Seed:         seed,
					Network:      network,
					Success:      true,
					DurationMs:   time.Since(start).Milliseconds(),
					ArtifactPath: artifactPath,
					FundTxID:     artifact.FundTxID,
					SpendTxID:    artifact.SpendTxID,
					Command:      commandDisplay,
					DryRun:       false,
				})
			}
		}
	}

	summary := summarize(*dryRun, *wallet, networks, *seedRuns, len(configs), results)
	pubSummary := writeOutputs(*artifactRoot, summary)

	fmt.Printf("On-chain orchestrator finished (raw): %d total runs, %d success, %d failed\n", summary.TotalRuns, summary.SuccessfulRuns, summary.FailedRuns)
	fmt.Printf("Publication summary (kappa>2): %d total runs, %d success, %d failed\n", pubSummary.TotalRuns, pubSummary.SuccessfulRuns, pubSummary.FailedRuns)
	fmt.Printf("Summary JSON: %s\n", filepath.Join(*artifactRoot, "repeated_onchain_summary.json"))
	fmt.Printf("Summary CSV : %s\n", filepath.Join(*artifactRoot, "repeated_onchain_runs.csv"))
}

func parseNetworks(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		n := strings.ToLower(strings.TrimSpace(p))
		if n == "regtest" || n == "signet" {
			out = append(out, n)
		}
	}
	return out
}

func summarize(dryRun bool, wallet string, networks []string, seedRuns int, configCount int, results []runResult) orchestratorSummary {
	networkMap := make(map[string]*networkSummary, len(networks))
	for _, n := range networks {
		networkMap[n] = &networkSummary{Network: n}
	}

	success := 0
	failed := 0
	for _, r := range results {
		ns, ok := networkMap[r.Network]
		if !ok {
			ns = &networkSummary{Network: r.Network}
			networkMap[r.Network] = ns
		}
		ns.TotalRuns++
		if r.Success {
			success++
			ns.SuccessRuns++
		} else {
			failed++
			ns.FailureRuns++
		}
	}

	networkSummaries := make([]networkSummary, 0, len(networkMap))
	for _, n := range networks {
		ns := networkMap[n]
		if ns.TotalRuns > 0 {
			ns.SuccessRate = float64(ns.SuccessRuns) / float64(ns.TotalRuns)
		}
		networkSummaries = append(networkSummaries, *ns)
	}

	return orchestratorSummary{
		GeneratedAtUTC:   time.Now().UTC().Format(time.RFC3339),
		DryRun:           dryRun,
		Wallet:           wallet,
		Networks:         networks,
		SeedRuns:         seedRuns,
		ConfigCount:      configCount,
		TotalRuns:        len(results),
		SuccessfulRuns:   success,
		FailedRuns:       failed,
		NetworkSummaries: networkSummaries,
		Results:          results,
	}
}

func writeOutputs(root string, summary orchestratorSummary) orchestratorSummary {
	pubSummary := publicationSummary(summary)
	jsonPath := filepath.Join(root, "repeated_onchain_summary.json")
	csvPath := filepath.Join(root, "repeated_onchain_runs.csv")
	must(writeJSON(jsonPath, pubSummary))
	must(writeCSV(csvPath, pubSummary.Results))
	return pubSummary
}

func publicationSummary(summary orchestratorSummary) orchestratorSummary {
	filteredRows := make([]runResult, 0, len(summary.Results))
	for _, r := range summary.Results {
		if strings.Contains(r.ConfigID, "-k2-") {
			continue
		}
		filteredRows = append(filteredRows, r)
	}

	configSet := make(map[string]struct{})
	for _, r := range filteredRows {
		configSet[r.ConfigID] = struct{}{}
	}

	return summarize(
		summary.DryRun,
		summary.Wallet,
		summary.Networks,
		summary.SeedRuns,
		len(configSet),
		filteredRows,
	)
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func writeCSV(path string, rows []runResult) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	head := []string{"config_id", "seed", "network", "success", "duration_ms", "artifact_path", "fund_txid", "spend_txid", "dry_run", "error", "command"}
	if err := w.Write(head); err != nil {
		return err
	}

	for _, r := range rows {
		if strings.Contains(r.ConfigID, "-k2-") {
			continue
		}
		rec := []string{
			r.ConfigID,
			fmt.Sprintf("%d", r.Seed),
			r.Network,
			fmt.Sprintf("%t", r.Success),
			fmt.Sprintf("%d", r.DurationMs),
			r.ArtifactPath,
			r.FundTxID,
			r.SpendTxID,
			fmt.Sprintf("%t", r.DryRun),
			r.Error,
			r.Command,
		}
		if err := w.Write(rec); err != nil {
			return err
		}
	}

	return w.Error()
}

func readDeployArtifact(path string) (deployArtifact, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return deployArtifact{}, err
	}
	var out deployArtifact
	if err := json.Unmarshal(b, &out); err != nil {
		return deployArtifact{}, err
	}
	return out, nil
}

func runDeployWithRetry(args []string, retryAttempts int, retryDelayMs int) ([]byte, error) {
	maxAttempts := retryAttempts + 1
	var out []byte
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		cmd := exec.Command("go", args...)
		out, err = cmd.CombinedOutput()
		if err == nil {
			return out, nil
		}
		if attempt == maxAttempts || !isRetryablePolicyError(out) {
			break
		}
		if retryDelayMs > 0 {
			time.Sleep(time.Duration(retryDelayMs) * time.Millisecond)
		}
	}
	return out, err
}

func isRetryablePolicyError(out []byte) bool {
	errText := strings.ToLower(strings.TrimSpace(string(out)))
	return strings.Contains(errText, mempoolChainPolicyMarker)
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
