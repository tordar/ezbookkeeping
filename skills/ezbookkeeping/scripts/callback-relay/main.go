// callback-relay is a minimal HTTP server that receives the Enable Banking OAuth
// redirect (e.g. via ngrok) and forwards the code/state to your local backend.
//
// Usage:
//
//	go run . --port 9999 --backend http://localhost:8080
//
// Then run: ngrok http 9999
// Set Enable Banking redirect URI to: https://YOUR-NGROK-URL/callback
// Set enablebanking_callback_url in config to: https://YOUR-NGROK-URL/callback
package main

import (
	"flag"
	"log"
	"net/http"
	"net/url"
)

func main() {
	port := flag.String("port", "9999", "Port to listen on")
	backend := flag.String("backend", "http://localhost:8080", "Backend base URL to forward callback to")
	flag.Parse()

	backendURL, err := url.Parse(*backend)
	if err != nil {
		log.Fatalf("invalid backend URL: %v", err)
	}
	if backendURL.Scheme == "" || backendURL.Host == "" {
		log.Fatalf("backend must be a full URL (e.g. http://localhost:8080)")
	}

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()
		forward := url.URL{
			Scheme:   backendURL.Scheme,
			Host:     backendURL.Host,
			Path:     "/api/bank_integration/callback",
			RawQuery: q.Encode(),
		}
		log.Printf("relay: forwarding callback to %s", forward.String())
		http.Redirect(w, r, forward.String(), http.StatusFound)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("callback-relay: use GET /callback (redirect from Enable Banking)\n"))
	})

	addr := ":" + *port
	log.Printf("callback-relay listening on %s, forwarding to %s", addr, backendURL.String())
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
