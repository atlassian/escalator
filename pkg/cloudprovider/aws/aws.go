package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
)

func importStub() {
	sess := session.Must(session.NewSession())
	_ = sess
	svc := autoscaling.New(sess, &aws.Config{
		Region: aws.String(endpoints.UsEast1RegionID),
	})
	_ = svc
}
