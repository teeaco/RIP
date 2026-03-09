package api

import (
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"rip/internal/app/handler"
	"rip/internal/app/repository"
	"rip/internal/app/rest"
	"rip/internal/app/storage"
)

func StartServer() {
	repo, err := repository.NewRepository(repository.Config{
		Host:     envOrDefault("DB_HOST", "127.0.0.1"),
		Port:     envOrDefault("DB_PORT", "55432"),
		User:     envOrDefault("DB_USER", "root"),
		Password: envOrDefault("DB_PASSWORD", "root"),
		DBName:   envOrDefault("DB_NAME", "RIP"),
		SSLMode:  envOrDefault("DB_SSLMODE", "disable"),
	})
	if err != nil {
		log.Fatalf("database init failed: %v", err)
	}

	h := handler.NewHandler(repo)
	var uploader storage.Uploader
	minioUploader, uploaderErr := storage.NewMinioUploader(storage.MinioConfig{
		Endpoint:      envOrDefault("MINIO_ENDPOINT", "localhost:9000"),
		AccessKey:     envOrDefault("MINIO_ACCESS_KEY", "root"),
		SecretKey:     envOrDefault("MINIO_SECRET_KEY", "rootroot"),
		UseSSL:        envBool("MINIO_USE_SSL", false),
		Bucket:        envOrDefault("MINIO_BUCKET", "images"),
		PublicBaseURL: envOrDefault("MINIO_PUBLIC_BASE_URL", "http://localhost:9000/images"),
	})
	if uploaderErr != nil {
		log.Printf("minio uploader disabled: %v", uploaderErr)
	} else {
		uploader = minioUploader
	}
	apiHandler := rest.NewHandler(repo, uploader)

	mux := http.NewServeMux()
	apiHandler.RegisterRoutes(mux)

	mux.HandleFunc("GET /services", h.GetServices)
	mux.HandleFunc("GET /services/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/services", http.StatusFound)
	})
	mux.HandleFunc("GET /services/{id}", h.GetServiceDetail)
	mux.HandleFunc("GET /oxygenation_request/{id}", h.GetRequest)
	mux.HandleFunc("POST /oxygenation_request/add-service", h.AddServiceToDraft)
	mux.HandleFunc("POST /oxygenation_request/{id}/delete", h.DeleteRequest)

	mux.HandleFunc("GET /{path...}", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/services", http.StatusFound)
	})

	staticFS := http.FileServer(http.Dir(resolveProjectPath("resources")))
	mux.Handle("GET /static/", http.StripPrefix("/static/", noCache(staticFS)))

	port := envOrDefault("APP_PORT", "8080")
	fallbackPort := envOrDefault("APP_FALLBACK_PORT", "8095")
	listener, err := listenWithFallback(port, fallbackPort)
	if err != nil {
		log.Fatalf("server failed: %v", err)
	}
	activePort := port
	if listener.Addr() != nil {
		if tcpAddr, ok := listener.Addr().(*net.TCPAddr); ok && tcpAddr.Port > 0 {
			activePort = strconv.Itoa(tcpAddr.Port)
		}
	}

	server := &http.Server{
		Handler:           loggingMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("server started at http://localhost:%s", activePort)
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}

func listenWithFallback(primaryPort, fallbackPort string) (net.Listener, error) {
	listener, err := net.Listen("tcp", ":"+primaryPort)
	if err == nil {
		return listener, nil
	}

	if primaryPort == "8080" && strings.TrimSpace(fallbackPort) != "" {
		log.Printf("port 8080 is busy, trying %s", fallbackPort)
		return net.Listen("tcp", ":"+fallbackPort)
	}

	return nil, err
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
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
