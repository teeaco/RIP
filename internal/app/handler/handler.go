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
	ID            uint
	Name          string
	Benchmark     string
	ImageURL      string
	VideoURL      string
	Quantity      int
	DoctorComment string
	IsDiagnosis   bool
}

type requestView struct {
	ID             uint
	PatientName    string
	BloodValuePaO2 string
	FiO2Value      string
	PaO2Input      string
	FiO2Input      string
	MMCoefficient  string
	DiagnosisLabel string
	MMComment      string
}

type indexPageData struct {
	Services  []repository.OxygenationService
	Query     string
	RequestID uint
	CartCount int
	HasDraft  bool
}

type detailPageData struct {
	Service repository.OxygenationService
	Signs   []string
	Recs    []string
}

type requestPageData struct {
	Request          requestView
	Rows             []requestServiceRow
	CalculationError string
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
		http.Error(w, "invalid service id", http.StatusBadRequest)
		return
	}

	service, err := h.repo.GetServiceByID(uint(id))
	if err != nil {
		if errors.Is(err, repository.ErrServiceNotFound) {
			http.Error(w, "service not found", http.StatusNotFound)
			return
		}
		http.Error(w, "service unavailable", http.StatusInternalServerError)
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
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid request id", http.StatusBadRequest)
		return
	}

	request, err := h.repo.GetRequestByID(demoUserID, uint(id))
	if err != nil {
		if errors.Is(err, repository.ErrRequestNotFound) || errors.Is(err, repository.ErrRequestDeleted) {
			http.Error(w, "request not found", http.StatusNotFound)
			return
		}
		http.Error(w, "request unavailable", http.StatusInternalServerError)
		return
	}

	displayPaO2 := request.BloodValuePaO2
	displayFiO2 := request.FiO2Value
	coefficient := request.MMCoefficient
	diagnosisLabel := safeString(request.DiagnosisLabel)
	calcError := ""

	rawPaO2 := strings.TrimSpace(r.URL.Query().Get("pao2"))
	rawFiO2 := strings.TrimSpace(r.URL.Query().Get("fio2"))
	hasCalculatorInput := rawPaO2 != "" || rawFiO2 != ""

	if hasCalculatorInput {
		paO2, paErr := parsePositiveFloat(rawPaO2)
		fiO2, fiErr := parsePositiveFloat(rawFiO2)
		if paErr != nil || fiErr != nil {
			calcError = "Для расчета введите числовые значения PaO2 и FiO2."
		} else {
			displayPaO2 = &paO2
			displayFiO2 = &fiO2
			if value, ok := repository.CalculateOxygenationIndex(paO2, fiO2); ok {
				coefficient = &value
				diagnosisLabel = repository.DiagnosisByOxygenationIndex(value)
			} else {
				calcError = "FiO2 должна быть больше 0."
			}
		}
	} else if displayPaO2 != nil && displayFiO2 != nil {
		if value, ok := repository.CalculateOxygenationIndex(*displayPaO2, *displayFiO2); ok {
			coefficient = &value
			diagnosisLabel = repository.DiagnosisByOxygenationIndex(value)
		}
	}

	rows := make([]requestServiceRow, 0, len(request.Items))
	for _, item := range request.Items {
		isDiagnosis := item.IsPrimary
		if strings.TrimSpace(diagnosisLabel) != "" && diagnosisLabel != "-" {
			isDiagnosis = item.Service.Name == diagnosisLabel
		}

		rows = append(rows, requestServiceRow{
			ID:            item.Service.ID,
			Name:          item.Service.Name,
			Benchmark:     item.Service.Benchmark,
			ImageURL:      safeString(item.Service.ImageURL),
			VideoURL:      safeString(item.Service.VideoURL),
			Quantity:      item.Quantity,
			DoctorComment: safeString(item.DoctorComment),
			IsDiagnosis:   isDiagnosis,
		})
	}

	data := requestPageData{
		Request: requestView{
			ID:             request.ID,
			PatientName:    safeString(request.PatientName),
			BloodValuePaO2: formatPaO2(displayPaO2),
			FiO2Value:      formatFiO2(displayFiO2),
			PaO2Input:      resolveInputValue(rawPaO2, displayPaO2, 1),
			FiO2Input:      resolveInputValue(rawFiO2, displayFiO2, 2),
			MMCoefficient:  formatCoefficient(coefficient),
			DiagnosisLabel: diagnosisLabel,
			MMComment:      safeString(request.MMComment),
		},
		Rows:             rows,
		CalculationError: calcError,
	}

	renderTemplate(w, "request.html", data)
}

func (h *Handler) AddServiceToDraft(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	serviceID, err := strconv.ParseUint(r.FormValue("service_id"), 10, 64)
	if err != nil || serviceID == 0 {
		http.Error(w, "invalid service id", http.StatusBadRequest)
		return
	}

	_, err = h.repo.AddServiceToDraft(demoUserID, uint(serviceID))
	if err != nil {
		if errors.Is(err, repository.ErrServiceNotFound) {
			http.Error(w, "service not found", http.StatusNotFound)
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
		http.Error(w, "invalid request id", http.StatusBadRequest)
		return
	}

	if err := h.repo.SoftDeleteDraftBySQL(demoUserID, uint(id)); err != nil {
		if errors.Is(err, repository.ErrRequestNotFound) {
			http.Error(w, "request not found", http.StatusNotFound)
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

func parsePositiveFloat(raw string) (float64, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(raw), ",", ".")
	value, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0, err
	}
	if value <= 0 {
		return 0, errors.New("value must be positive")
	}
	return value, nil
}

func resolveInputValue(raw string, value *float64, precision int) string {
	if strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'f', precision, 64)
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
