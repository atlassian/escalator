package controller

import (
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"

	"github.com/atlassian/escalator/pkg/test"
)

func TestSortOldestNode(t *testing.T) {
	// Ordered nodes for testing
	nodesOrdered := []*v1.Node{
		test.BuildTestNode(test.NodeOpts{
			Creation: time.Date(1996, time.May, 12, 9, 0, 0, 0, time.UTC),
		}),
		test.BuildTestNode(test.NodeOpts{
			Creation: time.Date(2018, time.January, 1, 1, 0, 0, 0, time.UTC),
		}),
		test.BuildTestNode(test.NodeOpts{
			Creation: time.Date(2018, time.January, 1, 1, 1, 0, 0, time.UTC),
		}),
		test.BuildTestNode(test.NodeOpts{
			Creation: time.Date(2018, time.January, 1, 1, 1, 1, 0, time.UTC),
		}),
		test.BuildTestNode(test.NodeOpts{
			Creation: time.Date(2018, time.January, 1, 1, 1, 1, 1, time.UTC),
		}),
		test.BuildTestNode(test.NodeOpts{
			Creation: time.Date(2020, time.December, 2, 2, 2, 2, 2, time.UTC),
		}),
	}

	// Shuffle with the nodesByOldestCreationTime bundle
	shuffled := make(nodesByOldestCreationTime, 0, len(nodesOrdered))
	for i, node := range nodesOrdered {
		shuffled = append(shuffled, nodeIndexBundle{node, i})
	}
	shuffleOldest(shuffled)

	// keep track of order before sorting
	shuffledOrder := make([]int, 0, len(shuffled))
	for i := range shuffled {
		shuffledOrder = append(shuffledOrder, shuffled[i].index)
	}

	// sort and test
	sort.Sort(shuffled)
	for i, bundle := range shuffled {
		t.Run("sort oldest", func(t *testing.T) {
			assert.Equal(t, nodesOrdered[i], bundle.node)
			assert.Equal(t, i, shuffledOrder[i])
		})
	}
}

func TestSortNewestNode(t *testing.T) {
	// Ordered nodes for testing
	nodesOrdered := []*v1.Node{
		test.BuildTestNode(test.NodeOpts{
			Creation: time.Date(2020, time.December, 2, 2, 2, 2, 2, time.UTC),
		}),
		test.BuildTestNode(test.NodeOpts{
			Creation: time.Date(2018, time.January, 1, 1, 1, 1, 1, time.UTC),
		}),
		test.BuildTestNode(test.NodeOpts{
			Creation: time.Date(2018, time.January, 1, 1, 1, 1, 0, time.UTC),
		}),
		test.BuildTestNode(test.NodeOpts{
			Creation: time.Date(2018, time.January, 1, 1, 1, 0, 0, time.UTC),
		}),
		test.BuildTestNode(test.NodeOpts{
			Creation: time.Date(2018, time.January, 1, 1, 0, 0, 0, time.UTC),
		}),
		test.BuildTestNode(test.NodeOpts{
			Creation: time.Date(1996, time.May, 12, 9, 0, 0, 0, time.UTC),
		}),
	}

	// Shuffle with the nodesByNewestCreationTime bundle
	shuffled := make(nodesByNewestCreationTime, 0, len(nodesOrdered))
	for i, node := range nodesOrdered {
		shuffled = append(shuffled, nodeIndexBundle{node, i})
	}
	shuffleNewest(shuffled)

	// keep track of order before sorting
	shuffledOrder := make([]int, 0, len(shuffled))
	for i := range shuffled {
		shuffledOrder = append(shuffledOrder, shuffled[i].index)
	}

	// sort and test
	sort.Sort(shuffled)
	for i, bundle := range shuffled {
		t.Run("sort newest", func(t *testing.T) {
			assert.Equal(t, nodesOrdered[i], bundle.node)
			assert.Equal(t, i, shuffledOrder[i])
		})
	}
}

// shuffle shuffles the nodes and also swap their original indices for testing
func shuffleOldest(nodes nodesByOldestCreationTime) {
	for i := range nodes {
		j := rand.Intn(i + 1)
		nodes[i], nodes[j] = nodes[j], nodes[i]
		nodes[i].index, nodes[j].index = nodes[j].index, nodes[i].index
	}
}

// shuffle shuffles the nodes and also swap their original indices for testing
func shuffleNewest(nodes nodesByNewestCreationTime) {
	for i := range nodes {
		j := rand.Intn(i + 1)
		nodes[i], nodes[j] = nodes[j], nodes[i]
		nodes[i].index, nodes[j].index = nodes[j].index, nodes[i].index
	}
}
