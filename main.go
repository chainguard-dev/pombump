package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/vaikas/pombump/cmd/pombump"
)

func main() {
	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt)
	defer done()

	if err := pombump.New().ExecuteContext(ctx); err != nil {
		log.Fatalf("error during command execution: %v", err)
	}
}
