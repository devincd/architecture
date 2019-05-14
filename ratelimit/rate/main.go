package main

import (
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

var (
	limiter = rate.NewLimiter(2, 5)
)

func limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			http.Error(w, http.StatusText(http.StatusTooManyRequests)+"; time="+time.Now().Format("2006-01-02 15:04:05.000"), http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func okHandler(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("OK; time=" + time.Now().Format("2006-01-02 15:04:05.000") + "\n"))
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", okHandler)
	_ = http.ListenAndServe(":4000", limit(mux))
}
