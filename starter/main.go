package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/z-sector/temporal-payment/payment"
	"github.com/z-sector/temporal-payment/workflows"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/contrib/envconfig"
)

func main() {
	from := flag.String("from", "", "счёт отправителя")
	to := flag.String("to", "", "счёт получателя")
	value := flag.Int64("value", 0, "сумма")
	key := flag.String("idempotency-key", "", "ключ идемпотентности платежа и workflow ID")
	flag.Parse()

	if *from == "" || *to == "" || *key == "" {
		fmt.Fprintln(os.Stderr, "флаги -from, -to и -idempotency-key обязательны")
		flag.Usage()
		os.Exit(2)
	}

	c, err := client.Dial(envconfig.MustLoadDefaultClientOptions())
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	input := payment.Input{
		From:           *from,
		To:             *to,
		Value:          *value,
		IdempotencyKey: *key,
	}
	options := client.StartWorkflowOptions{
		ID:                       *key,
		TaskQueue:                payment.TaskQueue,
		WorkflowIDReusePolicy:    enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
		WorkflowIDConflictPolicy: enums.WORKFLOW_ID_CONFLICT_POLICY_FAIL,
	}

	we, err := c.ExecuteWorkflow(context.Background(), options, workflows.PaymentWorkflow, input)
	if err != nil {
		var already *serviceerror.WorkflowExecutionAlreadyStarted
		if errors.As(err, &already) {
			log.Fatalf("платёж с ключом идемпотентности %q уже существует", *key)
		}
		log.Fatalln("Unable to execute workflow", err)
	}

	var result payment.Result
	if err := we.Get(context.Background(), &result); err != nil {
		log.Fatalln("Unable to get workflow result", err)
	}
	fmt.Printf("workflow_id=%s run_id=%s status=%s\n", we.GetID(), we.GetRunID(), result.Status)
}
