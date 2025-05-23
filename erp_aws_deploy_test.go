package main

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/jsii-runtime-go"
)

// example tests. To run these tests, uncomment this file along with the
// example resource in erp_aws_deploy_test.go
func TestErpAwsDeployStack(t *testing.T) {
	// GIVEN
	app := awscdk.NewApp(nil)

	// WHEN
	stack := NewErpAwsDeployStack(app, "MyStack", nil)

	// THEN
	template := assertions.Template_FromStack(stack, nil)

	template.HasResourceProperties(jsii.String("AWS::ECS::Service"), map[string]interface{}{
		"LaunchType": "EC2",
	})
}
