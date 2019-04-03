package controller

import (
	"math"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// calcScaleUpDelta determines the amount of nodes to scale up
func calcScaleUpDelta(allNodes []*v1.Node, cpuPercent float64, memPercent float64, nodeGroup *NodeGroupState) (int, error) {
	nodeCount := float64(len(allNodes))
	scaleUpThresholdPercent := float64(nodeGroup.Opts.ScaleUpThresholdPercent)

	percentageNeededCPU := (cpuPercent - scaleUpThresholdPercent) / scaleUpThresholdPercent
	percentageNeededMem := (memPercent - scaleUpThresholdPercent) / scaleUpThresholdPercent

	nodesNeededCPU := math.Ceil(nodeCount * (percentageNeededCPU))
	nodesNeededMem := math.Ceil(nodeCount * (percentageNeededMem))

	// Determine the delta based on whichever is higher (cpu or mem)
	delta := int(math.Max(nodesNeededCPU, nodesNeededMem))
	if delta < 0 {
		return delta, errors.New("negative scale up delta")
	}
	return delta, nil
}

func allEqual(matchValue int64, resourceValues ...int64) bool {
	for _, v := range resourceValues {
		if v != matchValue {
			return false
		}
	}
	return true
}

// calcPercentUsage helper works out the percentage of cpu and mem for request/capacity
func calcPercentUsage(cpuRequest, memRequest, cpuCapacity, memCapacity resource.Quantity) (float64, float64, error) {

	mCPUReq, mMemReq, mCPUCap, mMemCap := cpuRequest.MilliValue(), memRequest.MilliValue(), cpuCapacity.MilliValue(), memCapacity.MilliValue()

	// in this case there is already 0 usage and 0 request. Escalator should do nothing
	if allEqual(0, mCPUReq, mMemReq, mCPUCap, mMemCap) {
		return 0, 0, nil
	}
	//

	if cpuCapacity.MilliValue() == 0 || memCapacity.MilliValue() == 0 {
		// Needs to return nil for just in case if ASG size is zero.
		return 0, 0, nil
	}

	cpuPercent := float64(cpuRequest.MilliValue()) / float64(cpuCapacity.MilliValue()) * 100
	memPercent := float64(memRequest.MilliValue()) / float64(memCapacity.MilliValue()) * 100
	return cpuPercent, memPercent, nil
}
