package controller_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/atlassian/escalator/pkg/controller"
	"github.com/atlassian/escalator/pkg/test"
	"k8s.io/api/core/v1"
)

func TestNewNodeLabelFilterFunc(t *testing.T) {
	buildengNode := test.BuildTestNode(test.NodeOpts{
		Name:       "buildeng-node",
		CPU:        1000,
		Mem:        1000,
		LabelKey:   "customer",
		LabelValue: "buildeng",
	})
	badKeyNode := test.BuildTestNode(test.NodeOpts{
		Name:       "buildeng-node",
		CPU:        1000,
		Mem:        1000,
		LabelKey:   "wronglabelkey",
		LabelValue: "buildeng",
	})
	badLabelNode := test.BuildTestNode(test.NodeOpts{
		Name:       "buildeng-node",
		CPU:        1000,
		Mem:        1000,
		LabelKey:   "customer",
		LabelValue: "wronglabelkey",
	})
	badBothNode := test.BuildTestNode(test.NodeOpts{
		Name:       "buildeng-node",
		CPU:        1000,
		Mem:        1000,
		LabelKey:   "wronglabelkey",
		LabelValue: "wronglabelkey",
	})

	type args struct {
		labelKey   string
		labelValue string
		node       *v1.Node
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"buildeng customer should pass",
			args{
				"customer",
				"buildeng",
				buildengNode,
			},
			true,
		},
		{
			"buildeng customer should fail",
			args{
				"customer",
				"kitt",
				buildengNode,
			},
			false,
		},
		{
			"buildeng wrong key should fail",
			args{
				"customer",
				"buildeng",
				badKeyNode,
			},
			false,
		},
		{
			"buildeng wrong value should fail",
			args{
				"customer",
				"buildeng",
				badLabelNode,
			},
			false,
		},
		{
			"buildeng bad both should fail",
			args{
				"customer",
				"buildeng",
				badBothNode,
			},
			false,
		},
	}
	for _, tt := range tests {
		f := controller.NewNodeLabelFilterFunc(tt.args.labelKey, tt.args.labelValue)
		got := f(tt.args.node)
		assert.Equal(t, tt.want, got)
	}
}
