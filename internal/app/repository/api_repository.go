package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ServiceCreateInput struct {
	Name        string
	Description string
	Benchmark   string
	ImageURL    *string
	VideoURL    *string
}

type RequestServiceUpdateInput struct {
	Quantity  *int
	Position  *int
	IsPrimary *bool
}

type RequestUpdateInput struct {
	PatientName    *string
	BloodValuePaO2 *float64
	FiO2Value      *float64
}

type RequestListFilter struct {
	Status     *RequestStatus
	FormedFrom *time.Time
	FormedTo   *time.Time
}

type RequestListItem struct {
	Request        OxygenationRequest
	CreatorLogin   string
	ModeratorLogin *string
	ResultsCount   int
}

func (r *Repository) CreateService(input ServiceCreateInput) (OxygenationService, error) {
	name := strings.TrimSpace(input.Name)
	description := strings.TrimSpace(input.Description)
	benchmark := strings.TrimSpace(input.Benchmark)
	if name == "" || description == "" || benchmark == "" {
		return OxygenationService{}, ErrValidationFailed
	}

	service := OxygenationService{
		Name:        name,
		Description: description,
		Status:      ServiceStatusActive,
		ImageURL:    normalizeNullableString(input.ImageURL),
		VideoURL:    normalizeNullableString(input.VideoURL),
		Benchmark:   benchmark,
		CreatedAt:   time.Now().UTC(),
	}

	if err := r.db.Create(&service).Error; err != nil {
		return OxygenationService{}, err
	}

	return service, nil
}

func (r *Repository) GetRequestByIDWithRelations(userID, requestID uint) (OxygenationRequest, error) {
	var request OxygenationRequest
	if err := r.db.
		Where("id = ? AND creator_id = ?", requestID, userID).
		Preload("Creator").
		Preload("Moderator").
		Preload("Items", func(db *gorm.DB) *gorm.DB {
			return db.Order("position asc")
		}).
		Preload("Items.Service").
		First(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return OxygenationRequest{}, ErrRequestNotFound
		}
		return OxygenationRequest{}, err
	}

	if request.Status == RequestStatusDeleted {
		return OxygenationRequest{}, ErrRequestDeleted
	}

	return request, nil
}

func (r *Repository) ListRequestsForAPI(filter RequestListFilter) ([]RequestListItem, error) {
	query := r.db.Model(&OxygenationRequest{}).
		Preload("Creator").
		Preload("Moderator").
		Preload("Items", func(db *gorm.DB) *gorm.DB {
			return db.Order("position asc")
		}).
		Preload("Items.Service").
		Where("status NOT IN ?", []RequestStatus{RequestStatusDraft, RequestStatusDeleted}).
		Order("id desc")

	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}
	if filter.FormedFrom != nil {
		query = query.Where("formed_at >= ?", *filter.FormedFrom)
	}
	if filter.FormedTo != nil {
		query = query.Where("formed_at <= ?", *filter.FormedTo)
	}

	var requests []OxygenationRequest
	if err := query.Find(&requests).Error; err != nil {
		return nil, err
	}

	items := make([]RequestListItem, 0, len(requests))
	for _, request := range requests {
		var resultsCount int64
		if err := r.db.Model(&RequestService{}).
			Where("request_id = ? AND result_coefficient IS NOT NULL", request.ID).
			Count(&resultsCount).Error; err != nil {
			return nil, err
		}

		var moderatorLogin *string
		if request.Moderator != nil && strings.TrimSpace(request.Moderator.Login) != "" {
			login := request.Moderator.Login
			moderatorLogin = &login
		}

		items = append(items, RequestListItem{
			Request:        request,
			CreatorLogin:   request.Creator.Login,
			ModeratorLogin: moderatorLogin,
			ResultsCount:   int(resultsCount),
		})
	}

	return items, nil
}

func (r *Repository) UpdateRequestFields(userID, requestID uint, input RequestUpdateInput) (OxygenationRequest, error) {
	if err := r.db.Transaction(func(tx *gorm.DB) error {
		request, err := lockRequestByCreator(tx, userID, requestID)
		if err != nil {
			return err
		}
		if request.Status != RequestStatusDraft {
			return ErrInvalidTransition
		}

		if input.PatientName != nil {
			value := strings.TrimSpace(*input.PatientName)
			if value == "" {
				return ErrValidationFailed
			}
			request.PatientName = &value
		}

		if input.BloodValuePaO2 != nil {
			if *input.BloodValuePaO2 <= 0 {
				return ErrValidationFailed
			}
			request.BloodValuePaO2 = input.BloodValuePaO2
		}

		if input.FiO2Value != nil {
			if *input.FiO2Value <= 0 {
				return ErrValidationFailed
			}
			request.FiO2Value = input.FiO2Value
		}

		if err := tx.Save(&request).Error; err != nil {
			return err
		}

		return tx.Model(&RequestService{}).
			Where("request_id = ?", requestID).
			Update("result_coefficient", nil).Error
	}); err != nil {
		return OxygenationRequest{}, err
	}

	return r.GetRequestByIDWithRelations(userID, requestID)
}

func (r *Repository) FormDraftRequest(userID, requestID uint) (OxygenationRequest, error) {
	if err := r.db.Transaction(func(tx *gorm.DB) error {
		request, err := lockRequestByCreatorWithItems(tx, userID, requestID)
		if err != nil {
			return err
		}
		if request.Status != RequestStatusDraft {
			return ErrInvalidTransition
		}

		if request.PatientName == nil || strings.TrimSpace(*request.PatientName) == "" {
			return ErrValidationFailed
		}
		if request.BloodValuePaO2 == nil || request.FiO2Value == nil || *request.BloodValuePaO2 <= 0 || *request.FiO2Value <= 0 {
			return ErrValidationFailed
		}
		if len(request.Items) == 0 {
			return ErrValidationFailed
		}

		formedAt := time.Now().UTC()
		request.Status = RequestStatusFormed
		request.FormedAt = &formedAt
		request.CompletedAt = nil
		request.ModeratorID = nil
		if err := tx.Save(&request).Error; err != nil {
			return err
		}

		return tx.Model(&RequestService{}).
			Where("request_id = ?", requestID).
			Update("result_coefficient", nil).Error
	}); err != nil {
		return OxygenationRequest{}, err
	}

	return r.GetRequestByIDWithRelations(userID, requestID)
}

func (r *Repository) ReviewFormedRequest(requestID, moderatorID uint, action string) (OxygenationRequest, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "complete" && action != "reject" {
		return OxygenationRequest{}, ErrValidationFailed
	}

	var creatorID uint
	if err := r.db.Transaction(func(tx *gorm.DB) error {
		request, err := lockRequestByIDWithItems(tx, requestID)
		if err != nil {
			return err
		}
		if request.Status != RequestStatusFormed {
			return ErrInvalidTransition
		}

		completedAt := time.Now().UTC()
		request.CompletedAt = &completedAt
		request.ModeratorID = &moderatorID
		if action == "complete" {
			request.Status = RequestStatusCompleted
		} else {
			request.Status = RequestStatusRejected
		}

		if err := tx.Save(&request).Error; err != nil {
			return err
		}

		var resultCoefficient *float64
		if action == "complete" {
			resultCoefficient = calculateOxygenationIndex(request.BloodValuePaO2, request.FiO2Value)
		}
		if err := tx.Model(&RequestService{}).
			Where("request_id = ?", requestID).
			Update("result_coefficient", resultCoefficient).Error; err != nil {
			return err
		}

		creatorID = request.CreatorID
		return nil
	}); err != nil {
		return OxygenationRequest{}, err
	}

	return r.GetRequestByIDWithRelations(creatorID, requestID)
}

func (r *Repository) DeleteDraftRequest(userID, requestID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		request, err := lockRequestByCreator(tx, userID, requestID)
		if err != nil {
			return err
		}
		if request.Status != RequestStatusDraft {
			return ErrInvalidTransition
		}

		request.Status = RequestStatusDeleted
		request.FormedAt = nil
		request.CompletedAt = nil
		request.ModeratorID = nil
		return tx.Save(&request).Error
	})
}

func (r *Repository) UpdateRequestServiceInDraft(userID, requestID, serviceID uint, input RequestServiceUpdateInput) (RequestService, error) {
	var item RequestService
	if err := r.db.Transaction(func(tx *gorm.DB) error {
		request, err := lockRequestByCreator(tx, userID, requestID)
		if err != nil {
			return err
		}
		if request.Status != RequestStatusDraft {
			return ErrInvalidTransition
		}

		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ? AND service_id = ?", requestID, serviceID).
			Preload("Service").
			First(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRequestServiceNotFound
			}
			return err
		}

		if input.Quantity != nil {
			if *input.Quantity <= 0 {
				return ErrValidationFailed
			}
			item.Quantity = *input.Quantity
		}
		if input.Position != nil {
			if *input.Position <= 0 {
				return ErrValidationFailed
			}
			item.Position = *input.Position
		}

		if input.IsPrimary != nil {
			if *input.IsPrimary {
				if err := tx.Model(&RequestService{}).Where("request_id = ?", requestID).Update("is_primary", false).Error; err != nil {
					return err
				}
				item.IsPrimary = true
			} else {
				item.IsPrimary = false
			}
		}

		if err := tx.Save(&item).Error; err != nil {
			return err
		}

		if err := ensurePrimary(tx, requestID); err != nil {
			return err
		}

		return tx.Model(&RequestService{}).
			Where("request_id = ?", requestID).
			Update("result_coefficient", nil).Error
	}); err != nil {
		return RequestService{}, err
	}

	if err := r.db.Where("request_id = ? AND service_id = ?", requestID, serviceID).
		Preload("Service").
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return RequestService{}, ErrRequestServiceNotFound
		}
		return RequestService{}, err
	}

	return item, nil
}

func (r *Repository) RemoveRequestServiceFromDraft(userID, requestID, serviceID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		request, err := lockRequestByCreator(tx, userID, requestID)
		if err != nil {
			return err
		}
		if request.Status != RequestStatusDraft {
			return ErrInvalidTransition
		}

		result := tx.Where("request_id = ? AND service_id = ?", requestID, serviceID).Delete(&RequestService{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrRequestServiceNotFound
		}

		var count int64
		if err := tx.Model(&RequestService{}).Where("request_id = ?", requestID).Count(&count).Error; err != nil {
			return err
		}

		if count == 0 {
			return nil
		}

		if err := ensurePrimary(tx, requestID); err != nil {
			return err
		}

		return tx.Model(&RequestService{}).
			Where("request_id = ?", requestID).
			Update("result_coefficient", nil).Error
	})
}

func (r *Repository) RegisterUser(login, fullName, password string) (User, error) {
	login = strings.ToLower(strings.TrimSpace(login))
	fullName = strings.TrimSpace(fullName)
	password = strings.TrimSpace(password)
	if login == "" || fullName == "" || password == "" {
		return User{}, ErrValidationFailed
	}

	user := User{
		Login:        login,
		FullName:     fullName,
		Role:         "creator",
		PasswordHash: HashPassword(password),
		CreatedAt:    time.Now().UTC(),
	}

	if err := r.db.Create(&user).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
			return User{}, ErrValidationFailed
		}
		return User{}, err
	}

	return user, nil
}

func (r *Repository) AuthenticateUser(login, password string) (User, error) {
	login = strings.ToLower(strings.TrimSpace(login))
	password = strings.TrimSpace(password)
	if login == "" || password == "" {
		return User{}, ErrValidationFailed
	}

	var user User
	if err := r.db.Where("login = ?", login).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return User{}, ErrValidationFailed
		}
		return User{}, err
	}

	if user.PasswordHash != HashPassword(password) {
		return User{}, ErrValidationFailed
	}

	return user, nil
}

func HashPassword(password string) string {
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])
}

func normalizeNullableString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func lockRequestByCreator(tx *gorm.DB, userID, requestID uint) (OxygenationRequest, error) {
	var request OxygenationRequest
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND creator_id = ?", requestID, userID).
		First(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return OxygenationRequest{}, ErrRequestNotFound
		}
		return OxygenationRequest{}, err
	}
	if request.Status == RequestStatusDeleted {
		return OxygenationRequest{}, ErrRequestDeleted
	}
	return request, nil
}

func lockRequestByCreatorWithItems(tx *gorm.DB, userID, requestID uint) (OxygenationRequest, error) {
	var request OxygenationRequest
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND creator_id = ?", requestID, userID).
		Preload("Items", func(db *gorm.DB) *gorm.DB {
			return db.Order("position asc")
		}).
		Preload("Items.Service").
		First(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return OxygenationRequest{}, ErrRequestNotFound
		}
		return OxygenationRequest{}, err
	}
	if request.Status == RequestStatusDeleted {
		return OxygenationRequest{}, ErrRequestDeleted
	}
	return request, nil
}

func lockRequestByIDWithItems(tx *gorm.DB, requestID uint) (OxygenationRequest, error) {
	var request OxygenationRequest
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", requestID).
		Preload("Items", func(db *gorm.DB) *gorm.DB {
			return db.Order("position asc")
		}).
		Preload("Items.Service").
		First(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return OxygenationRequest{}, ErrRequestNotFound
		}
		return OxygenationRequest{}, err
	}
	if request.Status == RequestStatusDeleted {
		return OxygenationRequest{}, ErrRequestDeleted
	}
	return request, nil
}

func ensurePrimary(tx *gorm.DB, requestID uint) error {
	var count int64
	if err := tx.Model(&RequestService{}).
		Where("request_id = ? AND is_primary = TRUE", requestID).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	var first RequestService
	if err := tx.Where("request_id = ?", requestID).
		Order("position asc, service_id asc").
		First(&first).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	return tx.Model(&RequestService{}).
		Where("request_id = ? AND service_id = ?", first.RequestID, first.ServiceID).
		Update("is_primary", true).Error
}
