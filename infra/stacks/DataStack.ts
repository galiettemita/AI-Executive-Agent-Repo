import { StackContext, use } from "sst/constructs";
import * as ec2 from "aws-cdk-lib/aws-ec2";
import * as rds from "aws-cdk-lib/aws-rds";
import * as elasticache from "aws-cdk-lib/aws-elasticache";
import * as s3 from "aws-cdk-lib/aws-s3";
import { RemovalPolicy, Duration } from "aws-cdk-lib";
import { NetworkStack } from "./NetworkStack";

export function DataStack({ stack }: StackContext) {
  const { vpc, appSecurityGroup } = use(NetworkStack);

  const dbSecurityGroup = new ec2.SecurityGroup(stack, "DbSecurityGroup", {
    vpc,
    description: "Security group for RDS",
  });
  dbSecurityGroup.addIngressRule(
    appSecurityGroup,
    ec2.Port.tcp(5432),
    "Allow ECS services to reach Postgres",
  );

  const redisSecurityGroup = new ec2.SecurityGroup(stack, "RedisSecurityGroup", {
    vpc,
    description: "Security group for Redis",
  });
  redisSecurityGroup.addIngressRule(
    appSecurityGroup,
    ec2.Port.tcp(6379),
    "Allow ECS services to reach Redis",
  );

  const database = new rds.DatabaseInstance(stack, "Postgres", {
    engine: rds.DatabaseInstanceEngine.postgres({
      version: rds.PostgresEngineVersion.VER_16_3,
    }),
    vpc,
    vpcSubnets: { subnetType: ec2.SubnetType.PRIVATE_WITH_EGRESS },
    credentials: rds.Credentials.fromGeneratedSecret("executive"),
    instanceType: ec2.InstanceType.of(
      ec2.InstanceClass.T4G,
      ec2.InstanceSize.MEDIUM,
    ),
    allocatedStorage: 100,
    maxAllocatedStorage: 200,
    multiAz: true,
    storageEncrypted: true,
    backupRetention: Duration.days(7),
    deletionProtection: true,
    securityGroups: [dbSecurityGroup],
  });

  const redisSubnetGroup = new elasticache.CfnSubnetGroup(stack, "RedisSubnetGroup", {
    description: "Private subnets for Redis",
    subnetIds: vpc.privateSubnets.map((s) => s.subnetId),
  });

  const redis = new elasticache.CfnReplicationGroup(stack, "Redis", {
    replicationGroupDescription: "Executive OS Redis",
    engine: "redis",
    engineVersion: "7.0",
    cacheNodeType: "cache.t4g.small",
    numNodeGroups: 1,
    replicasPerNodeGroup: 1,
    automaticFailoverEnabled: true,
    cacheSubnetGroupName: redisSubnetGroup.ref,
    transitEncryptionEnabled: true,
    atRestEncryptionEnabled: true,
    securityGroupIds: [redisSecurityGroup.securityGroupId],
  });

  const attachments = new s3.Bucket(stack, "AttachmentsBucket", {
    encryption: s3.BucketEncryption.S3_MANAGED,
    versioned: true,
    blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
    removalPolicy: RemovalPolicy.RETAIN,
  });

  stack.addOutputs({
    RdsEndpoint: database.instanceEndpoint.hostname,
    RdsSecretArn: database.secret?.secretArn || "",
    RedisEndpoint: redis.attrPrimaryEndPointAddress,
    AttachmentsBucket: attachments.bucketName,
  });

  return {
    database,
    redis,
    attachments,
    dbSecurityGroup,
    redisSecurityGroup,
  };
}
