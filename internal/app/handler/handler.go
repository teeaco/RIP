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
	"unicode"

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
		Service: service,
		Signs:   splitBySemicolon(service.ClinicalSigns),
		Recs:    splitBySemicolon(service.Recommendations),
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

	diagnosisLabel := safeString(request.DiagnosisLabel)
	mmComment := safeString(request.MMComment)
	if shouldUseGeneratedDoctorOpinion(mmComment) || mmComment == "-" {
		mmComment = defaultRequestComment()
	}
	mmComment = resolveRequestDoctorComment(request.Items, diagnosisLabel, mmComment)

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
			BloodValuePaO2: formatPaO2(request.BloodValuePaO2),
			FiO2Value:      formatFiO2(request.FiO2Value),
			MMCoefficient:  formatCoefficient(request.MMCoefficient),
			DiagnosisLabel: diagnosisLabel,
			MMComment:      mmComment,
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

func defaultRequestComment() string {
	return "Состояние средней тяжести. Рекомендован повторный контроль коэффициента через 6 часов."
}

func normalizeServiceDescription(description string) string {
	trimmed := strings.TrimSpace(description)
	if trimmed == "" {
		return trimmed
	}

	for _, prefix := range []string{
		"Эталон степени для ",
		"эталон степени для ",
		"Эталон услуги для ",
		"эталон услуги для ",
	} {
		if strings.HasPrefix(trimmed, prefix) {
			cleaned := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
			cleaned = strings.TrimSpace(strings.TrimSuffix(cleaned, "."))
			if cleaned == "" {
				return trimmed
			}
			return normalizeKnownNominative(cleaned)
		}
	}

	lowered := strings.ToLower(trimmed)
	for _, loweredPrefix := range []string{"эталон степени для ", "эталон услуги для "} {
		if strings.HasPrefix(lowered, loweredPrefix) {
			textRunes := []rune(trimmed)
			prefixRunes := []rune(loweredPrefix)
			if len(textRunes) < len(prefixRunes) {
				return trimmed
			}
			cleaned := strings.TrimSpace(string(textRunes[len(prefixRunes):]))
			cleaned = strings.TrimSpace(strings.TrimSuffix(cleaned, "."))
			if cleaned == "" {
				return trimmed
			}
			return normalizeKnownNominative(cleaned)
		}
	}

	return normalizeKnownNominative(trimmed)
}

func resolveRequestDoctorComment(items []repository.RequestService, diagnosisLabel, fallback string) string {
	recommendation := recommendationForDiagnosis(items, diagnosisLabel)
	if recommendation != "" {
		return recommendation
	}
	if shouldUseGeneratedDoctorOpinion(fallback) || strings.TrimSpace(fallback) == "" || strings.TrimSpace(fallback) == "-" {
		return defaultRequestComment()
	}
	return fallback
}

func recommendationForDiagnosis(items []repository.RequestService, diagnosisLabel string) string {
	label := strings.TrimSpace(diagnosisLabel)
	if label != "" && label != "-" {
		for _, item := range items {
			if item.Service.Name == label {
				return strings.TrimSpace(item.Service.Recommendations)
			}
		}
	}

	for _, item := range items {
		if item.IsPrimary {
			if recommendation := strings.TrimSpace(item.Service.Recommendations); recommendation != "" {
				return recommendation
			}
		}
	}

	for _, item := range items {
		if recommendation := strings.TrimSpace(item.Service.Recommendations); recommendation != "" {
			return recommendation
		}
	}

	return ""
}

func upperFirstRune(text string) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return text
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func normalizeKnownNominative(text string) string {
	cleaned := strings.TrimSpace(strings.TrimSuffix(text, "."))
	normalized := strings.ToLower(cleaned)

	switch normalized {
	case "нормального газообмена":
		return "Нормальный газообмен."
	case "легкой дыхательной недостаточности":
		return "Легкая дыхательная недостаточность."
	case "умеренной дыхательной недостаточности":
		return "Умеренная дыхательная недостаточность."
	case "тяжелой дыхательной недостаточности":
		return "Тяжелая дыхательная недостаточность."
	default:
		if cleaned == "" {
			return ""
		}
		return upperFirstRune(cleaned) + "."
	}
}

func shouldUseGeneratedDoctorOpinion(comment string) bool {
	normalized := strings.ToLower(strings.TrimSpace(comment))
	if normalized == "" || normalized == "-" {
		return true
	}
	return strings.Contains(normalized, "по мнению врача")
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
