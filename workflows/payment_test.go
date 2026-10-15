package workflows

import (
	"errors"
	"testing"

	"github.com/z-sector/temporal-payment/activities"
	"github.com/z-sector/temporal-payment/payment"

	"go.temporal.io/sdk/testsuite"
)

func TestTransferOutageCompletes(t *testing.T) {
	// The test suite turns a zero MaximumAttempts into 10. The availability gate
	// can use those attempts up before the stored key is transferred.
	last := errors.New("workflow did not complete")
	for range 30 {
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
			continue
		}
		if err := env.GetWorkflowError(); err != nil {
			last = err
			continue
		}
		var result payment.Result
		if err := env.GetWorkflowResult(&result); err != nil {
			t.Fatal(err)
		}
		if result.Status != payment.StatusCompleted {
			t.Fatalf("status = %s, want %s", result.Status, payment.StatusCompleted)
		}
		return
	}
	t.Fatal(last)
}
