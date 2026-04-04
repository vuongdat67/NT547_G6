package experiments

import (
	"crypto/sha256"
	"fmt"
	"math"

	"github.com/crab-he/internal/htlc"
)

// TxStage is a publication-friendly transaction stage summary.
type TxStage struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	VBytes      int    `json:"vbytes"`
	Witness     string `json:"witness"`
	Description string `json:"description"`
}

// Pipeline captures a standalone baseline execution model at transaction level.
type Pipeline struct {
	Baseline          string    `json:"baseline"`
	ConfigID          string    `json:"configId"`
	Seed              int64     `json:"seed"`
	HonestPath        []TxStage `json:"honestPath"`
	AttackPath        []TxStage `json:"attackPath"`
	TotalHonestVBytes int       `json:"totalHonestVBytes"`
	TotalAttackVBytes int       `json:"totalAttackVBytes"`
	AttackWidthSat    int64     `json:"attackWidthSat"`
	Feasible          bool      `json:"feasible"`
	Notes             string    `json:"notes"`
}

// BuildMADStandalone models MAD-HTLC as a standalone contract with explicit tx stages.
func BuildMADStandalone(cfg Config, seed int64) Pipeline {
	honest := []TxStage{
		{
			Name:        "tx_mad_lock",
			Path:        "lock",
			VBytes:      176,
			Witness:     "<sig_A> <sig_B>",
			Description: "Create MAD standalone lock UTXO for deposit and collateral.",
		},
		{
			Name:        "tx_mad_claim_A",
			Path:        "honest-claim",
			VBytes:      188,
			Witness:     "<sig_A> <pre_a>",
			Description: "Alice reveals preimage and redeems deposit in honest path.",
		},
	}

	attack := []TxStage{
		{
			Name:        "tx_mad_lock",
			Path:        "lock",
			VBytes:      176,
			Witness:     "<sig_A> <sig_B>",
			Description: "Create MAD standalone lock UTXO for deposit and collateral.",
		},
		{
			Name:        "tx_mad_bribe_censor",
			Path:        "attack-censor",
			VBytes:      152,
			Witness:     "<sig_B>",
			Description: "Bob executes timeout-side recovery while miners censor Alice claim.",
		},
	}

	width := cfg.VDepSat - cfg.VColSat
	return Pipeline{
		Baseline:          "MAD-HTLC standalone",
		ConfigID:          cfg.ID,
		Seed:              seed,
		HonestPath:        honest,
		AttackPath:        attack,
		TotalHonestVBytes: totalVBytes(honest),
		TotalAttackVBytes: totalVBytes(attack),
		AttackWidthSat:    width,
		Feasible:          width > 0,
		Notes:             "Transaction-level standalone MAD path; positive width indicates economically feasible attack interval.",
	}
}

// BuildHeStandalone models He-HTLC as a standalone protocol using explicit tx paths.
func BuildHeStandalone(cfg Config, seed int64) Pipeline {
	alicePK := "alice_pk_sample"
	bobPK := "bob_pk_sample"
	preA := seedBytes(cfg.ID, seed, "preA")
	preB := seedBytes(cfg.ID, seed, "preB")
	hA := sha256.Sum256(preA)
	hB := sha256.Sum256(preB)

	h := htlc.NewHTLC(cfg.VDepSat, cfg.VColSat, hA[:], hB[:], 288, 6, alicePK, bobPK)
	h.Dep.TxID = fmt.Sprintf("dep-%s-%d", cfg.ID, seed)
	h.Col.TxID = fmt.Sprintf("col-%s-%d", cfg.ID, seed)

	honestDepA := htlc.DepA(h.Dep, preA)
	attackDepB := htlc.DepB(h.Dep, preB)
	attackColB := htlc.ColB(h.Col)
	enforceColM := htlc.ColM(h.Col, preA, preB, "miner_sample")

	hPath := []TxStage{
		{
			Name:        honestDepA.Label,
			Path:        honestDepA.Path,
			VBytes:      honestDepA.SizeVB,
			Witness:     honestDepA.Witness,
			Description: honestDepA.Note,
		},
		{
			Name:        enforceColM.Label,
			Path:        enforceColM.Path,
			VBytes:      enforceColM.SizeVB,
			Witness:     enforceColM.Witness,
			Description: "He enforcement path for miner claim once both secrets are exposed.",
		},
	}

	aPath := []TxStage{
		{
			Name:        attackDepB.Label,
			Path:        attackDepB.Path,
			VBytes:      attackDepB.SizeVB,
			Witness:     attackDepB.Witness,
			Description: attackDepB.Note,
		},
		{
			Name:        attackColB.Label,
			Path:        attackColB.Path,
			VBytes:      attackColB.SizeVB,
			Witness:     attackColB.Witness,
			Description: attackColB.Note,
		},
	}

	lower := int64(math.Ceil(float64(cfg.VDepSat) / float64(max(1, cfg.Kappa-1))))
	width := lower - cfg.VColSat
	return Pipeline{
		Baseline:          "He-HTLC standalone",
		ConfigID:          cfg.ID,
		Seed:              seed,
		HonestPath:        hPath,
		AttackPath:        aPath,
		TotalHonestVBytes: totalVBytes(hPath),
		TotalAttackVBytes: totalVBytes(aPath),
		AttackWidthSat:    width,
		Feasible:          width > 0,
		Notes:             "Transaction-level standalone He path; non-positive width indicates no profitable miner-assisted deviation under theorem bounds.",
	}
}

func seedBytes(configID string, seed int64, label string) []byte {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%s", configID, seed, label)))
	out := make([]byte, len(h))
	copy(out, h[:])
	return out
}

func totalVBytes(stages []TxStage) int {
	total := 0
	for _, s := range stages {
		total += s.VBytes
	}
	return total
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
