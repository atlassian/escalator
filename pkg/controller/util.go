package controller

import (
	"errors"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"math"
)

// calcNodeWorth determines the worth of a list of nodes
// For example, 200 nodes = 0.5 node worth i.e. 5% of total nodes
func calcNodeWorth(nodes []*v1.Node) (float64, error) {
	if len(nodes) == 0 {
		return 0, errors.New("cannot divide by zero in percent calculation")
	}
	// We are assuming that all nodes have the same capacity
	return 1.0 / float64(len(nodes)) * 100.0, nil
}

// calcScaleUpDelta determines the amount of nodes to scale up
func calcScaleUpDelta(allNodes []*v1.Node, cpuPercent float64, memPercent float64, nodeGroup *NodeGroupState) (int, error) {
	nodeWorth, err := calcNodeWorth(allNodes)
	if err != nil {
		return 0, err
	}

	percentageNeededCPU := cpuPercent - float64(nodeGroup.Opts.ScaleUpThreshholdPercent)
	percentageNeededMem := memPercent - float64(nodeGroup.Opts.ScaleUpThreshholdPercent)

	nodesNeededCPU := math.Ceil(percentageNeededCPU / nodeWorth)
	nodesNeededMem := math.Ceil(percentageNeededMem / nodeWorth)

	// Determine the delta based on whichever is higher (cpu or mem)
	delta := int(math.Max(nodesNeededCPU, nodesNeededMem))
	if delta < 0 {
		return delta, errors.New("negative scale up delta")
	}
	return delta, nil
}

// calcPercentUsage helper works out the percentage of cpu and mem for request/capacity
func calcPercentUsage(cpuR, memR, cpuA, memA resource.Quantity) (float64, float64, error) {
	if cpuA.MilliValue() == 0 || memA.MilliValue() == 0 {
		return 0, 0, errors.New("Cannot divide by zero in percent calculation")
	}
	cpuPercent := float64(cpuR.MilliValue()) / float64(cpuA.MilliValue()) * 100
	memPercent := float64(memR.MilliValue()) / float64(memA.MilliValue()) * 100
	return cpuPercent, memPercent, nil
}
