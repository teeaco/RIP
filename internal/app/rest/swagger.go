package rest

import (
	"encoding/json"
	"net/http"
)

func (h *Handler) SwaggerUI(w http.ResponseWriter, r *http.Request) {
	const page = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>RIP API Swagger</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({
      url: '/api/swagger/openapi.json',
      dom_id: '#swagger-ui'
    });
  </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(page))
}

func (h *Handler) SwaggerOpenAPI(w http.ResponseWriter, r *http.Request) {
	spec := map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "RIP Oxygenation API",
			"version":     "4.0.0",
			"description": "Lab 4 API: JWT auth + Redis session + role-based access",
		},
		"servers": []map[string]any{
			{"url": "http://localhost:8095"},
		},
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"bearerAuth": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
			},
			"schemas": map[string]any{
				"ErrorResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"error": map[string]any{"type": "string"},
					},
					"required": []string{"error"},
				},
				"RegisterUserRequest": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"login":     map[string]any{"type": "string"},
						"full_name": map[string]any{"type": "string"},
						"password":  map[string]any{"type": "string"},
					},
					"required": []string{"login", "full_name", "password"},
				},
				"LoginRequest": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"login":    map[string]any{"type": "string"},
						"password": map[string]any{"type": "string"},
					},
					"required": []string{"login", "password"},
				},
				"LoginResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"token_type": map[string]any{"type": "string", "example": "Bearer"},
						"token":      map[string]any{"type": "string"},
						"expires_at": map[string]any{"type": "string", "format": "date-time"},
						"user": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"id":         map[string]any{"type": "integer"},
								"login":      map[string]any{"type": "string"},
								"full_name":  map[string]any{"type": "string"},
								"role":       map[string]any{"type": "string"},
								"session_id": map[string]any{"type": "string"},
							},
						},
					},
				},
				"AddServiceToDraftRequest": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"service_id": map[string]any{"type": "integer"},
					},
					"required": []string{"service_id"},
				},
				"UpdateRequestServiceRequest": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"doctor_comment": map[string]any{"type": "string"},
					},
				},
				"UpdateRequestRequest": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"patient_name":     map[string]any{"type": "string"},
						"blood_value_pao2": map[string]any{"type": "number"},
						"fio2_value":       map[string]any{"type": "number"},
					},
				},
				"ReviewRequestRequest": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type": "string",
							"enum": []string{"complete", "reject"},
						},
					},
					"required": []string{"action"},
				},
				"ServiceResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":          map[string]any{"type": "integer"},
						"name":        map[string]any{"type": "string"},
						"description": map[string]any{"type": "string"},
						"status":      map[string]any{"type": "string"},
						"image_url":   map[string]any{"type": "string"},
						"video_url":   map[string]any{"type": "string"},
						"benchmark":   map[string]any{"type": "string"},
						"created_at":  map[string]any{"type": "string", "format": "date-time"},
					},
				},
				"RequestServiceResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"request_id":         map[string]any{"type": "integer"},
						"service_id":         map[string]any{"type": "integer"},
						"service_name":       map[string]any{"type": "string"},
						"image_url":          map[string]any{"type": "string"},
						"video_url":          map[string]any{"type": "string"},
						"benchmark":          map[string]any{"type": "string"},
						"doctor_comment":     map[string]any{"type": "string"},
						"result_coefficient": map[string]any{"type": "number"},
					},
				},
				"RequestResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":                 map[string]any{"type": "integer"},
						"created_at":         map[string]any{"type": "string", "format": "date-time"},
						"formed_at":          map[string]any{"type": "string", "format": "date-time"},
						"completed_at":       map[string]any{"type": "string", "format": "date-time"},
						"creator_login":      map[string]any{"type": "string"},
						"moderator_login":    map[string]any{"type": "string"},
						"patient_name":       map[string]any{"type": "string"},
						"blood_value_pao2":   map[string]any{"type": "number"},
						"fio2_value":         map[string]any{"type": "number"},
						"primary_service":    map[string]any{"type": "string"},
						"result_coefficient": map[string]any{"type": "number"},
						"result":             map[string]any{"type": "string"},
						"results_count":      map[string]any{"type": "integer"},
						"items": map[string]any{
							"type": "array",
							"items": map[string]any{
								"$ref": "#/components/schemas/RequestServiceResponse",
							},
						},
					},
				},
			},
		},
		"paths": map[string]any{
			"/api/users/register": map[string]any{
				"post": map[string]any{
					"summary": "Register creator user",
					"requestBody": map[string]any{
						"required": true,
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{"$ref": "#/components/schemas/RegisterUserRequest"},
							},
						},
					},
					"responses": map[string]any{
						"201": map[string]any{"description": "Created"},
						"400": map[string]any{"description": "Validation failed"},
					},
				},
			},
			"/api/users/login": map[string]any{
				"post": map[string]any{
					"summary": "Authenticate user and return JWT",
					"requestBody": map[string]any{
						"required": true,
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{"$ref": "#/components/schemas/LoginRequest"},
							},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{
							"description": "OK",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{"$ref": "#/components/schemas/LoginResponse"},
								},
							},
						},
						"400": map[string]any{"description": "Validation failed"},
						"401": map[string]any{"description": "Unauthorized"},
					},
				},
			},
			"/api/users/logout": map[string]any{
				"post": map[string]any{
					"summary":  "Logout user and drop Redis session",
					"security": []map[string]any{{"bearerAuth": []string{}}},
					"responses": map[string]any{
						"200": map[string]any{"description": "OK"},
						"401": map[string]any{"description": "Unauthorized"},
					},
				},
			},
			"/api/services": map[string]any{
				"get": map[string]any{
					"summary": "List services",
					"parameters": []map[string]any{
						{
							"name":     "query",
							"in":       "query",
							"required": false,
							"schema":   map[string]any{"type": "string"},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{"description": "OK"},
					},
				},
				"post": map[string]any{
					"summary":  "Create service (multipart: name, description, benchmark, image?, video?)",
					"security": []map[string]any{{"bearerAuth": []string{}}},
					"requestBody": map[string]any{
						"required": true,
						"content": map[string]any{
							"multipart/form-data": map[string]any{
								"schema": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"name":        map[string]any{"type": "string"},
										"description": map[string]any{"type": "string"},
										"benchmark":   map[string]any{"type": "string"},
										"image":       map[string]any{"type": "string", "format": "binary"},
										"video":       map[string]any{"type": "string", "format": "binary"},
									},
									"required": []string{"name", "description", "benchmark"},
								},
							},
						},
					},
					"responses": map[string]any{
						"201": map[string]any{"description": "Created"},
						"401": map[string]any{"description": "Unauthorized"},
					},
				},
			},
			"/api/services/{id}": map[string]any{
				"get": map[string]any{
					"summary": "Get service by id",
					"parameters": []map[string]any{
						{
							"name":     "id",
							"in":       "path",
							"required": true,
							"schema":   map[string]any{"type": "integer"},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{"description": "OK"},
						"404": map[string]any{"description": "Not found"},
					},
				},
			},
			"/api/request-services": map[string]any{
				"post": map[string]any{
					"summary":  "Add service to creator draft",
					"security": []map[string]any{{"bearerAuth": []string{}}},
					"requestBody": map[string]any{
						"required": true,
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{"$ref": "#/components/schemas/AddServiceToDraftRequest"},
							},
						},
					},
					"responses": map[string]any{
						"201": map[string]any{"description": "Created"},
						"401": map[string]any{"description": "Unauthorized"},
					},
				},
			},
			"/api/request-services/{requestID}/{serviceID}": map[string]any{
				"put": map[string]any{
					"summary":  "Update request-service m-m fields",
					"security": []map[string]any{{"bearerAuth": []string{}}},
					"parameters": []map[string]any{
						{
							"name":     "requestID",
							"in":       "path",
							"required": true,
							"schema":   map[string]any{"type": "integer"},
						},
						{
							"name":     "serviceID",
							"in":       "path",
							"required": true,
							"schema":   map[string]any{"type": "integer"},
						},
					},
					"requestBody": map[string]any{
						"required": true,
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{"$ref": "#/components/schemas/UpdateRequestServiceRequest"},
							},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{"description": "OK"},
						"401": map[string]any{"description": "Unauthorized"},
						"409": map[string]any{"description": "Invalid transition"},
					},
				},
				"delete": map[string]any{
					"summary":  "Delete service from draft request",
					"security": []map[string]any{{"bearerAuth": []string{}}},
					"parameters": []map[string]any{
						{
							"name":     "requestID",
							"in":       "path",
							"required": true,
							"schema":   map[string]any{"type": "integer"},
						},
						{
							"name":     "serviceID",
							"in":       "path",
							"required": true,
							"schema":   map[string]any{"type": "integer"},
						},
					},
					"responses": map[string]any{
						"204": map[string]any{"description": "No Content"},
						"401": map[string]any{"description": "Unauthorized"},
					},
				},
			},
			"/api/oxygenation_request/cart": map[string]any{
				"get": map[string]any{
					"summary":  "Get draft cart icon payload",
					"security": []map[string]any{{"bearerAuth": []string{}}},
					"responses": map[string]any{
						"200": map[string]any{"description": "OK"},
						"401": map[string]any{"description": "Unauthorized"},
					},
				},
			},
			"/api/oxygenation_request": map[string]any{
				"get": map[string]any{
					"summary":  "List requests (creator sees own, moderator sees all)",
					"security": []map[string]any{{"bearerAuth": []string{}}},
					"parameters": []map[string]any{
						{
							"name":     "status",
							"in":       "query",
							"required": false,
							"schema": map[string]any{
								"type": "string",
								"enum": []string{"formed", "completed", "rejected"},
							},
						},
						{
							"name":     "formed_from",
							"in":       "query",
							"required": false,
							"schema":   map[string]any{"type": "string", "example": "2026-01-01"},
						},
						{
							"name":     "formed_to",
							"in":       "query",
							"required": false,
							"schema":   map[string]any{"type": "string", "example": "2026-12-31"},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{"description": "OK"},
						"401": map[string]any{"description": "Unauthorized"},
					},
				},
			},
			"/api/oxygenation_request/{id}": map[string]any{
				"get": map[string]any{
					"summary":  "Get request by id",
					"security": []map[string]any{{"bearerAuth": []string{}}},
					"parameters": []map[string]any{
						{
							"name":     "id",
							"in":       "path",
							"required": true,
							"schema":   map[string]any{"type": "integer"},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{"description": "OK"},
						"401": map[string]any{"description": "Unauthorized"},
						"404": map[string]any{"description": "Not found"},
					},
				},
				"put": map[string]any{
					"summary":  "Update draft request fields",
					"security": []map[string]any{{"bearerAuth": []string{}}},
					"parameters": []map[string]any{
						{
							"name":     "id",
							"in":       "path",
							"required": true,
							"schema":   map[string]any{"type": "integer"},
						},
					},
					"requestBody": map[string]any{
						"required": true,
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{"$ref": "#/components/schemas/UpdateRequestRequest"},
							},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{"description": "OK"},
						"401": map[string]any{"description": "Unauthorized"},
						"409": map[string]any{"description": "Invalid transition"},
					},
				},
				"delete": map[string]any{
					"summary":  "Delete draft request",
					"security": []map[string]any{{"bearerAuth": []string{}}},
					"parameters": []map[string]any{
						{
							"name":     "id",
							"in":       "path",
							"required": true,
							"schema":   map[string]any{"type": "integer"},
						},
					},
					"responses": map[string]any{
						"204": map[string]any{"description": "No Content"},
						"401": map[string]any{"description": "Unauthorized"},
						"409": map[string]any{"description": "Invalid transition"},
					},
				},
			},
			"/api/oxygenation_request/{id}/form": map[string]any{
				"put": map[string]any{
					"summary":  "Form draft request",
					"security": []map[string]any{{"bearerAuth": []string{}}},
					"parameters": []map[string]any{
						{
							"name":     "id",
							"in":       "path",
							"required": true,
							"schema":   map[string]any{"type": "integer"},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{"description": "OK"},
						"401": map[string]any{"description": "Unauthorized"},
						"409": map[string]any{"description": "Invalid transition"},
					},
				},
			},
			"/api/oxygenation_request/{id}/review": map[string]any{
				"put": map[string]any{
					"summary":  "Review formed request (moderator only)",
					"security": []map[string]any{{"bearerAuth": []string{}}},
					"parameters": []map[string]any{
						{
							"name":     "id",
							"in":       "path",
							"required": true,
							"schema":   map[string]any{"type": "integer"},
						},
					},
					"requestBody": map[string]any{
						"required": true,
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{"$ref": "#/components/schemas/ReviewRequestRequest"},
							},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{"description": "OK"},
						"401": map[string]any{"description": "Unauthorized"},
						"403": map[string]any{"description": "Forbidden"},
						"409": map[string]any{"description": "Invalid transition"},
					},
				},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(spec)
}
