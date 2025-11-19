
// First, we define all of our inputs
source "aws_cloudtrail" "account-x" { }

// We now want to evaluate for any IAM user action
// on the AWS cloudtrail sink
evaluation "aws_cloudtrail" "any_action" {
  inputs = [source.aws_cloudtrail.account-x]

  condition {
    path = "$.userIdentity.type"
    value = "Root"
  }

  outputs = [output.console.log_cloudtrail_user_actions]
}

// Specify alert destinations for emitting hits to
output "console" "log_cloudtrail_user_actions" { }
