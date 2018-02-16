package aws

import "github.com/aws/aws-sdk-go/aws/session"

func importStub() {
	sess := session.Must(session.NewSession())
	_ = sess
}
