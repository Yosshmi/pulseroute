package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"pulseroute/internal/democlient"
	"pulseroute/internal/security"
	"time"
)

func main() {
	base := flag.String("url", "http://localhost:8080", "API URL")
	endpoint := flag.String("endpoint", "http://receiver:8090", "destination base URL")
	email := flag.String("email", "demo@pulseroute.test", "synthetic account email")
	n := flag.Int("events", 30, "number of synthetic events")
	flag.Parse()
	password := os.Getenv("DEMO_PASSWORD")
	if password == "" {
		password = security.Token("demo_")
		fmt.Println("Generated demo password (store locally):", password)
	}
	secret := os.Getenv("RECEIVER_SECRET")
	if len(secret) < 16 {
		log.Fatal("set RECEIVER_SECRET to the receiver's configured secret")
	}
	c := democlient.New(*base)
	project, key, e := c.Bootstrap(*email, password, *endpoint, secret)
	if e != nil {
		log.Fatal(e)
	}
	types := []string{"payment.completed", "order.created", "subscription.cancelled", "conversion.completed", "user.created", "invoice.failed"}
	for i := 0; i < *n; i++ {
		_, e = c.Call("POST", "/v1/events", key, map[string]any{"event_id": fmt.Sprintf("seed-%d-%d", time.Now().UnixNano(), i), "type": types[i%len(types)], "data": map[string]any{"synthetic": true, "sequence": i, "value": 1499}})
		if e != nil {
			log.Fatal(e)
		}
		time.Sleep(25 * time.Millisecond)
	}
	fmt.Printf("Seeded %d events in project %s. Sign in as %s.\n", *n, project, *email)
}
