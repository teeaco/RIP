# Лабораторная 2 (PostgreSQL + GORM + шаблоны)

## Формула расчета результата
Индекс оксигенации рассчитывается по формуле:

`PaO2 / FiO2`

Поле в заявке: `mm_coefficient`.

На странице заявки есть форма ввода `PaO2` и `FiO2` (без JavaScript).  
Пользователь вводит значения, отправляет GET-форму, и сервер пересчитывает:
- индекс оксигенации
- степень по результату

## Что реализовано
- 4 таблицы по предметной области:
  - `app_users`
  - `oxygenation_services`
  - `oxygenation_requests`
  - `oxygenation_request_services`
- 5 статусов заявки:
  - `draft`
  - `deleted`
  - `formed`
  - `completed`
  - `rejected`
- составной уникальный ключ в m-m:
  - `(request_id, service_id)` в `oxygenation_request_services`
- ограничение: у пользователя не более одной `draft` заявки:
  - частичный уникальный индекс `ux_single_draft_request`
- ORM (GORM) для:
  - получения/поиска услуг
  - карточки услуги
  - создания/чтения черновика
  - добавления услуги в черновик
- логическое удаление заявки через SQL `UPDATE` (без ORM).

## HTTP методы (5)
- `GET /services` — список услуг + поиск
- `GET /services/{id}` — карточка услуги
- `GET /requests/{id}` — состав заявки
- `POST /requests/add-service` — добавить услугу в текущую заявку (черновик)
- `POST /requests/{id}/delete` — логически удалить заявку (SQL UPDATE)

## Запуск
1. Поднять инфраструктуру:
   - `docker compose up -d db adminer minio redis`
2. Проверить контейнеры:
   - `docker compose ps`
3. Запустить приложение:
   - `go test ./...`
   - `go run ./cmd/rip`
4. Открыть:
   - приложение: `http://localhost:8080/services`
   - Adminer: `http://localhost:8081`

## Параметры подключения приложения к БД
По умолчанию приложение подключается к контейнерному Postgres:

- `DB_HOST=127.0.0.1`
- `DB_PORT=55432`
- `DB_USER=root`
- `DB_PASSWORD=root`
- `DB_NAME=RIP`
- `DB_SSLMODE=disable`

Если нужно, можно переопределить:
- PowerShell: `$env:DB_PORT="55432"; $env:APP_PORT="8080"; go run ./cmd/rip`

## Доступ в Adminer
- System: `PostgreSQL`
- Server: `db` (если Adminer открыт из docker-compose) или `localhost` (если открываете локально)
- Username: `root`
- Password: `root`
- Database: `RIP`

## Как показать лабораторную
1. В Adminer добавить новую услугу в таблицу `oxygenation_services`.
2. На `/services` выполнить поиск по названию/диапазону.
3. Добавить две услуги кнопкой `Добавить в заявку` (POST через ORM).
4. Открыть корзину (черновик) и показать состав заявки.
5. Нажать `Логически удалить заявку` (POST, SQL UPDATE).
6. Перейти по URL удаленной заявки и показать, что она недоступна.
7. В БД сделать `SELECT` по `oxygenation_requests` и `oxygenation_request_services`:
   - показать `status='deleted'`
   - показать новую `draft` после повторного добавления услуг.
8. Изменить поля заявки/м-м в БД и обновить страницу приложения.

## Где что находится
- Сервер и роутинг: `internal/api/server.go`
- Контроллеры: `internal/app/handler/handler.go`
- Модели + миграции + ORM + SQL update: `internal/app/repository/repository.go`
- Шаблоны: `templates/*.html`
- Стили: `resources/styles/style.css`
