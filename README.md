# Лабораторная 1 (backend + шаблоны + MinIO)

## Запуск сервера
1. `go test ./...`
2. `go run ./cmd/rip`
3. Открыть `http://localhost:8080/services`

Если `8080` занят:
- PowerShell: `$env:APP_PORT="8081"; go run ./cmd/rip`
- Тогда URL: `http://localhost:8081/services`

## Роутинг (3 GET + 3 контроллера)
- `GET /services` -> список услуг + серверный поиск `query`
- `GET /services/{id}` -> карточка услуги по `id`
- `GET /requests/{id}` -> просмотр заявки по `id`

## Данные (без БД)
- Коллекция услуг: массив `[]Service`
- Коллекция заявок: словарь `map[int]Request`
- Источник: `internal/app/repository/repository.go`

## Где находится логика
- Модели: `internal/app/model/model.go`
- Репозиторий: `internal/app/repository/repository.go`
- Контроллеры: `internal/app/handler/handler.go`
- HTTP сервер: `internal/api/server.go`
- Точка входа: `cmd/rip/main.go`

## Шаблоны и статика
- Шаблоны: `templates/*.html`
- Общий header (одинаковая кнопка Домой): `templates/partials/header.html`
- CSS: `resources/styles/style.css`
- Раздача статики: `/static/...`

## MinIO (локально)
1. `docker compose up -d minio`
2. Консоль: `http://localhost:9001`
3. Логин/пароль: `root / rootroot`
4. Создать bucket `images`
5. Загрузить файлы:
   - `normal.png`, `mild.png`, `moderate.png`, `severe.png`
   - `normal.mp4`, `mild.mp4`, `moderate.mp4`, `severe.mp4`
6. Дать bucket политику чтения (public read)
