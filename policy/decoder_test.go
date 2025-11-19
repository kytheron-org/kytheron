package policy

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDecode(t *testing.T) {
	policyHcl := `
source "aws_cloudtrail" "account-x" {}

evaluation "aws_cloudtrail" "any_action" {
  inputs = [source.aws_cloudtrail.account-x]

  condition {
    path = "$.userIdentity.type"
    value = "IAMUser"
  }

  outputs = [output.console.log_cloudtrail_user_actions]
}

output "console" "log_cloudtrail_user_actions" {}
`
	policy, err := Decode("test_policy.hcl", []byte(policyHcl))
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	assert.Equal(t, 1, len(policy.Sources))
	assert.Equal(t, "aws_cloudtrail", policy.Sources[0].Type)
	assert.Equal(t, "account-x", policy.Sources[0].Name)

	assert.Equal(t, 1, len(policy.Outputs))
	assert.Equal(t, "console", policy.Outputs[0].Type)
	assert.Equal(t, "log_cloudtrail_user_actions", policy.Outputs[0].Name)

	assert.Equal(t, 1, len(policy.Evaluations))
	assert.Equal(t, 1, len(policy.Evaluations[0].Inputs))
	assert.Equal(t, 1, len(policy.Evaluations[0].Outputs))
	assert.Equal(t, 1, len(policy.Evaluations[0].Conditions))
	assert.Equal(t, "$.userIdentity.type", policy.Evaluations[0].Conditions[0].Path)
	assert.Equal(t, "IAMUser", policy.Evaluations[0].Conditions[0].Value)
}
