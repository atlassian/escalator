# AWS

Escalator is able to scale auto scaling groups (ASG) in AWS. These must be specified in the `nodegroups_config.yaml` passed
to the `--nodegroups=` flag. All of the auto scaling groups that are specified must reside in the same AWS region.

## How to enable

Start Escalator with the `--cloud-provider=aws` flag.

## Permissions

Escalator requires the following IAM policy to be able to properly integrate with AWS:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "autoscaling:AttachInstances",
        "autoscaling:DescribeAutoScalingGroups",
        "autoscaling:SetDesiredCapacity",
        "autoscaling:TerminateInstanceInAutoScalingGroup",
        "ec2:CreateFleet",
        "ec2:DescribeInstanceStatus",
        "ec2:DescribeInstances"
      ],
      "Resource": "*"
    }
  ]
}
```

## AWS Credentials

Escalator makes use of [aws-sdk-go](https://github.com/aws/aws-sdk-go) for communicating with the AWS API to perform
scaling of auto scaling groups and terminating of instances.

To initiate the session, Escalator uses the [session](https://docs.aws.amazon.com/sdk-for-go/api/aws/session/) package:

```go
sess, err := session.NewSession()
```

This will use the [default credential chain](https://docs.aws.amazon.com/sdk-for-go/api/aws/defaults/#CredChain)
for obtaining access.

See [Configuring the AWS SDK for Go](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html)
for more information on how the SDK obtains access.

It is highly recommended to use IAM roles for Escalator access, using the above IAM policy.

### STS Assume Role

Escalator supports assuming a role when it starts. This is configured using the `--aws-assume-role-arn` flag when
running Escalator. You can read more about it in 
[assume-role](https://docs.aws.amazon.com/cli/latest/reference/sts/assume-role.html) and
[Assuming a Role](https://docs.aws.amazon.com/cli/latest/userguide/cli-roles.html).

## Auto Scaling Group Configuration

The targeted auto scaling groups in AWS don't have to be configured in a specific way, but we recommend the following:

- Change the [Default Cooldown](https://docs.aws.amazon.com/autoscaling/ec2/userguide/Cooldown.html) for the auto
scaling group to be a lower value than the `scale_up_cool_down_period` option. This prevents situations where
Escalator needs to scale up but has to wait on AWS to finish the scaling activity. This allows for faster and 
more responsive scaling. For a `scale_up_cool_down_period` period of `2m` (2 minutes), we recommend setting the default 
cooldown to `60` (60 seconds).
- Have the same min/max nodes in the auto scaling group as `min_nodes` and `max_nodes` in the `nodegroups_config.yaml`
file.
- Have "scale in protection" enabled on all instances in the ASG to prevent cases where instances are terminated by
AWS but may still have workloads running.

## Deployment

To create a deployment of Escalator that uses AWS as the cloud provider with an IAM role, run the following:

```bash
kubectl create -f escalator-deployment-aws.yaml
```

**Note:** this example uses [kube2iam](https://github.com/jtblin/kube2iam) for obtaining an IAM role inside a pod. See
the `iam.amazonaws.com/role` annotation.

**Note:** the `AWS_REGION` environment variable.

## Using Launch Templates

Escalator works out of the box with either [Launch-Configurations](https://docs.aws.amazon.com/autoscaling/ec2/userguide/LaunchConfiguration.html) or [Launch-Templates](https://docs.aws.amazon.com/autoscaling/ec2/userguide/LaunchTemplates.html). When using Launch-Templates, Escalator supports using multiple instance types in the one Auto Scaling Group. However, if the instance types have significantly different sizing (CPU / Memory), Escalator may take more than one Scaling operation to reach the desired capacity depending on the type of existing nodes in the cluster and the size of the new instance provided. We recommend having smaller sized instances as the first priority in your Auto Scaling Group, and the larger type second. This will increase the chance that Escalator will over provision instead of under provision the first time.

## Common issues, caveats and gotchas

- Ensure that if you are using the remote credential provider that `AWS_REGION` or `AWS_DEFAULT_REGION` is set to the 
region that the auto scaling groups reside in.
- Escalator does not perform any "auto discovery" of auto scaling groups within AWS. These need to be manually defined
in the `nodegroups_config.yaml` config passed to the `--nodegroups=` flag.
- Escalator does not support scaling auto scaling groups that are located in different regions, they must all reside
in the same region. Escalator will terminate itself if it isn't able to describe an auto scaling group in a different 
region.
- Ensure "scale in protection" is enabled for all ASGs.
- Do not use 
 [Auto Scaling Lifecycle Hooks](https://docs.aws.amazon.com/autoscaling/ec2/userguide/lifecycle-hooks.html) for
 terminating of instances as Escalator will handle the termination of instances itself. 
- If using launch templates do not use the "network settings" area to configure the security groups. The security groups
 should be configured via a network interface.