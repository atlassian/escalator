package k8s_test

import (
	"testing"

	"github.com/atlassian/escalator/pkg/k8s"
	"github.com/atlassian/escalator/test"
	"github.com/stretchr/testify/assert"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestCalculateNodesCapacityTotal(t *testing.T) {

	n1 := test.BuildTestNode(test.NodeOpts{
		Name: "n1",
		CPU:  1000,
		Mem:  1000,
	})
	n2 := test.BuildTestNode(test.NodeOpts{
		Name: "n2",
		CPU:  1000,
		Mem:  1000,
	})
	n3 := test.BuildTestNode(test.NodeOpts{
		Name: "n3",
		CPU:  300,
		Mem:  800,
	})
	n4 := test.BuildTestNode(test.NodeOpts{
		Name: "n4",
		CPU:  200,
		Mem:  50000,
	})
	n5 := test.BuildTestNode(test.NodeOpts{
		Name: "n5",
		CPU:  0,
		Mem:  0,
	})

	type args struct {
		nodes []*v1.Node
	}
	tests := []struct {
		name string
		args args
		mem  resource.Quantity
		cpu  resource.Quantity
	}{
		{
			"test 1000 + 1000 == 2000",
			args{
				[]*v1.Node{n1, n2},
			},
			*resource.NewQuantity(2000, resource.DecimalSI),
			*resource.NewMilliQuantity(2000, resource.DecimalSI),
		},
		{
			"test 1000,1000 + 300,800 == 1300,1800",
			args{
				[]*v1.Node{n1, n3},
			},
			*resource.NewQuantity(1800, resource.DecimalSI),
			*resource.NewMilliQuantity(1300, resource.DecimalSI),
		},
		{
			"test 1000,1000 + 300,800 + 200,50000 == 1500,51800",
			args{
				[]*v1.Node{n1, n3, n4},
			},
			*resource.NewQuantity(51800, resource.DecimalSI),
			*resource.NewMilliQuantity(1500, resource.DecimalSI),
		},
		{
			"test 1000+0 = 1000",
			args{
				[]*v1.Node{n1, n5},
			},
			*resource.NewQuantity(1000, resource.DecimalSI),
			*resource.NewMilliQuantity(1000, resource.DecimalSI),
		},
		{
			"test 0+1000 = 1000",
			args{
				[]*v1.Node{n5, n1},
			},
			*resource.NewQuantity(1000, resource.DecimalSI),
			*resource.NewMilliQuantity(1000, resource.DecimalSI),
		},
		{
			"test 0",
			args{
				[]*v1.Node{n5},
			},
			*resource.NewQuantity(0, resource.DecimalSI),
			*resource.NewQuantity(0, resource.DecimalSI),
		},
		{
			"test 0+0",
			args{
				[]*v1.Node{n5, n5},
			},
			*resource.NewQuantity(0, resource.DecimalSI),
			*resource.NewQuantity(0, resource.DecimalSI),
		},
	}
	for _, tt := range tests {
		mem, cpu, err := k8s.CalculateNodesCapacityTotal(tt.args.nodes)
		assert.Equal(t, tt.mem, mem)
		assert.Equal(t, tt.cpu, cpu)
		assert.NoError(t, err)
	}
}
