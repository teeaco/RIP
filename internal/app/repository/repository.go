package repository

import (
	"errors"
	"math"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	DefaultCreatorUserID   uint = 1
	DefaultModeratorUserID uint = 2
)

type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

func (c Config) DSN() string {
	return "host=" + c.Host +
		" port=" + c.Port +
		" user=" + c.User +
		" password=" + c.Password +
		" dbname=" + c.DBName +
		" sslmode=" + c.SSLMode +
		" TimeZone=UTC"
}

type RequestStatus string

const (
	RequestStatusDraft     RequestStatus = "draft"
	RequestStatusDeleted   RequestStatus = "deleted"
	RequestStatusFormed    RequestStatus = "formed"
	RequestStatusCompleted RequestStatus = "completed"
	RequestStatusRejected  RequestStatus = "rejected"
)

type ServiceStatus string

const (
	ServiceStatusActive  ServiceStatus = "active"
	ServiceStatusDeleted ServiceStatus = "deleted"
)

var (
	ErrDraftNotFound   = errors.New("draft request not found")
	ErrRequestNotFound = errors.New("request not found")
	ErrRequestDeleted  = errors.New("request deleted")
	ErrServiceNotFound = errors.New("service not found")
)

type Repository struct {
	db *gorm.DB
}

type User struct {
	ID        uint      `gorm:"primaryKey"`
	Login     string    `gorm:"size:64;not null;uniqueIndex"`
	FullName  string    `gorm:"size:120;not null"`
	Role      string    `gorm:"size:32;not null"`
	CreatedAt time.Time `gorm:"not null"`
}

func (User) TableName() string {
	return "app_users"
}

type OxygenationService struct {
	ID              uint          `gorm:"primaryKey"`
	Name            string        `gorm:"size:140;not null"`
	Description     string        `gorm:"type:text;not null"`
	Status          ServiceStatus `gorm:"type:varchar(16);not null;default:'active';index"`
	ImageURL        *string       `gorm:"type:text"`
	VideoURL        *string       `gorm:"type:text"`
	Benchmark       string        `gorm:"size:120;not null"`
	ClinicalSigns   string        `gorm:"type:text"`
	Recommendations string        `gorm:"type:text"`
	CreatedAt       time.Time     `gorm:"not null"`
}

func (OxygenationService) TableName() string {
	return "oxygenation_services"
}

type OxygenationRequest struct {
	ID             uint          `gorm:"primaryKey"`
	Status         RequestStatus `gorm:"type:varchar(16);not null;index"`
	CreatedAt      time.Time     `gorm:"not null"`
	CreatorID      uint          `gorm:"not null;index"`
	Creator        User          `gorm:"foreignKey:CreatorID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT;"`
	FormedAt       *time.Time
	CompletedAt    *time.Time
	ModeratorID    *uint
	Moderator      *User   `gorm:"foreignKey:ModeratorID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT;"`
	PatientName    *string `gorm:"size:120"`
	BloodValuePaO2 *float64
	FiO2Value      *float64
	MMCoefficient  *float64
	DiagnosisLabel *string          `gorm:"size:140"`
	MMComment      *string          `gorm:"type:text"`
	Items          []RequestService `gorm:"foreignKey:RequestID"`
}

func (OxygenationRequest) TableName() string {
	return "oxygenation_requests"
}

type RequestService struct {
	ID            uint               `gorm:"primaryKey"`
	RequestID     uint               `gorm:"not null;index;uniqueIndex:ux_request_service_unique"`
	ServiceID     uint               `gorm:"not null;index;uniqueIndex:ux_request_service_unique"`
	Quantity      int                `gorm:"not null;default:1"`
	Position      int                `gorm:"not null;default:1"`
	IsPrimary     bool               `gorm:"not null;default:false"`
	DoctorComment *string            `gorm:"type:text"`
	CreatedAt     time.Time          `gorm:"not null"`
	Request       OxygenationRequest `gorm:"foreignKey:RequestID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT;"`
	Service       OxygenationService `gorm:"foreignKey:ServiceID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT;"`
}

func (RequestService) TableName() string {
	return "oxygenation_request_services"
}

type DraftCart struct {
	RequestID  uint
	ItemsCount int
	HasDraft   bool
}

func NewRepository(cfg Config) (*Repository, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	repo := &Repository{db: db}
	if err := repo.migrate(); err != nil {
		return nil, err
	}
	if err := repo.seed(); err != nil {
		return nil, err
	}

	return repo, nil
}

func (r *Repository) migrate() error {
	if err := r.db.AutoMigrate(
		&User{},
		&OxygenationService{},
		&OxygenationRequest{},
		&RequestService{},
	); err != nil {
		return err
	}

	if err := r.db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS ux_single_draft_request
		ON oxygenation_requests (creator_id)
		WHERE status = 'draft'
	`).Error; err != nil {
		return err
	}

	if err := r.db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS ux_request_service_unique
		ON oxygenation_request_services (request_id, service_id)
	`).Error; err != nil {
		return err
	}

	return nil
}

func (r *Repository) seed() error {
	now := time.Now().UTC()

	users := []User{
		{
			ID:        DefaultCreatorUserID,
			Login:     "creator",
			FullName:  "Иванов И.И.",
			Role:      "creator",
			CreatedAt: now,
		},
		{
			ID:        DefaultModeratorUserID,
			Login:     "moderator",
			FullName:  "Петров П.П.",
			Role:      "moderator",
			CreatedAt: now,
		},
	}

	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&users).Error; err != nil {
		return err
	}

	var servicesCount int64
	if err := r.db.Model(&OxygenationService{}).Count(&servicesCount).Error; err != nil {
		return err
	}
	if servicesCount > 0 {
		return nil
	}

	services := []OxygenationService{
		{
			Name:            "Нормальная оксигенация",
			Description:     "Эталон степени для нормального газообмена.",
			Status:          ServiceStatusActive,
			ImageURL:        strPtr("http://localhost:9000/images/normal.png"),
			VideoURL:        strPtr("http://localhost:9000/images/normal.mp4"),
			Benchmark:       "PaO2/FiO2 > 300 мм рт.ст.",
			ClinicalSigns:   "Одышки нет; SpO2 обычно > 95%; стабильные газы крови.",
			Recommendations: "Наблюдение в динамике; контроль сатурации и общего состояния.",
			CreatedAt:       now,
		},
		{
			Name:            "Легкая ДН",
			Description:     "Эталон степени для легкой дыхательной недостаточности.",
			Status:          ServiceStatusActive,
			ImageURL:        strPtr("http://localhost:9000/images/mild.png"),
			VideoURL:        strPtr("http://localhost:9000/images/mild.mp4"),
			Benchmark:       "PaO2/FiO2 201-300 мм рт.ст.",
			ClinicalSigns:   "Одышка при нагрузке; SpO2 90-94%; умеренная тахипноэ.",
			Recommendations: "Кислородотерапия по показаниям; контроль газов крови в динамике.",
			CreatedAt:       now,
		},
		{
			Name:            "Умеренная ДН (ОРДС)",
			Description:     "Эталон степени для умеренной дыхательной недостаточности.",
			Status:          ServiceStatusActive,
			ImageURL:        strPtr("http://localhost:9000/images/moderate.png"),
			VideoURL:        strPtr("http://localhost:9000/images/moderate.mp4"),
			Benchmark:       "PaO2/FiO2 101-200 мм рт.ст.",
			ClinicalSigns:   "Одышка в покое; SpO2 85-89%; тахипноэ более 20 в минуту.",
			Recommendations: "Мониторинг газов крови каждые 4-6 часов; оценка необходимости NIV/ИВЛ.",
			CreatedAt:       now,
		},
		{
			Name:            "Тяжелая ДН (ОРДС)",
			Description:     "Эталон степени для тяжелой дыхательной недостаточности.",
			Status:          ServiceStatusActive,
			ImageURL:        strPtr("http://localhost:9000/images/severe.png"),
			VideoURL:        strPtr("http://localhost:9000/images/severe.mp4"),
			Benchmark:       "PaO2/FiO2 <= 100 мм рт.ст.",
			ClinicalSigns:   "Выраженная дыхательная недостаточность; SpO2 < 85%; признаки истощения дыхания.",
			Recommendations: "Интенсивная терапия; инвазивная вентиляция по показаниям; круглосуточный мониторинг.",
			CreatedAt:       now,
		},
	}

	return r.db.Create(&services).Error
}

func (r *Repository) GetServicesByQuery(query string) ([]OxygenationService, error) {
	dbQuery := r.db.Model(&OxygenationService{}).
		Where("status = ?", ServiceStatusActive).
		Order("id asc")

	trimmed := strings.TrimSpace(query)
	if trimmed != "" {
		pattern := "%" + trimmed + "%"
		dbQuery = dbQuery.Where(
			"name ILIKE ? OR benchmark ILIKE ? OR description ILIKE ?",
			pattern,
			pattern,
			pattern,
		)
	}

	var services []OxygenationService
	if err := dbQuery.Find(&services).Error; err != nil {
		return nil, err
	}

	return services, nil
}

func (r *Repository) GetServiceByID(id uint) (OxygenationService, error) {
	var service OxygenationService
	if err := r.db.Where("id = ? AND status = ?", id, ServiceStatusActive).First(&service).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return OxygenationService{}, ErrServiceNotFound
		}
		return OxygenationService{}, err
	}

	return service, nil
}

func (r *Repository) GetDraftCart(userID uint) (DraftCart, error) {
	var request OxygenationRequest
	if err := r.db.Select("id").Where("creator_id = ? AND status = ?", userID, RequestStatusDraft).First(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return DraftCart{}, nil
		}
		return DraftCart{}, err
	}

	var itemsCount int64
	if err := r.db.Model(&RequestService{}).
		Where("request_id = ?", request.ID).
		Select("COALESCE(SUM(quantity), 0)").
		Scan(&itemsCount).Error; err != nil {
		return DraftCart{}, err
	}

	return DraftCart{
		RequestID:  request.ID,
		ItemsCount: int(itemsCount),
		HasDraft:   true,
	}, nil
}

func (r *Repository) AddServiceToDraft(userID, serviceID uint) (uint, error) {
	var requestID uint

	err := r.db.Transaction(func(tx *gorm.DB) error {
		var service OxygenationService
		if err := tx.Where("id = ? AND status = ?", serviceID, ServiceStatusActive).First(&service).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrServiceNotFound
			}
			return err
		}

		var request OxygenationRequest
		err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("creator_id = ? AND status = ?", userID, RequestStatusDraft).
			First(&request).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			request = newDraftRequest(userID, service.Name)
			if err := tx.Create(&request).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		if err := tx.Model(&RequestService{}).Where("request_id = ?", request.ID).Update("is_primary", false).Error; err != nil {
			return err
		}

		var item RequestService
		err = tx.Where("request_id = ? AND service_id = ?", request.ID, serviceID).First(&item).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			var maxPosition int64
			if err := tx.Model(&RequestService{}).
				Where("request_id = ?", request.ID).
				Select("COALESCE(MAX(position), 0)").
				Scan(&maxPosition).Error; err != nil {
				return err
			}

			item = RequestService{
				RequestID:     request.ID,
				ServiceID:     serviceID,
				Quantity:      1,
				Position:      int(maxPosition) + 1,
				IsPrimary:     true,
				DoctorComment: strPtr("По мнению врача: степень добавлена в черновик."),
				CreatedAt:     time.Now().UTC(),
			}
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			item.Quantity++
			item.IsPrimary = true
			if item.DoctorComment == nil || strings.TrimSpace(*item.DoctorComment) == "" {
				item.DoctorComment = strPtr("По мнению врача: количество степени изменено.")
			}
			if err := tx.Save(&item).Error; err != nil {
				return err
			}
		}

		request.DiagnosisLabel = strPtr(service.Name)
		request.MMComment = strPtr("По мнению врача: текущая степень обновлена.")
		request.MMCoefficient = calculateOxygenationIndex(request.BloodValuePaO2, request.FiO2Value)
		if err := tx.Save(&request).Error; err != nil {
			return err
		}

		requestID = request.ID
		return nil
	})

	return requestID, err
}

func (r *Repository) GetRequestByID(userID, requestID uint) (OxygenationRequest, error) {
	var request OxygenationRequest
	if err := r.db.
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

	request.MMCoefficient = calculateOxygenationIndex(request.BloodValuePaO2, request.FiO2Value)
	return request, nil
}

func (r *Repository) SoftDeleteDraftBySQL(userID, requestID uint) error {
	result := r.db.Exec(`
		UPDATE oxygenation_requests
		SET status = ?, formed_at = NULL, completed_at = NULL, moderator_id = NULL
		WHERE id = ? AND creator_id = ? AND status = ?
	`, RequestStatusDeleted, requestID, userID, RequestStatusDraft)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrRequestNotFound
	}

	return nil
}

func newDraftRequest(userID uint, diagnosis string) OxygenationRequest {
	now := time.Now().UTC()
	patientName := "Иванов И.И."
	bloodValue := 85.5
	fio2Value := 0.60

	request := OxygenationRequest{
		Status:         RequestStatusDraft,
		CreatedAt:      now,
		CreatorID:      userID,
		PatientName:    &patientName,
		BloodValuePaO2: &bloodValue,
		FiO2Value:      &fio2Value,
		DiagnosisLabel: &diagnosis,
		MMComment:      strPtr("По мнению врача: черновик создан автоматически."),
	}
	request.MMCoefficient = calculateOxygenationIndex(request.BloodValuePaO2, request.FiO2Value)

	return request
}

func calculateOxygenationIndex(paO2, fiO2 *float64) *float64 {
	if paO2 == nil || fiO2 == nil || *fiO2 <= 0 {
		return nil
	}

	result, ok := CalculateOxygenationIndex(*paO2, *fiO2)
	if !ok {
		return nil
	}

	return &result
}

func CalculateOxygenationIndex(paO2, fiO2 float64) (float64, bool) {
	if fiO2 <= 0 {
		return 0, false
	}

	result := math.Round((paO2 / fiO2) * 10)
	return result / 10, true
}

func DiagnosisByOxygenationIndex(index float64) string {
	switch {
	case index > 300:
		return "Нормальная оксигенация"
	case index > 200:
		return "Легкая ДН"
	case index > 100:
		return "Умеренная ДН (ОРДС)"
	default:
		return "Тяжелая ДН (ОРДС)"
	}
}

func strPtr(value string) *string {
	return &value
}
