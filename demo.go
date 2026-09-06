package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/tomgachet/conso-dashboard/internal/storage"
)

func runDemo(args []string) error {
	flags := flag.NewFlagSet("demo", flag.ContinueOnError)
	addr := flags.String("addr", "127.0.0.1:3457", "adresse d'écoute HTTP")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("argument inattendu %q", flags.Arg(0))
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	store, err := storage.NewDemo(ctx, time.Now())
	if err != nil {
		return err
	}
	defer store.Close()
	log.Print("démo : données fictives en mémoire, aucun appel à l’API ; Ctrl+C pour arrêter")
	return serveDashboard(*addr, store, true)
}
