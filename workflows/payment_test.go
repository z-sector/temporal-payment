package workflows

import (
	"context"
	"errors"
	"testing"

	"github.com/z-sector/temporal-payment/activities"
	"github.com/z-sector/temporal-payment/payment"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
)

func TestTransferOutageFailsThenSameWorkerCompletes(t *testing.T) {
	acts := &activities.Activities{}
	input := payment.Input{
		From:           "uma",
		To:             "carol",
		Value:          100,
		IdempotencyKey: "pay-transfer-down-1",
	}

	first, transferCalls := newPaymentEnv(acts)
	first.ExecuteWorkflow(PaymentWorkflow, input)
	if !first.IsWorkflowCompleted() {
		t.Fatal("workflow did not finish")
	}
	err := first.GetWorkflowError()
	var appErr *temporal.ApplicationError
	if !errors.As(err, &appErr) || appErr.Type() != payment.ErrUnavailable {
		t.Fatalf("workflow error = %v", err)
	}
	if appErr.NonRetryable() {
		t.Fatal("unavailable should stay retryable")
	}
	if *transferCalls != int(payment.TransferOutageAttempts) {
		t.Fatalf("transfer calls = %d, want %d", *transferCalls, payment.TransferOutageAttempts)
	}

	second, _ := newPaymentEnv(acts)
	second.ExecuteWorkflow(PaymentWorkflow, input)
	if err := second.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var result payment.Result
	if err := second.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result.Status != payment.StatusCompleted {
		t.Fatalf("status = %s", result.Status)
	}
}

func TestNormalTransferRetriesPastOutageLimit(t *testing.T) {
	var calls int
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()
	registerPassThrough(env, payment.HoldActivity)
	registerPassThrough(env, payment.CheckAntifraudActivity)
	env.RegisterActivityWithOptions(
		func(context.Context, payment.StepInput) error {
			calls++
			if calls <= int(payment.TransferOutageAttempts) {
				return temporal.NewApplicationError("service unavailable", payment.ErrUnavailable)
			}
			return nil
		},
		activity.RegisterOptions{Name: payment.TransferActivity},
	)

	env.ExecuteWorkflow(PaymentWorkflow, payment.Input{
		From:           "bob",
		To:             "carol",
		Value:          100,
		IdempotencyKey: "pay-ok-1",
	})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var result payment.Result
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result.Status != payment.StatusCompleted {
		t.Fatalf("status = %s", result.Status)
	}
	if calls != int(payment.TransferOutageAttempts)+1 {
		t.Fatalf("transfer calls = %d, want %d", calls, int(payment.TransferOutageAttempts)+1)
	}
}

func TestInsufficientFundsCompletesWithoutTransfer(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()
	var transfers int
	env.RegisterActivityWithOptions(
		func(context.Context, payment.StepInput) error {
			return temporal.NewNonRetryableApplicationError(
				"insufficient funds",
				payment.ErrInsufficientFunds,
				nil,
			)
		},
		activity.RegisterOptions{Name: payment.HoldActivity},
	)
	env.RegisterActivityWithOptions(
		func(context.Context, payment.StepInput) error {
			transfers++
			return nil
		},
		activity.RegisterOptions{Name: payment.TransferActivity},
	)

	env.ExecuteWorkflow(PaymentWorkflow, payment.Input{
		From:           "frank",
		To:             "carol",
		Value:          50,
		IdempotencyKey: "pay-nofunds-1",
	})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var result payment.Result
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result.Status != payment.StatusInsufficientFunds {
		t.Fatalf("status = %s", result.Status)
	}
	if transfers != 0 {
		t.Fatalf("transfer calls = %d", transfers)
	}
}

func TestAntifraudRejectedRefunds(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()
	registerPassThrough(env, payment.HoldActivity)
	var refunds int
	env.RegisterActivityWithOptions(
		func(context.Context, payment.StepInput) error {
			return temporal.NewNonRetryableApplicationError(
				"antifraud check rejected",
				payment.ErrAntifraudRejected,
				nil,
			)
		},
		activity.RegisterOptions{Name: payment.CheckAntifraudActivity},
	)
	env.RegisterActivityWithOptions(
		func(context.Context, payment.StepInput) error {
			refunds++
			return nil
		},
		activity.RegisterOptions{Name: payment.RefundActivity},
	)

	env.ExecuteWorkflow(PaymentWorkflow, payment.Input{
		From:           "alice",
		To:             "carol",
		Value:          75,
		IdempotencyKey: "pay-fraud-from-1",
	})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var result payment.Result
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result.Status != payment.StatusAntifraudRejected {
		t.Fatalf("status = %s", result.Status)
	}
	if refunds != 1 {
		t.Fatalf("refund calls = %d", refunds)
	}
}

func newPaymentEnv(acts *activities.Activities) (*testsuite.TestWorkflowEnvironment, *int) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()
	registerPassThrough(env, payment.HoldActivity)
	registerPassThrough(env, payment.CheckAntifraudActivity)
	env.RegisterActivity(acts.Transfer)

	var transferCalls int
	env.SetOnActivityStartedListener(func(info *activity.Info, _ context.Context, _ converter.EncodedValues) {
		if info.ActivityType.Name == payment.TransferActivity {
			transferCalls++
		}
	})
	return env, &transferCalls
}

func registerPassThrough(env *testsuite.TestWorkflowEnvironment, name string) {
	env.RegisterActivityWithOptions(
		func(context.Context, payment.StepInput) error { return nil },
		activity.RegisterOptions{Name: name},
	)
}
