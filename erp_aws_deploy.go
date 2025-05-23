package main

import (
	"os"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awselasticache"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type ErpAwsDeployStackProps struct {
	awscdk.StackProps
}

func NewErpAwsDeployStack(scope constructs.Construct, id string, props *ErpAwsDeployStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	// The code that defines your stack goes here

	// example resource
	vpc := awsec2.NewVpc(stack, jsii.String("ERP_VPC"), &awsec2.VpcProps{
		MaxAzs:      jsii.Number(2),
		NatGateways: jsii.Number(1),
		SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
			{
				Name:       jsii.String("Public"),
				SubnetType: awsec2.SubnetType_PUBLIC,
				CidrMask:   jsii.Number(24),
			},
			{
				Name:       jsii.String("Private"),
				SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
				CidrMask:   jsii.Number(24),
			},
		},
	})

	ecsCluster := awsecs.NewCluster(stack, jsii.String("ERP_Cluster"), &awsecs.ClusterProps{
		Vpc: vpc,
	})
	ecsCluster.AddCapacity(jsii.String("AutoScaleCap"), &awsecs.AddCapacityOptions{
		InstanceType: awsec2.NewInstanceType(jsii.String("t3.nano")),
		MaxCapacity:  jsii.Number(4),
		MinCapacity:  jsii.Number(1),
	})

	// Task role?

	// Shared EBS Volume

	// Task Definition for Backend

	backendTaskDef := awsecs.NewEc2TaskDefinition(stack, jsii.String("ERPBackendTask"), &awsecs.Ec2TaskDefinitionProps{
		NetworkMode: awsecs.NetworkMode_BRIDGE,
	})

	backendTaskDef.AddContainer(jsii.String("Backend"), &awsecs.ContainerDefinitionOptions{
		Image:          awsecs.ContainerImage_FromRegistry(jsii.String("frappe/erpnext:v15.62.0"), nil),
		MemoryLimitMiB: jsii.Number(256),
		HealthCheck: &awsecs.HealthCheck{
			Command: jsii.Strings("CMD-SHELL", "docker-compose exec backend healthcheck.sh"),
		},
	})

	frontendTaskDef := awsecs.NewEc2TaskDefinition(stack, jsii.String("ERPFrontendTask"), &awsecs.Ec2TaskDefinitionProps{
		NetworkMode: awsecs.NetworkMode_BRIDGE,
	})

	frontendTaskDef.AddContainer(jsii.String("Frontend"), &awsecs.ContainerDefinitionOptions{
		Image:          awsecs.ContainerImage_FromRegistry(jsii.String("frappe/erpnext:v15.62.0"), nil),
		MemoryLimitMiB: jsii.Number(256),
		Command:        jsii.Strings("CMD-SHELL", "ngix-entrypoint.sh"),
		HealthCheck: &awsecs.HealthCheck{
			Command: jsii.Strings("CMD-SHELL", "curl localhost:8080"),
		},
	})
	shortQueueDef := awsecs.NewEc2TaskDefinition(stack, jsii.String("ERPShortQ"), &awsecs.Ec2TaskDefinitionProps{
		NetworkMode: awsecs.NetworkMode_BRIDGE,
	})

	shortQueueDef.AddContainer(jsii.String("ShortQ"), &awsecs.ContainerDefinitionOptions{
		Image:          awsecs.ContainerImage_FromRegistry(jsii.String("frappe/erpnext:v15.62.0"), nil),
		MemoryLimitMiB: jsii.Number(256),
		Command: jsii.Strings(
			"CMD-SHELL",
			"bench",
			"worker",
			"--queue",
			"short,default",
		),
		HealthCheck: &awsecs.HealthCheck{
			Command: jsii.Strings("CMD-SHELL", "docker-compose exec backend healthcheck.sh"),
		},
	})

	longQueueDef := awsecs.NewEc2TaskDefinition(stack, jsii.String("ERPLongQ"), &awsecs.Ec2TaskDefinitionProps{
		NetworkMode: awsecs.NetworkMode_BRIDGE,
	})

	longQueueDef.AddContainer(jsii.String("LongQ"), &awsecs.ContainerDefinitionOptions{
		Image:          awsecs.ContainerImage_FromRegistry(jsii.String("frappe/erpnext:v15.62.0"), nil),
		MemoryLimitMiB: jsii.Number(256),
		Command: jsii.Strings(
			"CMD-SHELL",
			"bench",
			"worker",
			"--queue",
			"long,default,short",
		),
		HealthCheck: &awsecs.HealthCheck{
			Command: jsii.Strings("CMD-SHELL", "docker-compose exec backend healthcheck.sh"),
		},
	})

	scheduleDef := awsecs.NewEc2TaskDefinition(stack, jsii.String("ERPSchedule"), &awsecs.Ec2TaskDefinitionProps{
		NetworkMode: awsecs.NetworkMode_BRIDGE,
	})

	scheduleDef.AddContainer(jsii.String("Schedule"), &awsecs.ContainerDefinitionOptions{
		Image:          awsecs.ContainerImage_FromRegistry(jsii.String("frappe/erpnext:v15.62.0"), nil),
		MemoryLimitMiB: jsii.Number(256),
		Command: jsii.Strings(
			"CMD-SHELL",
			"bench",
			"schedule",
		),
		HealthCheck: &awsecs.HealthCheck{
			Command: jsii.Strings("CMD-SHELL", "docker-compose exec backend healthcheck.sh"),
		},
	})

	awsecs.NewEc2Service(stack, jsii.String("BackendService"), &awsecs.Ec2ServiceProps{
		Cluster:           ecsCluster,
		TaskDefinition:    backendTaskDef,
		MinHealthyPercent: jsii.Number(100),
	})

	awsecs.NewEc2Service(stack, jsii.String("Frontend_Service"), &awsecs.Ec2ServiceProps{
		Cluster:           ecsCluster,
		TaskDefinition:    frontendTaskDef,
		MinHealthyPercent: jsii.Number(100),
	})

	awsecs.NewEc2Service(stack, jsii.String("ShortQ_Service"), &awsecs.Ec2ServiceProps{
		Cluster:           ecsCluster,
		TaskDefinition:    shortQueueDef,
		MinHealthyPercent: jsii.Number(100),
	})

	awsecs.NewEc2Service(stack, jsii.String("LongQ_Service"), &awsecs.Ec2ServiceProps{
		Cluster:           ecsCluster,
		TaskDefinition:    longQueueDef,
		MinHealthyPercent: jsii.Number(100),
	})

	awsecs.NewEc2Service(stack, jsii.String("Schedule_Service"), &awsecs.Ec2ServiceProps{
		Cluster:           ecsCluster,
		TaskDefinition:    scheduleDef,
		MinHealthyPercent: jsii.Number(100),
	})

	// Elasticache Def
	// TODO Add Sec Groups
	selectedSubnets := vpc.SelectSubnets(&awsec2.SubnetSelection{
		SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
	}).Subnets

	var subnetIds []*string
	for _, subnet := range *selectedSubnets {
		subnetIds = append(subnetIds, subnet.SubnetId())
	}
	cacheSecurityGroup := awsec2.NewSecurityGroup(stack, jsii.String("CacheSG"), &awsec2.SecurityGroupProps{
		Vpc:              vpc,
		AllowAllOutbound: jsii.Bool(true),
		Description:      jsii.String("Security group for Redis"),
	})
	subnetGroup := awselasticache.NewCfnSubnetGroup(stack, jsii.String("ERPSubnetGroup"), &awselasticache.CfnSubnetGroupProps{
		Description:          jsii.String("Subnet group for ElastiCache Redis"),
		SubnetIds:            &subnetIds,
		CacheSubnetGroupName: jsii.String("erp-cache-subnet-group"),
	})

	awselasticache.NewCfnReplicationGroup(stack, jsii.String("ERPCacheCluster"), &awselasticache.CfnReplicationGroupProps{
		ReplicationGroupDescription: jsii.String("Valkey replication group for ERP cluster"),
		CacheNodeType:               jsii.String("cache.t3.nano"),
		Engine:                      jsii.String("valkey"),
		NumNodeGroups:               jsii.Number(1),
		ReplicasPerNodeGroup:        jsii.Number(1),
		ReplicationGroupId:          jsii.String("erp-cache-cluster"),
		SecurityGroupIds:            &[]*string{cacheSecurityGroup.SecurityGroupId()},
		CacheSubnetGroupName:        subnetGroup.CacheSubnetGroupName(),
		TransitEncryptionEnabled:    jsii.Bool(true),
	})

	return stack
}

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	NewErpAwsDeployStack(app, "ErpAwsDeployStack", &ErpAwsDeployStackProps{
		awscdk.StackProps{
			Env: env(),
		},
	})

	app.Synth(nil)
}

// env determines the AWS environment (account+region) in which our stack is to
// be deployed. For more information see: https://docs.aws.amazon.com/cdk/latest/guide/environments.html
func env() *awscdk.Environment {
	// If unspecified, this stack will be "environment-agnostic".
	// Account/Region-dependent features and context lookups will not work, but a
	// single synthesized template can be deployed anywhere.
	//---------------------------------------------------------------------------
	//return nil

	// Uncomment if you know exactly what account and region you want to deploy
	// the stack to. This is the recommendation for production stacks.
	//---------------------------------------------------------------------------
	// return &awscdk.Environment{
	// 	Account: jsii.String("543365275061"),
	// 	Region:  jsii.String("us-east-1"),
	// }

	// Uncomment to specialize this stack for the AWS Account and Region that are
	// implied by the current CLI configuration. This is recommended for dev
	// stacks.
	//---------------------------------------------------------------------------
	return &awscdk.Environment{
		Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
		Region:  jsii.String(os.Getenv("CDK_DEFAULT_REGION")),
	}
}
