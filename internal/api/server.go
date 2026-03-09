package api

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"rip/internal/app/handler"
	"rip/internal/app/repository"
)

func StartServer() {
	repo := repository.NewRepository()
	h := handler.NewHandler(repo)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/services", http.StatusFound)
	})
	mux.HandleFunc("GET /services", h.GetServices)
	mux.HandleFunc("GET /services/{id}", h.GetServiceDetail)
	mux.HandleFunc("GET /oxygenation_request/{id}", h.GetRequest)
	mux.HandleFunc("GET /requests/{id}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/oxygenation_request/"+r.PathValue("id"), http.StatusMovedPermanently)
	})

	staticFS := http.FileServer(http.Dir(resolveProjectPath("resources")))
	mux.Handle("GET /static/", http.StripPrefix("/static/", noCache(staticFS)))

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           loggingMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("server started at http://localhost:%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.RequestURI(), time.Since(start))
	})
}

func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}

func resolveProjectPath(parts ...string) string {
	candidates := []string{
		filepath.Join(parts...),
		filepath.Join(append([]string{"..", ".."}, parts...)...),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return filepath.Join(parts...)
}
