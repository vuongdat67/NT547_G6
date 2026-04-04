package experiments

import (
	"fmt"
	"math"
)

// Config is a single parameter point from the experiment guide grid.
type Config struct {
	ID                string  `json:"id"`
	VSat              int64   `json:"vSat"`
	VDepSat           int64   `json:"vDepSat"`
	VColSat           int64   `json:"vColSat"`
	VDepOverV         float64 `json:"vDepOverV"`
	VColOverVDep      float64 `json:"vColOverVDep"`
	Kappa             int     `json:"kappa"`
	HopsN             int     `json:"hopsN"`
	HeConditionValid  bool    `json:"heConditionValid"`
	HeConditionReason string  `json:"heConditionReason"`
}

func BuildGridConfigs(vSat int64) []Config {
	vDepRatios := []float64{0.01, 0.025, 0.05, 0.1}
	vColOverVDep := []float64{0.5, 0.75, 1.0}
	kappas := []int{2, 3, 5, 7}
	hops := []int{1, 3, 5, 7}

	out := make([]Config, 0, len(vDepRatios)*len(vColOverVDep)*len(kappas)*len(hops))
	for _, depRatio := range vDepRatios {
		vDep := int64(math.Round(float64(vSat) * depRatio))
		for _, colRatio := range vColOverVDep {
			vCol := int64(math.Round(float64(vDep) * colRatio))
			for _, kappa := range kappas {
				valid, reason := HeCondition(vDep, vCol, kappa)
				for _, n := range hops {
					id := fmt.Sprintf("dep%.3f-col%.2f-k%d-n%d", depRatio, colRatio, kappa, n)
					out = append(out, Config{
						ID:                id,
						VSat:              vSat,
						VDepSat:           vDep,
						VColSat:           vCol,
						VDepOverV:         depRatio,
						VColOverVDep:      colRatio,
						Kappa:             kappa,
						HopsN:             n,
						HeConditionValid:  valid,
						HeConditionReason: reason,
					})
				}
			}
		}
	}
	return out
}

func HeCondition(vDepSat, vColSat int64, kappa int) (bool, string) {
	if kappa <= 2 {
		return false, "kappa must be > 2 by He-HTLC theorem assumptions"
	}
	lower := int64(math.Ceil(float64(vDepSat) / float64(kappa-1)))
	if vColSat < lower {
		return false, fmt.Sprintf("v_col(%d) < ceil(v_dep/(kappa-1))(%d)", vColSat, lower)
	}
	if vColSat > vDepSat {
		return false, fmt.Sprintf("v_col(%d) > v_dep(%d)", vColSat, vDepSat)
	}
	return true, "valid"
}

func LinkedWidth(vSat, vDepSat, vColSat, cSat int64) int64 {
	return (vSat + vDepSat - cSat) - vColSat
}

func CStar(vSat, vDepSat, vColSat int64) int64 {
	return vSat + vDepSat - vColSat
}

func CNStar(vSat, vDepSat, vColSat int64, hops int) int64 {
	return vSat + int64(hops)*(vDepSat-vColSat)
}
