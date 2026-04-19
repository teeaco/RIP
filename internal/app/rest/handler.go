package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"rip/internal/app/actor"
	"rip/internal/app/repository"
	"rip/internal/app/storage"
)

type Handler struct {
	repo     *repository.Repository
	uploader storage.Uploader
}

func NewHandler(repo *repository.Repository, uploader storage.Uploader) *Handler {
	return &Handler{repo: repo, uploader: uploader}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/services", h.GetServices)
	mux.HandleFunc("GET /api/services/{id}", h.GetServiceByID)
	mux.HandleFunc("POST /api/services", h.CreateService)

	mux.HandleFunc("POST /api/request-services", h.AddServiceToDraft)
	mux.HandleFunc("PUT /api/request-services/{requestID}/{serviceID}", h.UpdateRequestService)
	mux.HandleFunc("DELETE /api/request-services/{requestID}/{serviceID}", h.DeleteRequestService)

	mux.HandleFunc("GET /api/oxygenation_request/cart", h.GetCart)
	mux.HandleFunc("GET /api/oxygenation_request", h.ListRequests)
	mux.HandleFunc("GET /api/oxygenation_request/{id}", h.GetRequestByID)
	mux.HandleFunc("PUT /api/oxygenation_request/{id}", h.UpdateRequest)
	mux.HandleFunc("PUT /api/oxygenation_request/{id}/form", h.FormRequest)
	mux.HandleFunc("PUT /api/oxygenation_request/{id}/review", h.ReviewRequest)
	mux.HandleFunc("DELETE /api/oxygenation_request/{id}", h.DeleteRequest)

	mux.HandleFunc("POST /api/users/register", h.RegisterUser)
	mux.HandleFunc("POST /api/users/login", h.LoginUser)
	mux.HandleFunc("POST /api/users/logout", h.LogoutUser)
}

type errorResponse struct {
	Error string `json:"error"`
}

type serviceResponse struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	ImageURL    string `json:"image_url,omitempty"`
	VideoURL    string `json:"video_url,omitempty"`
	Benchmark   string `json:"benchmark"`
	CreatedAt   string `json:"created_at"`
}

type requestServiceResponse struct {
	RequestID         uint   `json:"request_id"`
	ServiceID         uint   `json:"service_id"`
	ServiceName       string `json:"service_name"`
	ImageURL          string `json:"image_url,omitempty"`
	VideoURL          string `json:"video_url,omitempty"`
	Benchmark         string `json:"benchmark,omitempty"`
	DoctorComment     string `json:"doctor_comment,omitempty"`
	ResultCoefficient any    `json:"result_coefficient"`
}

type requestResponse struct {
	ID                uint                     `json:"id"`
	CreatedAt         string                   `json:"created_at"`
	FormedAt          string                   `json:"formed_at,omitempty"`
	CompletedAt       string                   `json:"completed_at,omitempty"`
	CreatorLogin      string                   `json:"creator_login,omitempty"`
	ModeratorLogin    string                   `json:"moderator_login,omitempty"`
	PatientName       string                   `json:"patient_name"`
	BloodValuePaO2    any                      `json:"blood_value_pao2"`
	FiO2Value         any                      `json:"fio2_value"`
	PrimaryService    string                   `json:"primary_service"`
	ResultCoefficient any                      `json:"result_coefficient"`
	Result            string                   `json:"result"`
	ResultsCount      int                      `json:"results_count"`
	Items             []requestServiceResponse `json:"items,omitempty"`
}

type registerUserRequest struct {
	Login    string `json:"login"`
	FullName string `json:"full_name"`
	Password string `json:"password"`
}

type loginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type updateRequestPayload struct {
	PatientName    *string  `json:"patient_name"`
	BloodValuePaO2 *float64 `json:"blood_value_pao2"`
	FiO2Value      *float64 `json:"fio2_value"`
}

type addRequestServicePayload struct {
	ServiceID uint `json:"service_id"`
}

type updateRequestServicePayload struct {
	DoctorComment *string `json:"doctor_comment"`
}

type reviewRequestPayload struct {
	Action string `json:"action"`
}

func (h *Handler) GetServices(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	services, err := h.repo.GetServicesByQuery(query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "services unavailable")
		return
	}

	out := make([]serviceResponse, 0, len(services))
	for _, service := range services {
		out = append(out, serializeService(service))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": out,
	})
}

func (h *Handler) GetServiceByID(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathUint(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid service id")
		return
	}

	service, err := h.repo.GetServiceByID(id)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, serializeService(service))
}

func (h *Handler) CreateService(w http.ResponseWriter, r *http.Request) {
	if h.uploader == nil {
		writeError(w, http.StatusServiceUnavailable, "media storage is unavailable")
		return
	}

	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart payload")
		return
	}

	imageURL, err := h.uploadOptionalFile(r, "image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "image upload failed")
		return
	}
	videoURL, err := h.uploadOptionalFile(r, "video")
	if err != nil {
		writeError(w, http.StatusBadRequest, "video upload failed")
		return
	}

	service, err := h.repo.CreateService(repository.ServiceCreateInput{
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
		Benchmark:   r.FormValue("benchmark"),
		ImageURL:    imageURL,
		VideoURL:    videoURL,
	})
	if err != nil {
		writeRepositoryError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, serializeService(service))
}

func (h *Handler) AddServiceToDraft(w http.ResponseWriter, r *http.Request) {
	var payload addRequestServicePayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	if payload.ServiceID == 0 {
		writeError(w, http.StatusBadRequest, "service_id is required")
		return
	}

	current := actor.Current()
	requestID, err := h.repo.AddServiceToDraft(current.CreatorID, payload.ServiceID)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}

	cart, err := h.repo.GetDraftCart(current.CreatorID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cart unavailable")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"request_id": requestID,
		"cart": map[string]any{
			"request_id":  cart.RequestID,
			"items_count": cart.ItemsCount,
			"has_draft":   cart.HasDraft,
		},
	})
}

func (h *Handler) UpdateRequestService(w http.ResponseWriter, r *http.Request) {
	requestID, err := parsePathUint(r, "requestID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request id")
		return
	}
	serviceID, err := parsePathUint(r, "serviceID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid service id")
		return
	}

	var payload updateRequestServicePayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}

	current := actor.Current()
	item, err := h.repo.UpdateRequestServiceInDraft(current.CreatorID, requestID, serviceID, repository.RequestServiceUpdateInput{
		DoctorComment: payload.DoctorComment,
	})
	if err != nil {
		writeRepositoryError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, serializeRequestService(item))
}

func (h *Handler) DeleteRequestService(w http.ResponseWriter, r *http.Request) {
	requestID, err := parsePathUint(r, "requestID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request id")
		return
	}
	serviceID, err := parsePathUint(r, "serviceID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid service id")
		return
	}

	current := actor.Current()
	if err := h.repo.RemoveRequestServiceFromDraft(current.CreatorID, requestID, serviceID); err != nil {
		writeRepositoryError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetCart(w http.ResponseWriter, r *http.Request) {
	current := actor.Current()
	cart, err := h.repo.GetDraftCart(current.CreatorID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cart unavailable")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"request_id":  cart.RequestID,
		"items_count": cart.ItemsCount,
		"has_draft":   cart.HasDraft,
	})
}

func (h *Handler) ListRequests(w http.ResponseWriter, r *http.Request) {
	filter, err := parseRequestListFilter(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	items, err := h.repo.ListRequestsForAPI(filter)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}

	out := make([]requestResponse, 0, len(items))
	for _, item := range items {
		out = append(out, serializeRequest(item.Request, item.CreatorLogin, item.ModeratorLogin, item.ResultsCount, false))
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) GetRequestByID(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathUint(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request id")
		return
	}

	current := actor.Current()
	request, err := h.repo.GetRequestByIDWithRelations(current.CreatorID, id)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}

	var moderatorLogin *string
	if request.Moderator != nil && strings.TrimSpace(request.Moderator.Login) != "" {
		login := request.Moderator.Login
		moderatorLogin = &login
	}

	writeJSON(w, http.StatusOK, serializeRequest(request, request.Creator.Login, moderatorLogin, 0, true))
}

func (h *Handler) UpdateRequest(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathUint(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request id")
		return
	}

	var payload updateRequestPayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}

	current := actor.Current()
	request, err := h.repo.UpdateRequestFields(current.CreatorID, id, repository.RequestUpdateInput{
		PatientName:    payload.PatientName,
		BloodValuePaO2: payload.BloodValuePaO2,
		FiO2Value:      payload.FiO2Value,
	})
	if err != nil {
		writeRepositoryError(w, err)
		return
	}

	var moderatorLogin *string
	if request.Moderator != nil && strings.TrimSpace(request.Moderator.Login) != "" {
		login := request.Moderator.Login
		moderatorLogin = &login
	}

	writeJSON(w, http.StatusOK, serializeRequest(request, request.Creator.Login, moderatorLogin, 0, true))
}

func (h *Handler) FormRequest(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathUint(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request id")
		return
	}

	current := actor.Current()
	request, err := h.repo.FormDraftRequest(current.CreatorID, id)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}

	var moderatorLogin *string
	if request.Moderator != nil && strings.TrimSpace(request.Moderator.Login) != "" {
		login := request.Moderator.Login
		moderatorLogin = &login
	}

	writeJSON(w, http.StatusOK, serializeRequest(request, request.Creator.Login, moderatorLogin, 0, true))
}

func (h *Handler) ReviewRequest(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathUint(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request id")
		return
	}

	var payload reviewRequestPayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}

	current := actor.Current()
	request, err := h.repo.ReviewFormedRequest(id, current.ModeratorID, payload.Action)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}

	var moderatorLogin *string
	if request.Moderator != nil && strings.TrimSpace(request.Moderator.Login) != "" {
		login := request.Moderator.Login
		moderatorLogin = &login
	}

	writeJSON(w, http.StatusOK, serializeRequest(request, request.Creator.Login, moderatorLogin, 0, true))
}

func (h *Handler) DeleteRequest(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathUint(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request id")
		return
	}

	current := actor.Current()
	if err := h.repo.DeleteDraftRequest(current.CreatorID, id); err != nil {
		writeRepositoryError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	var payload registerUserRequest
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}

	user, err := h.repo.RegisterUser(payload.Login, payload.FullName, payload.Password)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":        user.ID,
		"login":     user.Login,
		"full_name": user.FullName,
		"role":      user.Role,
	})
}

func (h *Handler) LoginUser(w http.ResponseWriter, r *http.Request) {
	var payload loginRequest
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}

	user, err := h.repo.AuthenticateUser(payload.Login, payload.Password)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "auth stub success",
		"token":   "stub-token-" + strconv.FormatUint(uint64(user.ID), 10),
		"user": map[string]any{
			"id":        user.ID,
			"login":     user.Login,
			"full_name": user.FullName,
			"role":      user.Role,
		},
	})
}

func (h *Handler) LogoutUser(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "logout stub success",
	})
}

func (h *Handler) uploadOptionalFile(r *http.Request, field string) (*string, error) {
	file, header, err := r.FormFile(field)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	url, err := h.uploader.UploadFormFile(r.Context(), file, header)
	if err != nil {
		return nil, err
	}
	return &url, nil
}

func parseRequestListFilter(r *http.Request) (repository.RequestListFilter, error) {
	filter := repository.RequestListFilter{}
	query := r.URL.Query()

	statusText := strings.ToLower(strings.TrimSpace(query.Get("status")))
	if statusText != "" {
		switch repository.RequestStatus(statusText) {
		case repository.RequestStatusFormed, repository.RequestStatusCompleted, repository.RequestStatusRejected:
			status := repository.RequestStatus(statusText)
			filter.Status = &status
		default:
			return repository.RequestListFilter{}, errors.New("unsupported status filter")
		}
	}

	formedFromRaw := strings.TrimSpace(query.Get("formed_from"))
	if formedFromRaw != "" {
		formedFrom, err := parseDateOrDateTime(formedFromRaw)
		if err != nil {
			return repository.RequestListFilter{}, errors.New("invalid formed_from")
		}
		filter.FormedFrom = &formedFrom
	}

	formedToRaw := strings.TrimSpace(query.Get("formed_to"))
	if formedToRaw != "" {
		formedTo, err := parseDateOrDateTime(formedToRaw)
		if err != nil {
			return repository.RequestListFilter{}, errors.New("invalid formed_to")
		}
		filter.FormedTo = &formedTo
	}

	return filter, nil
}

func parseDateOrDateTime(value string) (time.Time, error) {
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02",
		"2006-01-02 15:04:05",
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, errors.New("unsupported date format")
}

func parsePathUint(r *http.Request, key string) (uint, error) {
	value := strings.TrimSpace(r.PathValue(key))
	if value == "" {
		return 0, errors.New("empty path parameter")
	}
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}

func serializeService(service repository.OxygenationService) serviceResponse {
	return serviceResponse{
		ID:          service.ID,
		Name:        service.Name,
		Description: service.Description,
		Status:      string(service.Status),
		ImageURL:    nullableStringToValue(service.ImageURL),
		VideoURL:    nullableStringToValue(service.VideoURL),
		Benchmark:   service.Benchmark,
		CreatedAt:   service.CreatedAt.Format(time.RFC3339),
	}
}

func serializeRequestService(item repository.RequestService) requestServiceResponse {
	return requestServiceResponse{
		RequestID:         item.RequestID,
		ServiceID:         item.ServiceID,
		ServiceName:       item.Service.Name,
		ImageURL:          nullableStringToValue(item.Service.ImageURL),
		VideoURL:          nullableStringToValue(item.Service.VideoURL),
		Benchmark:         item.Service.Benchmark,
		DoctorComment:     nullableStringToValue(item.DoctorComment),
		ResultCoefficient: nullableFloatToAny(item.ResultCoefficient),
	}
}

func serializeRequest(request repository.OxygenationRequest, creatorLogin string, moderatorLogin *string, resultsCount int, withItems bool) requestResponse {
	primaryService, resultCoefficient := requestResultInfo(request)

	response := requestResponse{
		ID:                request.ID,
		CreatedAt:         request.CreatedAt.Format(time.RFC3339),
		FormedAt:          nullableTimeToValue(request.FormedAt),
		CompletedAt:       nullableTimeToValue(request.CompletedAt),
		CreatorLogin:      creatorLogin,
		ModeratorLogin:    nullableStringToValue(moderatorLogin),
		PatientName:       nullableStringToValue(request.PatientName),
		BloodValuePaO2:    nullableFloatToAny(request.BloodValuePaO2),
		FiO2Value:         nullableFloatToAny(request.FiO2Value),
		PrimaryService:    primaryService,
		ResultCoefficient: nullableFloatToAny(resultCoefficient),
		Result:            resultText(resultCoefficient),
		ResultsCount:      resultsCount,
	}

	if withItems {
		response.Items = make([]requestServiceResponse, 0, len(request.Items))
		for _, item := range request.Items {
			response.Items = append(response.Items, serializeRequestService(item))
		}
		response.ResultsCount = countCalculatedResults(request.Items)
	}

	return response
}

func requestResultInfo(request repository.OxygenationRequest) (string, *float64) {
	if len(request.Items) == 0 {
		return "", nil
	}

	first := request.Items[0]
	return first.Service.Name, first.ResultCoefficient
}

func resultText(coefficient *float64) string {
	if coefficient == nil {
		return "не рассчитан"
	}
	return repository.DiagnosisByOxygenationIndex(*coefficient)
}

func countCalculatedResults(items []repository.RequestService) int {
	count := 0
	for _, item := range items {
		if item.ResultCoefficient != nil {
			count++
		}
	}
	return count
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeRepositoryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrValidationFailed):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, repository.ErrServiceNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, repository.ErrRequestNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, repository.ErrRequestDeleted):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, repository.ErrRequestServiceNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, repository.ErrInvalidTransition):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func nullableStringToValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func nullableTimeToValue(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func nullableFloatToAny(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}
