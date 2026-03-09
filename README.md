# Лабораторная 3 (REST веб-сервис + PostgreSQL + Minio)

## Что реализовано
- REST API под префиксом `/api` (16 методов по доменам услуги, м-м, заявки, пользователи).
- Работа с БД через GORM ORM.
- Фильтрация:
  - услуги: `GET /api/services?query=...`
  - заявки: `GET /api/oxygenation_request?status=...&formed_from=...&formed_to=...`
- Загрузка файлов изображения и видео в Minio при `POST /api/services`.
- Бизнес-правила переходов статусов:
  - `draft -> formed` (создатель),
  - `formed -> completed|rejected` (модератор),
  - удаление только черновика в `deleted`.
- Системные поля выставляются только на бэкенде.
- Singleton с фиксированным пользователем (создатель/модератор):
  - `internal/app/actor/singleton.go`

## Формула результата
`PaO2 / FiO2`

Формула вычисляется на сервере при формировании заявки: `PUT /api/oxygenation_request/{id}/form`.
Результат сохраняется в `mm_coefficient`.

## 16 REST методов (коллекция для Postman/Insomnia)

### Домен услуги
1. `GET /api/services` — список с фильтром `query`
2. `GET /api/services/{id}` — одна услуга
3. `POST /api/services` — добавление услуги + upload image/video в Minio (multipart/form-data)

### Домен м-м заявки-услуги
4. `POST /api/request-services` — добавить услугу в draft-заявку (если нет draft, создается)
5. `PUT /api/request-services/{requestID}/{serviceID}` — изменить количество/порядок/комментарий/признак основной
6. `DELETE /api/request-services/{requestID}/{serviceID}` — удалить услугу из draft-заявки

### Домен заявки
7. `GET /api/oxygenation_request/cart` — иконка корзины (id draft + количество)
8. `GET /api/oxygenation_request` — список заявок (кроме `draft` и `deleted`) с фильтрацией
9. `GET /api/oxygenation_request/{id}` — одна заявка + список услуг с картинками/видео
10. `PUT /api/oxygenation_request/{id}` — изменение тематических полей заявки
11. `PUT /api/oxygenation_request/{id}/form` — сформировать заявку (валидация + расчет `mm_coefficient`)
12. `PUT /api/oxygenation_request/{id}/review` — завершить/отклонить сформированную заявку модератором
13. `DELETE /api/oxygenation_request/{id}` — удалить черновик (логический статус `deleted`)

### Домен пользователя
14. `POST /api/users/register` — регистрация
15. `POST /api/users/login` — аутентификация (заглушка для ЛР4)
16. `POST /api/users/logout` — деавторизация (заглушка для ЛР4)

## Запуск
1. Поднять инфраструктуру:
```powershell
docker compose up -d db adminer minio redis
```

2. Запустить сервер:
```powershell
$env:DB_HOST='127.0.0.1'
$env:DB_PORT='55432'
$env:DB_USER='root'
$env:DB_PASSWORD='root'
$env:DB_NAME='RIP'
$env:DB_SSLMODE='disable'
go run ./cmd/rip
```

Если `8080` занят, сервер автоматически перейдет на `8095`.  
Смотреть активный порт в логе: `server started at http://localhost:<port>`.

## Minio env
- `MINIO_ENDPOINT` (default `localhost:9000`)
- `MINIO_ACCESS_KEY` (default `root`)
- `MINIO_SECRET_KEY` (default `rootroot`)
- `MINIO_BUCKET` (default `images`)
- `MINIO_USE_SSL` (default `false`)
- `MINIO_PUBLIC_BASE_URL` (default `http://localhost:9000/images`)

## Быстрый сценарий показа ЛР3 (по порядку)
1. `GET /api/oxygenation_request` с фильтрами `formed_from/status`.
2. `GET /api/oxygenation_request/cart`.
3. При наличии draft: `DELETE /api/oxygenation_request/{id}`.
4. `GET /api/services?query=...`.
5. `POST /api/services` с `image` и `video`.
6. `POST /api/request-services` (первая услуга).
7. `POST /api/request-services` (вторая услуга).
8. `GET /api/oxygenation_request/cart`.
9. `GET /api/oxygenation_request/{id}`.
10. `PUT /api/request-services/{requestID}/{serviceID}` (изменить поле м-м).
11. `PUT /api/oxygenation_request/{id}` (изменить поля заявки).
12. `PUT /api/oxygenation_request/{id}/review` до формирования (ожидаем ошибка перехода).
13. `PUT /api/oxygenation_request/{id}/form`.
14. `PUT /api/oxygenation_request/{id}/review` с `{"action":"complete"}`.
15. `POST /api/users/register`.
16. Проверка изменений в БД через Adminer (`SELECT` по `oxygenation_requests`, `oxygenation_request_services`).

## Где смотреть код
- Роутинг: `internal/api/server.go`
- REST контроллеры + сериализаторы: `internal/app/rest/handler.go`
- ORM модели/бизнес-логика/миграции: `internal/app/repository/repository.go`
- ORM методы ЛР3: `internal/app/repository/api_repository.go`
- Singleton пользователя: `internal/app/actor/singleton.go`
- Minio upload: `internal/app/storage/minio.go`
