package main

import (
	"fmt"
	"net/http"
	"os"

	"payroute/internal/api"
)

func main() {
	mux := http.NewServeMux()

	// API
	mux.HandleFunc("POST /api/v1/pay", api.CreatePayment)
	mux.HandleFunc("POST /api/v1/paypal/order", api.CreatePayPalOrder)
	mux.HandleFunc("GET /api/v1/payments", api.ListPayments)
	mux.HandleFunc("GET /api/v1/channels", api.ListChannels)
	mux.HandleFunc("GET /api/v1/stats", api.Stats)

	// 静态 Dashboard
	mux.Handle("/", http.FileServer(http.Dir("./web/static")))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("PayRoute running on http://localhost:%s\n", port)
	http.ListenAndServe(":"+port, mux)
}
