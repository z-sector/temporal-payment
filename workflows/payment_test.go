package workflows

import (
	"errors"
	"testing"

	"github.com/z-sector/temporal-payment/activities"
	"github.com/z-sector/temporal-payment/payment"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
)

func TestTransferDownFailsThenResetCompletes(t *testing.T) {
	// The test suite turns a zero MaximumAttempts into 10. The availability gate
	// can use those attempts before TransferDown is returned or the reset transfers.
	a := &activities.Activities{}
	input := payment.Input{
		From:           "uma",
		To:             "carol",
		Value:          100,
		IdempotencyKey: "pay-u",
	}
	if err := runUntil(t, a, input, wantTransferDown); err != nil {
		t.Fatal(err)
	}
	if err := runUntil(t, a, input, wantCompleted); err != nil {
		t.Fatal(err)
	}
}

type workflowWant int

const (
	wantTransferDown workflowWant = iota
	wantCompleted
)

func runUntil(t *testing.T, a *activities.Activities, input payment.Input, want workflowWant) error {
	t.Helper()
	last := errors.New("workflow did not reach the expected result")
	for range 30 {
		var suite testsuite.WorkflowTestSuite
		env := suite.NewTestWorkflowEnvironment()
		env.RegisterWorkflow(PaymentWorkflow)
		env.RegisterActivity(a)
		env.ExecuteWorkflow(PaymentWorkflow, input)
		if !env.IsWorkflowCompleted() {
			continue
		}
		err := env.GetWorkflowError()
		switch want {
		case wantTransferDown:
			if err == nil {
				t.Fatal("transfer completed before TransferDown")
			}
			var appErr *temporal.ApplicationError
			if errors.As(err, &appErr) && appErr.Type() == payment.ErrTransferDown {
				return nil
			}
			last = err
		case wantCompleted:
			if err != nil {
				var appErr *temporal.ApplicationError
				if errors.As(err, &appErr) && appErr.Type() == payment.ErrTransferDown {
					t.Fatal("reset returned TransferDown for a stored key")
				}
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
			return nil
		}
	}
	return last
}
