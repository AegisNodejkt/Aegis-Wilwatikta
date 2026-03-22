package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/aegis-wilwatikta/example/internal/auth"
)

func main() {
	service := auth.NewAuthService()

	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		user := r.URL.Query().Get("user")
		pass := r.URL.Query().Get("pass")

		// 🛡️ Aegis-Wilwatikta Architect should catch this logic bug:
		// We're returning the raw token in the response header!
		token, err := service.Login(user, pass)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("X-Auth-Token", token)
		fmt.Fprintf(w, "Welcome, %s!", user)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("🚀 Example Server starting on :%s\n", port)
	http.ListenAndServe(":"+port, nil)
}
