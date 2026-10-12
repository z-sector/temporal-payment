package activities

import (
	"context"
	"math/rand/v2"
	"strings"

	"github.com/z-sector/temporal-payment/payment"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

// Activities logs payment steps. It does not move money.
// Exported method names must match the activity name constants in the payment package.
type Activities struct{}

// Hold places a hold. A from account that starts with "f" has insufficient funds.
func (a *Activities) Hold(ctx context.Context, input payment.StepInput) error {
	if err := a.gate(ctx, input); err != nil {
		return err
	}
	if strings.HasPrefix(input.From, "f") {
		logStep(ctx, "недостаточно средств", input)
		return temporal.NewNonRetryableApplicationError(
			"недостаточно средств",
			payment.ErrInsufficientFunds,
			nil,
		)
	}
	logStep(ctx, "холдирование", input)
	return nil
}

// CheckAntifraud rejects the payment when from or to starts with "a".
func (a *Activities) CheckAntifraud(ctx context.Context, input payment.StepInput) error {
	if err := a.gate(ctx, input); err != nil {
		return err
	}
	if strings.HasPrefix(input.From, "a") || strings.HasPrefix(input.To, "a") {
		logStep(ctx, "антифрод: проверка не пройдена", input)
		return temporal.NewNonRetryableApplicationError(
			"антифрод: проверка не пройдена",
			payment.ErrAntifraudRejected,
			nil,
		)
	}
	logStep(ctx, "антифрод: проверка пройдена", input)
	return nil
}

// Transfer records a successful transfer.
func (a *Activities) Transfer(ctx context.Context, input payment.StepInput) error {
	if err := a.gate(ctx, input); err != nil {
		return err
	}
	logStep(ctx, "перевод", input)
	return nil
}

// Refund records a release of the held amount.
func (a *Activities) Refund(ctx context.Context, input payment.StepInput) error {
	if err := a.gate(ctx, input); err != nil {
		return err
	}
	logStep(ctx, "возврат холдированных средств", input)
	return nil
}

// gate returns a retryable Unavailable error on 70% of attempts.
// The business result is produced only on the other 30%.
func (a *Activities) gate(ctx context.Context, input payment.StepInput) error {
	if rand.Float64() < 0.3 {
		return nil
	}
	activity.GetLogger(ctx).Info("Unavailable",
		"step", activity.GetInfo(ctx).ActivityType.Name,
		"idempotency_key", input.IdempotencyKey,
		"from", input.From,
		"to", input.To,
		"value", input.Value,
	)
	return temporal.NewApplicationError("сервис недоступен", payment.ErrUnavailable)
}

func logStep(ctx context.Context, message string, input payment.StepInput) {
	activity.GetLogger(ctx).Info(message,
		"step", activity.GetInfo(ctx).ActivityType.Name,
		"idempotency_key", input.IdempotencyKey,
		"from", input.From,
		"to", input.To,
		"value", input.Value,
	)
}
