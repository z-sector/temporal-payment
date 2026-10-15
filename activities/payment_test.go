package activities

import (
	"errors"
	"testing"

	"github.com/z-sector/temporal-payment/payment"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
)

func TestTransferOutageStoresKeyThenTransfers(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestActivityEnvironment()
	a := &Activities{}
	env.RegisterActivity(a.Transfer)

	input := payment.StepInput{
		From:           "uma",
		To:             "carol",
		Value:          100,
		IdempotencyKey: "pay:transfer",
	}

	recorded := false
	transferred := false
	for i := 0; i < 200 && !transferred; i++ {
		_, err := env.ExecuteActivity(a.Transfer, input)
		_, seen := a.transferFailures[input.IdempotencyKey]
		if !recorded {
			if err == nil {
				t.Fatal("transfer completed before the key was stored")
			}
			if !seen {
				continue
			}
			recorded = true
			var appErr *temporal.ApplicationError
			if !errors.As(err, &appErr) || appErr.Type() != payment.ErrUnavailable {
				t.Fatalf("first stored call returned %v", err)
			}
			if a.transferFailures[input.IdempotencyKey] != 1 {
				t.Fatalf("stored count = %d, want 1", a.transferFailures[input.IdempotencyKey])
			}
			continue
		}
		if err == nil {
			transferred = true
		}
	}
	if !recorded {
		t.Fatal("gate never passed on the first outage call")
	}
	if !transferred {
		t.Fatal("stored key never performed the transfer")
	}
	if a.transferFailures[input.IdempotencyKey] != 1 {
		t.Fatalf("stored count = %d, want 1", a.transferFailures[input.IdempotencyKey])
	}
}

func TestTransferOutageKeysAreIndependent(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestActivityEnvironment()
	a := &Activities{}
	env.RegisterActivity(a.Transfer)

	first := payment.StepInput{From: "uma", To: "carol", Value: 10, IdempotencyKey: "one:transfer"}
	second := payment.StepInput{From: "uma", To: "carol", Value: 10, IdempotencyKey: "two:transfer"}

	if !recordOutage(t, env, a, first) {
		t.Fatal("first key was not stored")
	}
	if !recordOutage(t, env, a, second) {
		t.Fatal("second key was not stored")
	}
	if err := transferUntilSuccess(t, env, a, second); err != nil {
		t.Fatal(err)
	}
	if a.transferFailures[first.IdempotencyKey] != 1 {
		t.Fatalf("first key count = %d, want 1", a.transferFailures[first.IdempotencyKey])
	}
}

func TestTransferWithoutOutageDoesNotStoreKey(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestActivityEnvironment()
	a := &Activities{}
	env.RegisterActivity(a.Transfer)

	input := payment.StepInput{From: "bob", To: "carol", Value: 15, IdempotencyKey: "ok:transfer"}
	if err := transferUntilSuccess(t, env, a, input); err != nil {
		t.Fatal(err)
	}
	if len(a.transferFailures) != 0 {
		t.Fatalf("non-outage transfer stored %#v", a.transferFailures)
	}
}

// recordOutage calls Transfer until the outage key is stored. That call must
// return Unavailable. It reports whether the key was stored within the loop.
func recordOutage(t *testing.T, env *testsuite.TestActivityEnvironment, a *Activities, input payment.StepInput) bool {
	t.Helper()
	for i := 0; i < 200; i++ {
		_, err := env.ExecuteActivity(a.Transfer, input)
		if _, seen := a.transferFailures[input.IdempotencyKey]; !seen {
			if err == nil {
				t.Fatal("transfer completed before the key was stored")
			}
			continue
		}
		var appErr *temporal.ApplicationError
		if !errors.As(err, &appErr) || appErr.Type() != payment.ErrUnavailable {
			t.Fatalf("first stored call returned %v", err)
		}
		return true
	}
	return false
}

func transferUntilSuccess(t *testing.T, env *testsuite.TestActivityEnvironment, a *Activities, input payment.StepInput) error {
	t.Helper()
	for i := 0; i < 200; i++ {
		_, err := env.ExecuteActivity(a.Transfer, input)
		if err == nil {
			return nil
		}
	}
	return errors.New("gate never passed")
}
