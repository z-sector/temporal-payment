package payment

import "strings"

const TaskQueue = "payment"

// TransferOutageAttempts is the workflow retry cap for Transfer when the sender
// name starts with "u". The activity stores the step key on the first call that
// passes the availability gate and returns Unavailable. A later call that finds
// the key performs the transfer.
const TransferOutageAttempts int32 = 3

// TransferOutage reports whether this sender must be remembered before a later
// Transfer call can perform the transfer.
func TransferOutage(from string) bool {
	return strings.HasPrefix(from, "u")
}

const (
	HoldActivity           = "Hold"
	CheckAntifraudActivity = "CheckAntifraud"
	TransferActivity       = "Transfer"
	RefundActivity         = "Refund"
)

const (
	StepHold      = "hold"
	StepAntifraud = "antifraud"
	StepTransfer  = "transfer"
	StepRefund    = "refund"
)

const (
	ErrUnavailable       = "Unavailable"
	ErrInsufficientFunds = "InsufficientFunds"
	ErrAntifraudRejected = "AntifraudRejected"
)

const (
	StatusCompleted         = "completed"
	StatusInsufficientFunds = "insufficient_funds"
	StatusAntifraudRejected = "antifraud_rejected"
)

// Input is the payment request sent by the client.
type Input struct {
	From           string `json:"from"`
	To             string `json:"to"`
	Value          int64  `json:"value"`
	IdempotencyKey string `json:"idempotency_key"`
}

// StepInput is one downstream request. IdempotencyKey is stable across activity retries.
type StepInput struct {
	From           string `json:"from"`
	To             string `json:"to"`
	Value          int64  `json:"value"`
	IdempotencyKey string `json:"idempotency_key"`
}

// Result is the business outcome of a payment.
type Result struct {
	Status string `json:"status"`
}

// Step builds a downstream request whose idempotency key is derived from the payment key.
func (in Input) Step(name string) StepInput {
	return StepInput{
		From:           in.From,
		To:             in.To,
		Value:          in.Value,
		IdempotencyKey: in.IdempotencyKey + ":" + name,
	}
}
