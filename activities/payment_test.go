package activities

import (
	"errors"
	"testing"

	"github.com/z-sector/temporal-payment/payment"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
)

func TestTransferOutageFailsUntilAttemptsThenSucceeds(t *testing.T) {
	acts := &Activities{}
	env := (&testsuite.WorkflowTestSuite{}).NewTestActivityEnvironment()
	env.RegisterActivity(acts.Transfer)

	input := payment.StepInput{
		From:           "uma",
		To:             "carol",
		Value:          100,
		IdempotencyKey: "pay-transfer-down-1:transfer",
	}
	for attempt := 1; attempt <= int(payment.TransferOutageAttempts); attempt++ {
		_, err := env.ExecuteActivity(acts.Transfer, input)
		var appErr *temporal.ApplicationError
		if !errors.As(err, &appErr) {
			t.Fatalf("attempt %d error = %v", attempt, err)
		}
		if appErr.Type() != payment.ErrUnavailable {
			t.Fatalf("attempt %d type = %s", attempt, appErr.Type())
		}
		if appErr.NonRetryable() {
			t.Fatalf("attempt %d is non-retryable", attempt)
		}
	}

	if _, err := env.ExecuteActivity(acts.Transfer, input); err != nil {
		t.Fatalf("transfer after outage: %v", err)
	}
	if _, err := env.ExecuteActivity(acts.Transfer, input); err != nil {
		t.Fatalf("second transfer after outage: %v", err)
	}

	other := input
	other.IdempotencyKey = "pay-transfer-down-2:transfer"
	_, err := env.ExecuteActivity(acts.Transfer, other)
	var appErr *temporal.ApplicationError
	if !errors.As(err, &appErr) || appErr.Type() != payment.ErrUnavailable {
		t.Fatalf("other key error = %v", err)
	}
}
