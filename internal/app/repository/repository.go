package repository

import (
	"fmt"
	"strings"
)

type Repository struct {
	services []Service
	requests map[int]Request
}

type Service struct {
	ID               int
	Name             string
	Benchmark        string
	ShortDescription string
	FullDescription  string
	ClinicalSigns    string
	Recommendations  string
	ImageURL         string
	VideoURL         string
}

type Request struct {
	ID             int
	PatientName    string
	BloodValuePaO2 string
	FiO2Value      string
	MMComment      string
	MMCoefficient  float64
	DiagnosisLabel string
	ServiceIDs     []int
	MMByServiceID  map[int]string
}

func NewRepository() *Repository {
	services := []Service{
		{
			ID:               1,
			Name:             "Нормальная оксигенация",
			Benchmark:        "PaO2/FiO2 > 300 мм рт.ст.",
			ShortDescription: "Эталон услуги для нормального газообмена.",
			FullDescription:  "Показатели соответствуют норме. Клинически значимой дыхательной недостаточности нет.",
			ClinicalSigns:    "Одышки нет; SpO2 обычно > 95%; стабильные газы крови.",
			Recommendations:  "Наблюдение в динамике; контроль сатурации и общего состояния.",
			ImageURL:         "http://localhost:9000/images/normal.png",
			VideoURL:         "http://localhost:9000/images/normal.mp4",
		},
		{
			ID:               2,
			Name:             "Легкая ДН",
			Benchmark:        "PaO2/FiO2 201-300 мм рт.ст.",
			ShortDescription: "Эталон услуги для легкой дыхательной недостаточности.",
			FullDescription:  "Умеренно сниженная оксигенация с риском прогрессирования при неблагоприятной динамике.",
			ClinicalSigns:    "Одышка при нагрузке; SpO2 90-94%; умеренная тахипноэ.",
			Recommendations:  "Кислородотерапия по показаниям; контроль газов крови в динамике.",
			ImageURL:         "http://localhost:9000/images/mild.png",
			VideoURL:         "http://localhost:9000/images/mild.mp4",
		},
		{
			ID:               3,
			Name:             "Умеренная ДН (ОРДС)",
			Benchmark:        "PaO2/FiO2 101-200 мм рт.ст.",
			ShortDescription: "Эталон услуги для умеренной степени ОРДС.",
			FullDescription:  "Значимое нарушение газообмена, требующее активного наблюдения и коррекции респираторной поддержки.",
			ClinicalSigns:    "Одышка в покое; SpO2 85-89%; тахипноэ более 20 в минуту.",
			Recommendations:  "Мониторинг газов крови каждые 4-6 часов; оценка необходимости NIV/ИВЛ.",
			ImageURL:         "http://localhost:9000/images/moderate.png",
			VideoURL:         "http://localhost:9000/images/moderate.mp4",
		},
		{
			ID:               4,
			Name:             "Тяжелая ДН (ОРДС)",
			Benchmark:        "PaO2/FiO2 <= 100 мм рт.ст.",
			ShortDescription: "Эталон услуги для тяжелой дыхательной недостаточности.",
			FullDescription:  "Критическая гипоксемия, требующая интенсивной терапии и респираторной поддержки.",
			ClinicalSigns:    "Выраженная дыхательная недостаточность; SpO2 < 85%; признаки истощения дыхания.",
			Recommendations:  "Интенсивная терапия; инвазивная вентиляция по показаниям; круглосуточный мониторинг.",
			ImageURL:         "http://localhost:9000/images/severe.png",
			VideoURL:         "http://localhost:9000/images/severe.mp4",
		},
	}

	requests := map[int]Request{
		101: {
			ID:             101,
			PatientName:    "Иванов И.И.",
			BloodValuePaO2: "85.5 мм рт.ст.",
			FiO2Value:      "0.60 (60%)",
			MMComment:      "Состояние средней тяжести. Рекомендован повторный контроль коэффициента через 6 часов.",
			MMCoefficient:  142.5,
			DiagnosisLabel: "Умеренная ДН (ОРДС)",
			ServiceIDs:     []int{1, 2, 3, 4},
			MMByServiceID: map[int]string{
				1: "-",
				2: "-",
				3: "Основной диагноз",
				4: "-",
			},
		},
	}

	return &Repository{
		services: services,
		requests: requests,
	}
}

func (r *Repository) GetServices() []Service {
	services := make([]Service, len(r.services))
	copy(services, r.services)
	return services
}

func (r *Repository) GetServicesByQuery(query string) []Service {
	if strings.TrimSpace(query) == "" {
		return r.GetServices()
	}

	queryLower := strings.ToLower(strings.TrimSpace(query))
	result := make([]Service, 0)

	for _, service := range r.services {
		if strings.Contains(strings.ToLower(service.Name), queryLower) ||
			strings.Contains(strings.ToLower(service.Benchmark), queryLower) {
			result = append(result, service)
		}
	}

	return result
}

func (r *Repository) GetServiceByID(id int) (Service, error) {
	for _, service := range r.services {
		if service.ID == id {
			return service, nil
		}
	}

	return Service{}, fmt.Errorf("service with id=%d not found", id)
}

func (r *Repository) GetRequestByID(id int) (Request, error) {
	request, ok := r.requests[id]
	if !ok {
		return Request{}, fmt.Errorf("request with id=%d not found", id)
	}

	return request, nil
}
