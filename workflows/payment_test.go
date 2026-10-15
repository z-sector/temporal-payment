package workflows

import (
	"testing"

	"github.com/z-sector/temporal-payment/activities"
	"github.com/z-sector/temporal-payment/payment"

	"go.temporal.io/sdk/testsuite"
)

func TestTransferOutageCompletes(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(PaymentWorkflow)
	env.RegisterActivity(&activities.Activities{})

	env.ExecuteWorkflow(PaymentWorkflow, payment.Input{
		From:           "uma",
		To:             "carol",
		Value:          100,
		IdempotencyKey: "pay-u",
	})
	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var result payment.Result
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result.Status != payment.StatusCompleted {
		t.Fatalf("status = %s, want %s", result.Status, payment.StatusCompleted)
	}
}
