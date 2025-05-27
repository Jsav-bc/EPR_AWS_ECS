package main

import (
	"os"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsautoscaling"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awselasticache"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
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

	instanceRole := awsiam.NewRole(stack, jsii.String("ECSInstanceRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("ec2.amazonaws.com"), nil),
		ManagedPolicies: &[]awsiam.IManagedPolicy{
			awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AmazonEC2ContainerServiceforEC2Role")),
		},
	})

	userData := awsec2.UserData_ForLinux(&awsec2.LinuxUserDataOptions{})
	userData.AddCommands(
		jsii.String("#!/bin/bash"),
		jsii.String("echo ECS_CLUSTER=ERP_Cluster >> /etc/ecs/ecs.config"),
		jsii.String("mkfs -t xfs /dev/xvdb"),
		jsii.String("mkdir -p /mnt/sites"),
		jsii.String("mount /dev/xvdb /mnt/sites"),
		jsii.String("echo '/dev/xvdb /mnt/sites xfs defaults,nofail 0 2' >> /etc/fstab"),
	)

	launchTemplate := awsec2.NewLaunchTemplate(stack, jsii.String("ERPLaunchTemplate"), &awsec2.LaunchTemplateProps{
		InstanceType: awsec2.NewInstanceType(jsii.String("t3.micro")),
		MachineImage: awsecs.EcsOptimizedImage_AmazonLinux2(awsecs.AmiHardwareType_STANDARD, &awsecs.EcsOptimizedImageOptions{}),
		BlockDevices: &[]*awsec2.BlockDevice{
			{
				DeviceName: jsii.String("/dev/xvdb"),
				Volume: awsec2.BlockDeviceVolume_Ebs(jsii.Number(20), &awsec2.EbsDeviceOptions{
					VolumeType: awsec2.EbsDeviceVolumeType_GP3,
				}),
			},
		},
		UserData: userData,
		Role:     instanceRole,
	})

	asg := awsautoscaling.NewAutoScalingGroup(stack, jsii.String("ERPASG"), &awsautoscaling.AutoScalingGroupProps{
		Vpc:             vpc,
		MaxCapacity:     jsii.Number(4),
		MinCapacity:     jsii.Number(1),
		DesiredCapacity: jsii.Number(1),
		LaunchTemplate:  launchTemplate,
		VpcSubnets:      &awsec2.SubnetSelection{SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS},
	})

	ecsCluster := awsecs.NewCluster(stack, jsii.String("ERP_Cluster"), &awsecs.ClusterProps{
		Vpc: vpc,
	})

	capp := awsecs.NewAsgCapacityProvider(stack, jsii.String("Capacity"), &awsecs.AsgCapacityProviderProps{
		AutoScalingGroup: asg,
	})

	ecsCluster.AddAsgCapacityProvider(capp, &awsecs.AddAutoScalingGroupCapacityOptions{})
	// Task role?

	// Shared EBS Volume

	// Task Definition for Backend

	backendTaskDef := awsecs.NewEc2TaskDefinition(stack, jsii.String("ERPBackendTask"), &awsecs.Ec2TaskDefinitionProps{
		NetworkMode: awsecs.NetworkMode_AWS_VPC,
	})

	backendTaskDef.AddVolume(&awsecs.Volume{
		Name: jsii.String("SitesVolume"),
		Host: &awsecs.Host{
			SourcePath: jsii.String("/mnt/sites"),
		},
	})

	backendContainer := backendTaskDef.AddContainer(jsii.String("Backend"), &awsecs.ContainerDefinitionOptions{
		Image:          awsecs.ContainerImage_FromRegistry(jsii.String("frappe/erpnext:v15.62.0"), nil),
		MemoryLimitMiB: jsii.Number(256),
		HealthCheck: &awsecs.HealthCheck{
			Command:     jsii.Strings("CMD-SHELL", "sh healthcheck.sh"),
			StartPeriod: awscdk.Duration_Seconds(jsii.Number(120)),
		},
	})

	backendContainer.AddMountPoints(&awsecs.MountPoint{
		ContainerPath: jsii.String("/home/frappe/frappe-bench/sites"),
		SourceVolume:  jsii.String("SitesVolume"),
		ReadOnly:      jsii.Bool(false),
	})

	frontendTaskDef := awsecs.NewEc2TaskDefinition(stack, jsii.String("ERPFrontendTask"), &awsecs.Ec2TaskDefinitionProps{
		NetworkMode: awsecs.NetworkMode_AWS_VPC,
	})

	frontendTaskDef.AddContainer(jsii.String("Frontend"), &awsecs.ContainerDefinitionOptions{
		Image:          awsecs.ContainerImage_FromRegistry(jsii.String("frappe/erpnext:v15.62.0"), nil),
		MemoryLimitMiB: jsii.Number(256),
		Command:        jsii.Strings("CMD-SHELL", "sh ngix-entrypoint.sh"),
		HealthCheck: &awsecs.HealthCheck{
			Command:     jsii.Strings("curl localhost:8080"),
			StartPeriod: awscdk.Duration_Seconds(jsii.Number(120)),
		},
	})

	frontendTaskDef.AddVolume(&awsecs.Volume{
		Name: jsii.String("SitesVolume"),
		Host: &awsecs.Host{
			SourcePath: jsii.String("/mnt/sites"),
		},
	})
	shortQueueDef := awsecs.NewEc2TaskDefinition(stack, jsii.String("ERPShortQ"), &awsecs.Ec2TaskDefinitionProps{
		NetworkMode: awsecs.NetworkMode_AWS_VPC,
	})

	shortQueueDef.AddContainer(jsii.String("ShortQ"), &awsecs.ContainerDefinitionOptions{
		Image:          awsecs.ContainerImage_FromRegistry(jsii.String("frappe/erpnext:v15.62.0"), nil),
		MemoryLimitMiB: jsii.Number(256),
		Command: jsii.Strings(
			"bench",
			"worker",
			"--queue",
			"short,default",
		),
	})

	shortQueueDef.AddVolume(&awsecs.Volume{
		Name: jsii.String("SitesVolume"),
		Host: &awsecs.Host{
			SourcePath: jsii.String("/mnt/sites"),
		},
	})

	longQueueDef := awsecs.NewEc2TaskDefinition(stack, jsii.String("ERPLongQ"), &awsecs.Ec2TaskDefinitionProps{
		NetworkMode: awsecs.NetworkMode_AWS_VPC,
	})

	longQueueDef.AddContainer(jsii.String("LongQ"), &awsecs.ContainerDefinitionOptions{
		Image:          awsecs.ContainerImage_FromRegistry(jsii.String("frappe/erpnext:v15.62.0"), nil),
		MemoryLimitMiB: jsii.Number(256),
		Command: jsii.Strings(
			"bench",
			"worker",
			"--queue",
			"long,default,short",
		),
	})

	longQueueDef.AddVolume(&awsecs.Volume{
		Name: jsii.String("SitesVolume"),
		Host: &awsecs.Host{
			SourcePath: jsii.String("/mnt/sites"),
		},
	})

	scheduleDef := awsecs.NewEc2TaskDefinition(stack, jsii.String("ERPSchedule"), &awsecs.Ec2TaskDefinitionProps{
		NetworkMode: awsecs.NetworkMode_AWS_VPC,
	})

	scheduleDef.AddContainer(jsii.String("Schedule"), &awsecs.ContainerDefinitionOptions{
		Image:          awsecs.ContainerImage_FromRegistry(jsii.String("frappe/erpnext:v15.62.0"), nil),
		MemoryLimitMiB: jsii.Number(256),
		Command: jsii.Strings(
			"bench",
			"schedule",
		),
	})

	scheduleDef.AddVolume(&awsecs.Volume{
		Name: jsii.String("SitesVolume"),
		Host: &awsecs.Host{
			SourcePath: jsii.String("/mnt/sites"),
		},
	})

	dbDef := awsecs.NewEc2TaskDefinition(stack, jsii.String("database"), &awsecs.Ec2TaskDefinitionProps{
		NetworkMode: awsecs.NetworkMode_AWS_VPC,
	})

	dbDef.AddVolume(&awsecs.Volume{
		Name: jsii.String("DbDataVolume"),
		Host: &awsecs.Host{
			SourcePath: jsii.String("/mnt/dbdata"),
		},
	})

	dbContainer := dbDef.AddContainer(jsii.String("db"), &awsecs.ContainerDefinitionOptions{
		Image:          awsecs.ContainerImage_FromRegistry(jsii.String("frappe/erpnext:v15.62.0"), nil),
		MemoryLimitMiB: jsii.Number(512),
		Command: jsii.Strings(
			"--character-set-server=utf8mb4",
			"--collation-server=utf8mb4_unicode_ci",
			"--skip-character-set-client-handshake",
			"--skip-innodb-read-only-compressed",
		),
		Environment: &map[string]*string{
			"MYSQL_ROOT_PASSWORD":   jsii.String("admin"),
			"MARIADB_ROOT_PASSWORD": jsii.String("admin"),
		},
	})

	dbContainer.AddMountPoints(&awsecs.MountPoint{
		SourceVolume:  jsii.String("DbDataVolume"),
		ContainerPath: jsii.String("/var/lib/mysql"),
		ReadOnly:      jsii.Bool(false),
	})

	backendSg := awsec2.NewSecurityGroup(stack, jsii.String("BackendSG"), &awsec2.SecurityGroupProps{
		Vpc:              vpc,
		AllowAllOutbound: jsii.Bool(true),
	})
	backendService := awsecs.NewEc2Service(stack, jsii.String("Backend_Service"), &awsecs.Ec2ServiceProps{
		Cluster:           ecsCluster,
		TaskDefinition:    backendTaskDef,
		MinHealthyPercent: jsii.Number(100),
		SecurityGroups:    &[]awsec2.ISecurityGroup{backendSg},
		CapacityProviderStrategies: &[]*awsecs.CapacityProviderStrategy{
			{
				CapacityProvider: capp.CapacityProviderName(),
				Weight:           jsii.Number(1),
			},
		},
	})

	frontendSg := awsec2.NewSecurityGroup(stack, jsii.String("frontendSG"), &awsec2.SecurityGroupProps{
		Vpc:              vpc,
		AllowAllOutbound: jsii.Bool(true),
	})

	frontendService := awsecs.NewEc2Service(stack, jsii.String("Frontend_Service"), &awsecs.Ec2ServiceProps{
		Cluster:           ecsCluster,
		TaskDefinition:    frontendTaskDef,
		MinHealthyPercent: jsii.Number(100),
		SecurityGroups:    &[]awsec2.ISecurityGroup{frontendSg},
		CapacityProviderStrategies: &[]*awsecs.CapacityProviderStrategy{
			{
				CapacityProvider: capp.CapacityProviderName(),
				Weight:           jsii.Number(1),
			},
		},
	})
	shortqSg := awsec2.NewSecurityGroup(stack, jsii.String("shortqSG"), &awsec2.SecurityGroupProps{
		Vpc:              vpc,
		AllowAllOutbound: jsii.Bool(true),
	})
	shortQService := awsecs.NewEc2Service(stack, jsii.String("ShortQ_Service"), &awsecs.Ec2ServiceProps{
		Cluster:           ecsCluster,
		TaskDefinition:    shortQueueDef,
		MinHealthyPercent: jsii.Number(100),
		SecurityGroups:    &[]awsec2.ISecurityGroup{shortqSg},
		CapacityProviderStrategies: &[]*awsecs.CapacityProviderStrategy{
			{
				CapacityProvider: capp.CapacityProviderName(),
				Weight:           jsii.Number(1),
			},
		},
	})

	longqSg := awsec2.NewSecurityGroup(stack, jsii.String("longqdSG"), &awsec2.SecurityGroupProps{
		Vpc:              vpc,
		AllowAllOutbound: jsii.Bool(true),
	})

	longQService := awsecs.NewEc2Service(stack, jsii.String("LongQ_Service"), &awsecs.Ec2ServiceProps{
		Cluster:           ecsCluster,
		TaskDefinition:    longQueueDef,
		MinHealthyPercent: jsii.Number(100),
		SecurityGroups:    &[]awsec2.ISecurityGroup{longqSg},
		CapacityProviderStrategies: &[]*awsecs.CapacityProviderStrategy{
			{
				CapacityProvider: capp.CapacityProviderName(),
				Weight:           jsii.Number(1),
			},
		},
	})
	scheduleSg := awsec2.NewSecurityGroup(stack, jsii.String("scheduleSG"), &awsec2.SecurityGroupProps{
		Vpc:              vpc,
		AllowAllOutbound: jsii.Bool(true),
	})
	scheduleService := awsecs.NewEc2Service(stack, jsii.String("Schedule_Service"), &awsecs.Ec2ServiceProps{
		Cluster:           ecsCluster,
		TaskDefinition:    scheduleDef,
		MinHealthyPercent: jsii.Number(100),
		SecurityGroups:    &[]awsec2.ISecurityGroup{scheduleSg},
		CapacityProviderStrategies: &[]*awsecs.CapacityProviderStrategy{
			{
				CapacityProvider: capp.CapacityProviderName(),
				Weight:           jsii.Number(1),
			},
		},
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

	cache := awselasticache.NewCfnReplicationGroup(stack, jsii.String("ERPCacheCluster"), &awselasticache.CfnReplicationGroupProps{
		ReplicationGroupDescription: jsii.String("Valkey replication group for ERP cluster"),
		CacheNodeType:               jsii.String("cache.t3.micro"),
		Engine:                      jsii.String("valkey"),
		NumNodeGroups:               jsii.Number(1),
		ReplicasPerNodeGroup:        jsii.Number(1),
		ReplicationGroupId:          jsii.String("erp-cache-cluster"),
		SecurityGroupIds:            &[]*string{cacheSecurityGroup.SecurityGroupId()},
		CacheSubnetGroupName:        subnetGroup.CacheSubnetGroupName(),
		TransitEncryptionEnabled:    jsii.Bool(true),
	})

	configTaskDef := awsecs.NewEc2TaskDefinition(stack, jsii.String("ConfiguratorTask"), &awsecs.Ec2TaskDefinitionProps{
		NetworkMode: awsecs.NetworkMode_AWS_VPC,
	})

	configTaskDef.Node().AddDependency(cache)

	valkeyEndpoint := awscdk.Fn_Sub(jsii.String("valkey://${Host}:${Port}"), &map[string]*string{
		"Host": cache.AttrConfigurationEndPointAddress(),
		"Port": cache.AttrPrimaryEndPointPort(),
	})

	configTaskDef.AddContainer(jsii.String("Configurator"), &awsecs.ContainerDefinitionOptions{
		Image: awsecs.ContainerImage_FromRegistry(jsii.String("frappe/erpnext:v15.63.0"), &awsecs.RepositoryImageProps{}),
		Command: jsii.Strings(
			"bash", "-c",
			`ls -1 apps > sites/apps.txt;
			bench set-config -g db_host $DB_HOST;
			bench set-config -gp db_port $DB_PORT;
			bench set-config -g redis_cache valkey://$REDIS_CACHE;
			bench set-config -g redis_queue valkey://$REDIS_QUEUE;
			bench set-config -g redis_socketio valkey://$REDIS_QUEUE;
			bench set-config -gp socketio_port $SOCKETIO_PORT;`,
		),
		Environment: &map[string]*string{
			"DB_HOST":       jsii.String("db"),
			"DB_PORT":       jsii.String("3306"),
			"REDIS_CACHE":   valkeyEndpoint,
			"REDIS_QUEUE":   valkeyEndpoint,
			"SOCKETIO_PORT": jsii.String("9000"),
		},
		MemoryLimitMiB: jsii.Number(256),
	})

	createSiteTaskDef := awsecs.NewEc2TaskDefinition(stack, jsii.String("CreateSiteTask"), &awsecs.Ec2TaskDefinitionProps{
		NetworkMode: awsecs.NetworkMode_AWS_VPC,
	})

	cmd := `
	wait-for-it -t 120 db:3306;
	wait-for-it -t 120 $REDIS_ENDPOINT;

	export start=$(date +%s);

	until [[ -n $(grep -hs ^ sites/common_site_config.json | jq -r ".db_host // empty") ]] &&
	      [[ -n $(grep -hs ^ sites/common_site_config.json | jq -r ".redis_cache // empty") ]] &&
	      [[ -n $(grep -hs ^ sites/common_site_config.json | jq -r ".redis_queue // empty") ]]; do
		echo "Waiting for sites/common_site_config.json to be created";
		sleep 5;
		if (( $(date +%s) - start > 120 )); then
			echo "could not find sites/common_site_config.json with required keys";
			exit 1
		fi
	done;

	echo "sites/common_site_config.json found";

	bench new-site --mariadb-user-host-login-scope='%' \
	  --admin-password=admin \
	  --db-root-username=root \
	  --db-root-password=admin \
	  --install-app erpnext \
	  --set-default frontend;
	`

	createSiteTaskDef.AddContainer(jsii.String("CreateSite"), &awsecs.ContainerDefinitionOptions{
		Image:          awsecs.AssetImage_FromRegistry(jsii.String("frappe/erpnext:v15.63.0"), &awsecs.RepositoryImageProps{}),
		MemoryLimitMiB: jsii.Number(256),
		Command: jsii.Strings(
			"bash", "-c", cmd),
		Environment: &map[string]*string{
			"REDIS_ENDPOINT": valkeyEndpoint,
			"DB_HOST":        jsii.String("db"),
			"DB_PORT":        jsii.String("3306"),
			"REDIS_CACHE":    valkeyEndpoint,
			"REDIS_QUEUE":    valkeyEndpoint,
			"SOCKETIO_PORT":  jsii.String("9000"),
		},
	})

	createSiteTaskDef.Node().AddDependency(cache)

	// connection loop
	services := []awsecs.Ec2Service{
		backendService,
		frontendService,
		shortQService,
		longQService,
		scheduleService,
	}

	for _, svc := range services {
		sgs := svc.Connections().SecurityGroups()
		if len(*sgs) == 0 {
			continue
		}
		cacheSecurityGroup.AddIngressRule(
			(*svc.Connections().SecurityGroups())[0],
			awsec2.Port_Tcp(jsii.Number(6379)),
			jsii.String("Allow Redis access from ECS Service"),
			jsii.Bool(false),
		)
	}
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
