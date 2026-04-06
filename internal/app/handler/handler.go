package handler

import (
	"errors"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"rip/internal/app/repository"
)

const demoUserID = repository.DefaultCreatorUserID

type Handler struct {
	repo *repository.Repository
}

type requestServiceRow struct {
	ID                uint
	Name              string
	Benchmark         string
	ImageURL          string
	VideoURL          string
	DoctorComment     string
	ResultCoefficient string
}

type requestView struct {
	ID                uint
	PatientName       string
	BloodValuePaO2    string
	FiO2Value         string
	ResultCoefficient string
	ResultLabel       string
	PrimaryService    string
}

type indexPageData struct {
	Services  []repository.OxygenationService
	Query     string
	RequestID uint
	CartCount int
	HasDraft  bool
}

type detailPageData struct {
	Service      repository.OxygenationService
	DoctorAdvice string
}

type requestPageData struct {
	Request requestView
	Rows    []requestServiceRow
}

func NewHandler(repo *repository.Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) GetServices(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	services, err := h.repo.GetServicesByQuery(query)
	if err != nil {
		http.Error(w, "services unavailable", http.StatusInternalServerError)
		return
	}

	for i := range services {
		services[i].Description = normalizeServiceDescription(services[i].Description)
	}

	cart, err := h.repo.GetDraftCart(demoUserID)
	if err != nil {
		http.Error(w, "cart unavailable", http.StatusInternalServerError)
		return
	}

	data := indexPageData{
		Services:  services,
		Query:     query,
		RequestID: cart.RequestID,
		CartCount: cart.ItemsCount,
		HasDraft:  cart.HasDraft,
	}

	renderTemplate(w, "index.html", data)
}

func (h *Handler) GetServiceDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		redirectToServices(w, r)
		return
	}

	service, err := h.repo.GetServiceByID(uint(id))
	if err != nil {
		if errors.Is(err, repository.ErrServiceNotFound) {
			redirectToServices(w, r)
			return
		}
		http.Error(w, "service unavailable", http.StatusInternalServerError)
		return
	}

	service.Description = normalizeServiceDescription(service.Description)

	data := detailPageData{
		Service:      service,
		DoctorAdvice: doctorAdvice(service),
	}

	renderTemplate(w, "detail.html", data)
}

func (h *Handler) GetRequest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		redirectToServices(w, r)
		return
	}

	request, err := h.repo.GetRequestByID(demoUserID, uint(id))
	if err != nil {
		if errors.Is(err, repository.ErrRequestNotFound) || errors.Is(err, repository.ErrRequestDeleted) {
			redirectToServices(w, r)
			return
		}
		http.Error(w, "request unavailable", http.StatusInternalServerError)
		return
	}

	rows := make([]requestServiceRow, 0, len(request.Items))
	primaryService := "-"
	var primaryCoefficient *float64

	for idx, item := range request.Items {
		rows = append(rows, requestServiceRow{
			ID:                item.Service.ID,
			Name:              item.Service.Name,
			Benchmark:         item.Service.Benchmark,
			ImageURL:          safeString(item.Service.ImageURL),
			VideoURL:          safeString(item.Service.VideoURL),
			DoctorComment:     safeString(item.DoctorComment),
			ResultCoefficient: formatCoefficient(item.ResultCoefficient),
		})

		if idx == 0 {
			primaryService = item.Service.Name
			primaryCoefficient = item.ResultCoefficient
		}
	}

	data := requestPageData{
		Request: requestView{
			ID:                request.ID,
			PatientName:       safeString(request.PatientName),
			BloodValuePaO2:    formatPaO2(request.BloodValuePaO2),
			FiO2Value:         formatFiO2(request.FiO2Value),
			ResultCoefficient: formatCoefficient(primaryCoefficient),
			ResultLabel:       resultLabel(primaryCoefficient),
			PrimaryService:    primaryService,
		},
		Rows: rows,
	}

	renderTemplate(w, "request.html", data)
}

func (h *Handler) AddServiceToDraft(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		redirectToServices(w, r)
		return
	}

	serviceID, err := strconv.ParseUint(r.FormValue("service_id"), 10, 64)
	if err != nil || serviceID == 0 {
		redirectToServices(w, r)
		return
	}

	_, err = h.repo.AddServiceToDraft(demoUserID, uint(serviceID))
	if err != nil {
		if errors.Is(err, repository.ErrServiceNotFound) {
			redirectToServices(w, r)
			return
		}
		http.Error(w, "cannot add service", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, normalizeReturnTo(r.FormValue("return_to")), http.StatusSeeOther)
}

func (h *Handler) DeleteRequest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		redirectToServices(w, r)
		return
	}

	if err := h.repo.SoftDeleteDraftBySQL(demoUserID, uint(id)); err != nil {
		if errors.Is(err, repository.ErrRequestNotFound) {
			redirectToServices(w, r)
			return
		}
		http.Error(w, "cannot delete request", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/services", http.StatusSeeOther)
}

func normalizeReturnTo(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") {
		return "/services"
	}
	return trimmed
}

func doctorAdvice(service repository.OxygenationService) string {
	if service.ID == 1 {
		return "Не идти к врачу"
	}

	return "Идти к врачу"
}

func formatPaO2(value *float64) string {
	if value == nil {
		return "-"
	}
	return strconv.FormatFloat(*value, 'f', 1, 64) + " мм рт.ст."
}

func formatFiO2(value *float64) string {
	if value == nil {
		return "-"
	}
	percent := *value * 100
	return strconv.FormatFloat(*value, 'f', 2, 64) + " (" + strconv.FormatFloat(percent, 'f', 0, 64) + "%)"
}

func formatCoefficient(value *float64) string {
	if value == nil {
		return "-"
	}
	return strconv.FormatFloat(*value, 'f', 1, 64)
}

func resultLabel(value *float64) string {
	if value == nil {
		return "-"
	}
	return repository.DiagnosisByOxygenationIndex(*value)
}

func normalizeServiceDescription(description string) string {
	return strings.TrimSpace(description)
}

func safeString(value *string) string {
	if value == nil {
		return "-"
	}
	if strings.TrimSpace(*value) == "" {
		return "-"
	}
	return *value
}

func redirectToServices(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/services", http.StatusSeeOther)
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
