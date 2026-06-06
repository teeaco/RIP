# Лабораторная работа 2

## Задание
Подключить PostgreSQL, перенести данные предметной области в таблицы и использовать GORM для работы с услугами, заявками, пользователями и связью заявки с услугами.

## Что сделано
- Созданы таблицы `app_users`, `oxygenation_services`, `oxygenation_requests`, `oxygenation_request_services`.
- Добавлены статусы заявки: `draft`, `deleted`, `formed`, `completed`, `rejected`.
- Сделан уникальный черновик заявки для пользователя.
- Реализованы получение услуг, поиск, карточка услуги и состав заявки.
- Добавлено создание черновика при добавлении услуги.
- Удаление заявки выполнено как смена статуса на `deleted`.
- Индекс оксигенации хранится в поле `mm_coefficient`.

## Формула
`PaO2 / FiO2`

## Маршруты
- `/services` — список услуг и поиск.
- `/services/{id}` — карточка услуги.
- `/oxygenation_request/{id}` — состав заявки.
- `/oxygenation_request/add-service` — добавление услуги в черновик.
- `/oxygenation_request/{id}/delete` — логическое удаление черновика.

## Запуск
1. Поднять инфраструктуру: `docker compose up -d db adminer minio redis`
2. Проверить контейнеры: `docker compose ps`
3. Проверить проект: `go test ./...`
4. Запустить сервер: `go run ./cmd/rip`
5. Открыть приложение: `http://localhost:8080/services`
6. Открыть Adminer: `http://localhost:8081`

## Подключение к базе
- `DB_HOST=127.0.0.1`
- `DB_PORT=55432`
- `DB_USER=root`
- `DB_PASSWORD=root`
- `DB_NAME=RIP`
- `DB_SSLMODE=disable`

## Проверка
1. Добавить услугу в таблицу `oxygenation_services`.
2. Найти услугу на странице `/services`.
3. Добавить две услуги в черновик заявки.
4. Открыть заявку и проверить состав.
5. Удалить черновик.
6. Проверить в Adminer таблицы `oxygenation_requests` и `oxygenation_request_services`.
