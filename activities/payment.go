package activities

import (
	"context"
	"math/rand/v2"
	"strings"
	"sync"

	"github.com/z-sector/temporal-payment/payment"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

// Activities logs payment steps. It does not move money.
// Exported method names must match the activity name constants in the payment package.
//
// seenTransfers is process memory for the transfer outage. The worker registers
// one Activities value and must keep that process running: the first call stores
// the step key, and a later Transfer call for that key performs the transfer.
type Activities struct {
	mu            sync.Mutex
	seenTransfers map[string]struct{}
}

// Hold places a hold. A from account that starts with "f" has insufficient funds.
func (a *Activities) Hold(ctx context.Context, input payment.StepInput) error {
	if err := a.gate(ctx, input); err != nil {
		return err
	}
	if strings.HasPrefix(input.From, "f") {
		logStep(ctx, "insufficient funds", input)
		return temporal.NewNonRetryableApplicationError(
			"insufficient funds",
			payment.ErrInsufficientFunds,
			nil,
		)
	}
	logStep(ctx, "hold", input)
	return nil
}

// CheckAntifraud rejects the payment when from or to starts with "a".
func (a *Activities) CheckAntifraud(ctx context.Context, input payment.StepInput) error {
	if err := a.gate(ctx, input); err != nil {
		return err
	}
	if strings.HasPrefix(input.From, "a") || strings.HasPrefix(input.To, "a") {
		logStep(ctx, "antifraud check rejected", input)
		return temporal.NewNonRetryableApplicationError(
			"antifraud check rejected",
			payment.ErrAntifraudRejected,
			nil,
		)
	}
	logStep(ctx, "antifraud check passed", input)
	return nil
}

// Transfer records a successful transfer.
// Every call starts at the availability gate. A from account that starts with "u"
// is stored on the first call that passes the gate, and that call returns
// Unavailable. A later call for a key already in the map performs the transfer.
func (a *Activities) Transfer(ctx context.Context, input payment.StepInput) error {
	if err := a.gate(ctx, input); err != nil {
		return err
	}
	if strings.HasPrefix(input.From, "u") {
		return a.recordTransferOutage(ctx, input)
	}
	logStep(ctx, "transfer", input)
	return nil
}

// recordTransferOutage stores the step key the first time it is seen and returns
// Unavailable. When the key is already in the map, it performs the transfer.
func (a *Activities) recordTransferOutage(ctx context.Context, input payment.StepInput) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.seenTransfers == nil {
		a.seenTransfers = make(map[string]struct{})
	}
	key := input.IdempotencyKey
	if _, seen := a.seenTransfers[key]; seen {
		logStep(ctx, "transfer", input)
		return nil
	}
	a.seenTransfers[key] = struct{}{}
	return unavailable(ctx, input)
}

// Refund records a release of the held amount.
func (a *Activities) Refund(ctx context.Context, input payment.StepInput) error {
	if err := a.gate(ctx, input); err != nil {
		return err
	}
	logStep(ctx, "refund held funds", input)
	return nil
}

// gate returns a retryable Unavailable error on 70% of attempts.
// The business result is produced only on the other 30%.
func (a *Activities) gate(ctx context.Context, input payment.StepInput) error {
	if rand.Float64() < 0.3 {
		return nil
	}
	return unavailable(ctx, input)
}

func unavailable(ctx context.Context, input payment.StepInput) error {
	activity.GetLogger(ctx).Info("Unavailable",
		"step", activity.GetInfo(ctx).ActivityType.Name,
		"idempotency_key", input.IdempotencyKey,
		"from", input.From,
		"to", input.To,
		"value", input.Value,
	)
	return temporal.NewApplicationError("service unavailable", payment.ErrUnavailable)
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
