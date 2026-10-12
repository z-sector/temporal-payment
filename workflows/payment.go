package workflows

import (
	"errors"
	"time"

	"github.com/z-sector/temporal-payment/payment"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// PaymentWorkflow holds funds, checks antifraud, transfers, and refunds the hold
// when antifraud rejects the payment.
func PaymentWorkflow(ctx workflow.Context, input payment.Input) (payment.Result, error) {
	logger := workflow.GetLogger(ctx)
	ctx = withActivityOptions(ctx)

	logger.Info("старт платежа",
		"idempotency_key", input.IdempotencyKey,
		"from", input.From,
		"to", input.To,
		"value", input.Value,
	)

	err := workflow.ExecuteActivity(ctx, payment.HoldActivity, input.Step(payment.StepHold)).Get(ctx, nil)
	if err != nil {
		if applicationType(err) == payment.ErrInsufficientFunds {
			return finish(ctx, payment.StatusInsufficientFunds)
		}
		return payment.Result{}, err
	}

	err = workflow.ExecuteActivity(ctx, payment.CheckAntifraudActivity, input.Step(payment.StepAntifraud)).Get(ctx, nil)
	if err != nil {
		if applicationType(err) == payment.ErrAntifraudRejected {
			if refundErr := refund(ctx, input.Step(payment.StepRefund)); refundErr != nil {
				return payment.Result{}, refundErr
			}
			return finish(ctx, payment.StatusAntifraudRejected)
		}
		return payment.Result{}, err
	}

	err = workflow.ExecuteActivity(ctx, payment.TransferActivity, input.Step(payment.StepTransfer)).Get(ctx, nil)
	if err != nil {
		return payment.Result{}, err
	}
	return finish(ctx, payment.StatusCompleted)
}

func refund(ctx workflow.Context, input payment.StepInput) error {
	disconnectedCtx, _ := workflow.NewDisconnectedContext(ctx)
	compCtx := withActivityOptions(disconnectedCtx)
	workflow.GetLogger(ctx).Info("возврат холдированных средств", "idempotency_key", input.IdempotencyKey)
	return workflow.ExecuteActivity(compCtx, payment.RefundActivity, input).Get(compCtx, nil)
}

func finish(ctx workflow.Context, status string) (payment.Result, error) {
	workflow.GetLogger(ctx).Info("платёж завершён", "status", status)
	return payment.Result{Status: status}, nil
}

func withActivityOptions(ctx workflow.Context) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2,
			// Keep Unavailable retries short enough to watch in the worker log.
			MaximumInterval: 5 * time.Second,
			NonRetryableErrorTypes: []string{
				payment.ErrInsufficientFunds,
				payment.ErrAntifraudRejected,
			},
		},
	})
}

func applicationType(err error) string {
	var appErr *temporal.ApplicationError
	if errors.As(err, &appErr) {
		return appErr.Type()
	}
	return ""
}
