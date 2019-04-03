package controller

import (
	"math"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// calcScaleUpDelta determines the amount of nodes to scale up
func calcScaleUpDelta(allNodes []*v1.Node, cpuPercent, memPercent float64, cpuRequest, memRequest resource.Quantity, nodeGroup *NodeGroupState) (int, error) {
	nodeCount := float64(len(allNodes))
	scaleUpThresholdPercent := float64(nodeGroup.Opts.ScaleUpThresholdPercent)

	var nodesNeededCPU, nodesNeededMem float64
	// Scale up node group when it's zero
	if cpuPercent == math.MaxFloat64 || memPercent == math.MaxFloat64 {
		if nodeGroup.cpuCapacity.IsZero() || nodeGroup.memCapacity.IsZero() {
			// there is no cached node capacity available
			// scale up by 1
			log.WithField("nodegroup", nodeGroup.Opts.Name).Debug("scale up node group by 1 from 0 as there is no cached version of node capacity")
			return 1, nil
		}
		log.WithField(
			"nodegroup",
			nodeGroup.Opts.Name).Debugf("scale up node group from 0 based on cached nodes cpu capacity: %s, nodes memory capacity: %s",
			nodeGroup.cpuCapacity.String(), nodeGroup.memCapacity.String())
		nodesNeededCPU = math.Ceil(float64(cpuRequest.MilliValue()) / float64(nodeGroup.cpuCapacity.MilliValue()) / scaleUpThresholdPercent * 100)
		nodesNeededMem = math.Ceil(float64(memRequest.MilliValue()) / float64(nodeGroup.memCapacity.MilliValue()) / scaleUpThresholdPercent * 100)
	} else {
		percentageNeededCPU := (cpuPercent - scaleUpThresholdPercent) / scaleUpThresholdPercent
		percentageNeededMem := (memPercent - scaleUpThresholdPercent) / scaleUpThresholdPercent

		nodesNeededCPU = math.Ceil(nodeCount * (percentageNeededCPU))
		nodesNeededMem = math.Ceil(nodeCount * (percentageNeededMem))
	}

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
func calcPercentUsage(cpuRequest, memRequest, cpuCapacity, memCapacity resource.Quantity, numberOfUntaintedNodes int64) (float64, float64, error) {

	mCPUReq, mMemReq, mCPUCap, mMemCap := cpuRequest.MilliValue(), memRequest.MilliValue(), cpuCapacity.MilliValue(), memCapacity.MilliValue()

	// in this case there is already 0 usage and 0 request. Escalator should do nothing
	if allEqual(0, mCPUReq, mMemReq, mCPUCap, mMemCap, numberOfUntaintedNodes) {
		return 0, 0, nil
	}

	if cpuCapacity.MilliValue() == 0 || memCapacity.MilliValue() == 0 {
		// Needs to return nil for just in case if node group untainted nodes size is zero
		// which means percent of usage will be ∞
		// use math.MaxFloat64 here which will trigger a scale-up
		if numberOfUntaintedNodes == 0 {
			return math.MaxFloat64, math.MaxFloat64, nil
		}

		return 0, 0, errors.New("cannot divide by zero in percent calculation")
	}

	cpuPercent := float64(cpuRequest.MilliValue()) / float64(cpuCapacity.MilliValue()) * 100
	memPercent := float64(memRequest.MilliValue()) / float64(memCapacity.MilliValue()) * 100
	return cpuPercent, memPercent, nil
}
