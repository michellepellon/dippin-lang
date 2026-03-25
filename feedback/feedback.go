package feedback

import (
	"math"

	"github.com/2389-research/dippin-lang/cost"
)

// Report is the full cost calibration result.
type Report struct {
	Nodes    []NodeComparison `json:"nodes"`
	Accuracy float64          `json:"accuracy_pct"`
	Outliers []Outlier        `json:"outliers"`
}

// NodeComparison shows predicted vs actual cost for a single node.
type NodeComparison struct {
	NodeID        string  `json:"node_id"`
	PredictedCost float64 `json:"predicted_cost"`
	ActualCost    float64 `json:"actual_cost"`
	Ratio         float64 `json:"ratio"`
}

// Outlier flags a node with >2x or <0.5x predicted/actual ratio.
type Outlier struct {
	NodeID  string  `json:"node_id"`
	Ratio   float64 `json:"ratio"`
	Message string  `json:"message"`
}

// Analyze compares predicted costs against actual telemetry data.
func Analyze(predicted *cost.Report, telemetryPath string) (*Report, error) {
	records, err := ReadTelemetry(telemetryPath)
	if err != nil {
		return nil, err
	}

	actuals := aggregateActualCosts(records)
	r := &Report{}
	r.Nodes = buildComparisons(predicted, actuals)
	r.Accuracy = computeAccuracy(r.Nodes)
	r.Outliers = findOutliers(r.Nodes)
	return r, nil
}

// aggregateActualCosts sums actual costs per node from telemetry.
func aggregateActualCosts(records []TelemetryRecord) map[string]float64 {
	costs := make(map[string]float64)
	for _, rec := range records {
		if rec.Node == "" {
			continue
		}
		costs[rec.Node] += rec.ActualCost
	}
	return costs
}

// buildComparisons creates a NodeComparison for each node with telemetry.
func buildComparisons(predicted *cost.Report, actuals map[string]float64) []NodeComparison {
	var comparisons []NodeComparison
	for nodeID, actual := range actuals {
		pc, ok := predicted.Nodes[nodeID]
		if !ok {
			continue
		}
		ratio := computeRatio(pc.Cost.Expected, actual)
		comparisons = append(comparisons, NodeComparison{
			NodeID:        nodeID,
			PredictedCost: pc.Cost.Expected,
			ActualCost:    actual,
			Ratio:         ratio,
		})
	}
	return comparisons
}

// computeRatio calculates the predicted/actual ratio.
func computeRatio(predicted, actual float64) float64 {
	if actual == 0 {
		return 0
	}
	return predicted / actual
}

// computeAccuracy calculates the average accuracy across all comparisons.
func computeAccuracy(comparisons []NodeComparison) float64 {
	if len(comparisons) == 0 {
		return 0
	}
	totalAccuracy := 0.0
	for _, c := range comparisons {
		totalAccuracy += nodeAccuracy(c.Ratio)
	}
	return totalAccuracy / float64(len(comparisons)) * 100
}

// nodeAccuracy converts a ratio to an accuracy percentage (1.0 = 100%).
func nodeAccuracy(ratio float64) float64 {
	if ratio == 0 {
		return 0
	}
	if ratio > 1 {
		return 1 / ratio
	}
	return ratio
}

// findOutliers returns nodes with significant prediction errors.
func findOutliers(comparisons []NodeComparison) []Outlier {
	var outliers []Outlier
	for _, c := range comparisons {
		if o := checkOutlier(c); o != nil {
			outliers = append(outliers, *o)
		}
	}
	return outliers
}

// checkOutlier returns an Outlier if the ratio is outside [0.5, 2.0].
func checkOutlier(c NodeComparison) *Outlier {
	if c.Ratio == 0 {
		return nil
	}
	if c.Ratio > 2.0 {
		return &Outlier{
			NodeID:  c.NodeID,
			Ratio:   math.Round(c.Ratio*100) / 100,
			Message: "predicted cost significantly higher than actual",
		}
	}
	if c.Ratio < 0.5 {
		return &Outlier{
			NodeID:  c.NodeID,
			Ratio:   math.Round(c.Ratio*100) / 100,
			Message: "predicted cost significantly lower than actual",
		}
	}
	return nil
}
