# Local Temporal cluster, the payment worker, and the payment scenarios.
#
#   make up        docker compose up -d
#   make down      docker compose down -v
#   make worker    go run ./worker
#   make scenarios one new run of each scenario, ending with transfer-down
#
# The idempotency key is the workflow ID. A second start with the same key is
# rejected, so each scenario run appends a new suffix unless KEY is set:
#
#   make scenario-ok KEY=pay-ok-1

.DEFAULT_GOAL := help

COMPOSE ?= docker compose

# Expanded when a recipe runs, so two invocations (and two targets in one
# invocation) do not share a workflow ID.
run_suffix = $(shell date +%Y%m%d%H%M%S)-$(shell od -An -N2 -tx1 /dev/urandom | tr -d ' \n')

.PHONY: help up down worker \
	scenario-ok scenario-nofunds scenario-fraud-from scenario-fraud-to \
	scenario-transfer-down scenarios

help:
	@printf '%s\n' \
		'up                       docker compose up -d' \
		'down                     docker compose down -v' \
		'worker                   go run ./worker' \
		'scenario-ok              completed payment' \
		'scenario-nofunds         insufficient funds (from starts with f)' \
		'scenario-fraud-from      antifraud reject (from starts with a)' \
		'scenario-fraud-to        antifraud reject (to starts with a)' \
		'scenario-transfer-down   u-prefix transfer; first stored call is Unavailable' \
		'scenarios                ok, nofunds, fraud-from, fraud-to, transfer-down' \
		'' \
		'KEY=pay-ok-1 make scenario-ok   pin the workflow ID for one scenario'

up:
	$(COMPOSE) up -d

down:
	$(COMPOSE) down -v

worker:
	go run ./worker

scenario-ok:
	go run ./starter -from bob -to carol -value 100 -idempotency-key $(if $(KEY),$(KEY),pay-ok-$(run_suffix))

scenario-nofunds:
	go run ./starter -from frank -to carol -value 50 -idempotency-key $(if $(KEY),$(KEY),pay-nofunds-$(run_suffix))

scenario-fraud-from:
	go run ./starter -from alice -to carol -value 75 -idempotency-key $(if $(KEY),$(KEY),pay-fraud-from-$(run_suffix))

scenario-fraud-to:
	go run ./starter -from bob -to ann -value 80 -idempotency-key $(if $(KEY),$(KEY),pay-fraud-to-$(run_suffix))

scenario-transfer-down:
	go run ./starter -from uma -to carol -value 100 -idempotency-key $(if $(KEY),$(KEY),pay-transfer-down-$(run_suffix))

scenarios: scenario-ok scenario-nofunds scenario-fraud-from scenario-fraud-to scenario-transfer-down
