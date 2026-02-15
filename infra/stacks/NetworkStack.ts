import { StackContext } from "sst/constructs";
import * as ec2 from "aws-cdk-lib/aws-ec2";
import * as elbv2 from "aws-cdk-lib/aws-elasticloadbalancingv2";

export function NetworkStack({ stack }: StackContext) {
  const vpc = new ec2.Vpc(stack, "ExecutiveVpc", {
    maxAzs: 2,
    natGateways: 1,
    subnetConfiguration: [
      {
        name: "public",
        subnetType: ec2.SubnetType.PUBLIC,
        cidrMask: 24,
      },
      {
        name: "private",
        subnetType: ec2.SubnetType.PRIVATE_WITH_EGRESS,
        cidrMask: 24,
      },
    ],
  });

  const albSecurityGroup = new ec2.SecurityGroup(stack, "AlbSecurityGroup", {
    vpc,
    description: "Security group for ALB",
  });
  albSecurityGroup.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(80), "Allow HTTP");
  albSecurityGroup.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(443), "Allow HTTPS");
  albSecurityGroup.addIngressRule(ec2.Peer.anyIpv6(), ec2.Port.tcp(443), "Allow HTTPS IPv6");

  const appSecurityGroup = new ec2.SecurityGroup(stack, "AppSecurityGroup", {
    vpc,
    description: "Security group for ECS services",
  });

  appSecurityGroup.addIngressRule(
    albSecurityGroup,
    ec2.Port.tcp(8000),
    "Allow ALB to reach services",
  );

  const alb = new elbv2.ApplicationLoadBalancer(stack, "Alb", {
    vpc,
    internetFacing: true,
    securityGroup: albSecurityGroup,
  });

  stack.addOutputs({
    VpcId: vpc.vpcId,
    AlbSecurityGroupId: albSecurityGroup.securityGroupId,
    AppSecurityGroupId: appSecurityGroup.securityGroupId,
    AlbDnsName: alb.loadBalancerDnsName,
  });

  return { vpc, alb, albSecurityGroup, appSecurityGroup };
}
