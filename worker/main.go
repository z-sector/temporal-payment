package main

import (
	"log"

	"github.com/z-sector/temporal-payment/activities"
	"github.com/z-sector/temporal-payment/payment"
	"github.com/z-sector/temporal-payment/workflows"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/contrib/envconfig"
	"go.temporal.io/sdk/worker"
)

func main() {
	c, err := client.Dial(envconfig.MustLoadDefaultClientOptions())
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	w := worker.New(c, payment.TaskQueue, worker.Options{})
	w.RegisterWorkflow(workflows.PaymentWorkflow)
	// One value for the process lifetime. Transfer's outage memory is stored here.
	w.RegisterActivity(&activities.Activities{})

	log.Printf("worker started, task queue %s", payment.TaskQueue)
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalln("Unable to start worker", err)
	}
}
