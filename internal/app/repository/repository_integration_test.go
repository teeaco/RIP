package repository

import (
	"errors"
	"math"
	"os"
	"testing"
	"time"
)

func TestSchemaLegacyColumnsRemoved(t *testing.T) {
	repo := newIntegrationRepository(t)

	legacyColumns := map[string][]string{
		"oxygenation_services": {
			"clinical_signs",
			"recommendations",
		},
		"oxygenation_requests": {
			"mm_coefficient",
			"diagnosis_label",
			"mm_comment",
		},
		"oxygenation_request_services": {
			"id",
			"created_at",
			"quantity",
			"position",
			"is_primary",
		},
	}

	for tableName, columns := range legacyColumns {
		for _, columnName := range columns {
			var count int64
			err := repo.db.Raw(`
				SELECT COUNT(*)
				FROM information_schema.columns
				WHERE table_schema = 'public'
					AND table_name = ?
					AND column_name = ?
			`, tableName, columnName).Scan(&count).Error
			if err != nil {
				t.Fatalf("failed to inspect column %s.%s: %v", tableName, columnName, err)
			}
			if count != 0 {
				t.Fatalf("legacy column still exists: %s.%s", tableName, columnName)
			}
		}
	}

	var primaryKeyColumns []string
	err := repo.db.Raw(`
		SELECT a.attname
		FROM pg_index i
		JOIN pg_class c ON c.oid = i.indrelid
		JOIN LATERAL unnest(i.indkey) WITH ORDINALITY AS k(attnum, ord) ON TRUE
		JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = k.attnum
		WHERE c.relname = 'oxygenation_request_services'
			AND i.indisprimary
		ORDER BY k.ord
	`).Scan(&primaryKeyColumns).Error
	if err != nil {
		t.Fatalf("failed to inspect primary key: %v", err)
	}

	if len(primaryKeyColumns) != 2 ||
		primaryKeyColumns[0] != "request_id" ||
		primaryKeyColumns[1] != "service_id" {
		t.Fatalf("unexpected primary key for oxygenation_request_services: %v", primaryKeyColumns)
	}

	var doctorCommentCount int64
	err = repo.db.Raw(`
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = 'public'
			AND table_name = 'oxygenation_request_services'
			AND column_name = 'doctor_comment'
	`).Scan(&doctorCommentCount).Error
	if err != nil {
		t.Fatalf("failed to inspect doctor_comment column: %v", err)
	}
	if doctorCommentCount != 1 {
		t.Fatalf("expected oxygenation_request_services.doctor_comment to exist")
	}
}

func TestRepositoryAddDeleteAndSearchFlow(t *testing.T) {
	repo := newIntegrationRepository(t)

	userID := uint(91001)
	prepareTestUser(t, repo, userID)
	defer cleanupTestUserData(t, repo, userID)

	servicesByBench, err := repo.GetServicesByQuery("PaO2/FiO2")
	if err != nil {
		t.Fatalf("search by benchmark failed: %v", err)
	}
	if len(servicesByBench) == 0 {
		t.Fatalf("search by benchmark returned no rows")
	}

	noMatchServices, err := repo.GetServicesByQuery("__unlikely_query__")
	if err != nil {
		t.Fatalf("search with no-match query failed: %v", err)
	}
	if len(noMatchServices) != 0 {
		t.Fatalf("search with no-match query returned rows: %d", len(noMatchServices))
	}

	serviceID := servicesByBench[0].ID
	requestID, err := repo.AddServiceToDraft(userID, serviceID)
	if err != nil {
		t.Fatalf("first add service to draft failed: %v", err)
	}

	sameRequestID, err := repo.AddServiceToDraft(userID, serviceID)
	if err != nil {
		t.Fatalf("second add service to draft failed: %v", err)
	}
	if sameRequestID != requestID {
		t.Fatalf("second add should update same draft request, got %d and %d", requestID, sameRequestID)
	}

	request, err := repo.GetRequestByID(userID, requestID)
	if err != nil {
		t.Fatalf("get request after add failed: %v", err)
	}
	if request.Status != RequestStatusDraft {
		t.Fatalf("unexpected request status: %s", request.Status)
	}
	if len(request.Items) != 1 {
		t.Fatalf("expected 1 request item, got %d", len(request.Items))
	}

	expectedCoefficient := calculateOxygenationIndex(request.BloodValuePaO2, request.FiO2Value)
	actualCoefficient := request.Items[0].ResultCoefficient
	if expectedCoefficient == nil && actualCoefficient != nil {
		t.Fatalf("expected NULL result_coefficient, got %v", *actualCoefficient)
	}
	if expectedCoefficient != nil && actualCoefficient == nil {
		t.Fatalf("expected non-NULL result_coefficient")
	}
	if expectedCoefficient != nil && actualCoefficient != nil &&
		math.Abs(*expectedCoefficient-*actualCoefficient) > 0.000001 {
		t.Fatalf("unexpected result_coefficient: expected %v, got %v", *expectedCoefficient, *actualCoefficient)
	}

	if err := repo.SoftDeleteDraftBySQL(userID, requestID); err != nil {
		t.Fatalf("soft delete draft failed: %v", err)
	}

	_, err = repo.GetRequestByID(userID, requestID)
	if !errors.Is(err, ErrRequestDeleted) {
		t.Fatalf("expected ErrRequestDeleted after delete, got: %v", err)
	}

	newRequestID, err := repo.AddServiceToDraft(userID, serviceID)
	if err != nil {
		t.Fatalf("add after delete failed: %v", err)
	}
	if newRequestID == requestID {
		t.Fatalf("expected new draft id after deleting previous draft")
	}

	var draftCount int64
	if err := repo.db.Model(&OxygenationRequest{}).
		Where("creator_id = ? AND status = ?", userID, RequestStatusDraft).
		Count(&draftCount).Error; err != nil {
		t.Fatalf("failed to count draft requests: %v", err)
	}
	if draftCount != 1 {
		t.Fatalf("expected exactly one draft request, got %d", draftCount)
	}
}

func newIntegrationRepository(t *testing.T) *Repository {
	t.Helper()

	cfg := Config{
		Host:     envOrDefaultTest("DB_HOST", "127.0.0.1"),
		Port:     envOrDefaultTest("DB_PORT", "55632"),
		User:     envOrDefaultTest("DB_USER", "root"),
		Password: envOrDefaultTest("DB_PASSWORD", "root"),
		DBName:   envOrDefaultTest("DB_NAME", "RIP"),
		SSLMode:  envOrDefaultTest("DB_SSLMODE", "disable"),
	}

	repo, err := NewRepository(cfg)
	if err != nil {
		t.Skipf("integration db unavailable: %v", err)
	}

	return repo
}

func prepareTestUser(t *testing.T, repo *Repository, userID uint) {
	t.Helper()

	cleanupTestUserData(t, repo, userID)

	login := "integration_creator_91001"
	now := time.Now().UTC()
	err := repo.db.Exec(`
		INSERT INTO app_users (id, login, full_name, role, password_hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, userID, login, "Integration Creator", "creator", "integration", now).Error
	if err != nil {
		t.Fatalf("failed to create integration user: %v", err)
	}
}

func cleanupTestUserData(t *testing.T, repo *Repository, userID uint) {
	t.Helper()

	queries := []string{
		`DELETE FROM oxygenation_request_services
			WHERE request_id IN (
				SELECT id FROM oxygenation_requests WHERE creator_id = ?
			)`,
		`DELETE FROM oxygenation_requests WHERE creator_id = ?`,
		`DELETE FROM app_users WHERE id = ?`,
	}

	for _, query := range queries {
		if err := repo.db.Exec(query, userID).Error; err != nil {
			t.Fatalf("cleanup failed for user %d: %v", userID, err)
		}
	}
}

func envOrDefaultTest(name, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}
