package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strconv"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const name = "go.opentelemetry.io/otel/example/dice"

var (
	tracer  = otel.Tracer(name)
	meter   = otel.Meter(name)
	logger  = otelslog.NewLogger(name)
	rollCnt metric.Int64Counter
)

func init() {
	var err error
	rollCnt, err = meter.Int64Counter("dice.rolls",
		metric.WithDescription("The number of rolls by roll value"),
		metric.WithUnit("{roll}"))
	if err != nil {
		panic(err)
	}
}

func rollDice(ctx context.Context) {
	roll := 1 + rand.Intn(6)

	// Logs
	logger.InfoContext(ctx, "rolling the dice", "result", roll)

	// Trace
	ctx, span := tracer.Start(ctx, "roll_dice")
	defer span.End()
	rollValueAttr := attribute.Int("roll.value", roll)
	span.SetAttributes(rollValueAttr)

	// Metric
	rollCnt.Add(ctx, 1, metric.WithAttributes(rollValueAttr))

	// Stdout for debugging
	resp := strconv.Itoa(roll) + "\n"
	log.Println(fmt.Sprintf("response: %s", resp))
}
