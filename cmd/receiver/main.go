package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"pulseroute/internal/receiver"
	"syscall"
	"time"
)

func main() {
	secret := os.Getenv("RECEIVER_SECRET")
	if len(secret) < 16 {
		log.Fatal("RECEIVER_SECRET must contain at least 16 bytes")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}
	s := &http.Server{Addr: ":" + port, Handler: receiver.New(secret), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 35 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.Shutdown(c)
	}()
	if e := s.ListenAndServe(); e != nil && e != http.ErrServerClosed {
		log.Fatal(e)
	}
}
