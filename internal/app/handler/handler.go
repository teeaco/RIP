package handler

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"rip/internal/app/repository"
)

type Handler struct {
	repo *repository.Repository
}

type requestServiceRow struct {
	ID          int
	Name        string
	Benchmark   string
	ImageURL    string
	VideoURL    string
	MMValue     string
	InRequest   string
	IsDiagnosis bool
}

type indexPageData struct {
	Services  []repository.Service
	Query     string
	RequestID int
	CartCount int
}

type detailPageData struct {
	Service repository.Service
	Signs   []string
	Recs    []string
}

type requestPageData struct {
	Request repository.Request
	Rows    []requestServiceRow
}

func NewHandler(repo *repository.Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) GetServices(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	services := h.repo.GetServicesByQuery(query)

	request, err := h.repo.GetRequestByID(101)
	if err != nil {
		http.Error(w, "request not found", http.StatusInternalServerError)
		return
	}

	data := indexPageData{
		Services:  services,
		Query:     query,
		RequestID: request.ID,
		CartCount: len(request.ServiceIDs),
	}

	renderTemplate(w, "index.html", data)
}

func (h *Handler) GetServiceDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid service id", http.StatusBadRequest)
		return
	}

	service, err := h.repo.GetServiceByID(id)
	if err != nil {
		http.Error(w, "service not found", http.StatusNotFound)
		return
	}

	data := detailPageData{
		Service: service,
		Signs:   splitBySemicolon(service.ClinicalSigns),
		Recs:    splitBySemicolon(service.Recommendations),
	}

	renderTemplate(w, "detail.html", data)
}

func (h *Handler) GetRequest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid request id", http.StatusBadRequest)
		return
	}

	request, err := h.repo.GetRequestByID(id)
	if err != nil {
		http.Error(w, "request not found", http.StatusNotFound)
		return
	}

	rows := make([]requestServiceRow, 0, len(request.ServiceIDs))
	for _, serviceID := range request.ServiceIDs {
		service, serviceErr := h.repo.GetServiceByID(serviceID)
		if serviceErr != nil {
			continue
		}

		mmValue := request.MMByServiceID[service.ID]
		if strings.TrimSpace(mmValue) == "" {
			mmValue = "-"
		}

		statusLabel := "Контроль в динамике"
		isDiagnosis := false
		if mmValue != "-" {
			statusLabel = "Текущая степень"
			isDiagnosis = true
		}

		rows = append(rows, requestServiceRow{
			ID:          service.ID,
			Name:        service.Name,
			Benchmark:   service.Benchmark,
			ImageURL:    service.ImageURL,
			VideoURL:    service.VideoURL,
			MMValue:     mmValue,
			InRequest:   statusLabel,
			IsDiagnosis: isDiagnosis,
		})
	}

	data := requestPageData{
		Request: request,
		Rows:    rows,
	}

	renderTemplate(w, "request.html", data)
}

func splitBySemicolon(text string) []string {
	parts := strings.Split(text, ";")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func renderTemplate(w http.ResponseWriter, templateName string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	tmpl, err := template.ParseFiles(
		resolveProjectPath("templates", "partials", "header.html"),
		resolveProjectPath("templates", templateName),
	)
	if err != nil {
		log.Printf("template parse error (%s): %v", templateName, err)
		http.Error(w, "template parse error", http.StatusInternalServerError)
		return
	}

	if err = tmpl.ExecuteTemplate(w, templateName, data); err != nil {
		log.Printf("template execute error (%s): %v", templateName, err)
		http.Error(w, "template execute error", http.StatusInternalServerError)
	}
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
