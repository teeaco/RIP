# Лабораторная работа 3

## Задание
Реализовать REST-интерфейс для предметной области: услуги оксигенации, заявки, состав заявки и пользователи. Данные хранить в PostgreSQL, файлы услуг загружать в MinIO.

## Что сделано
- Добавлен префикс `/api` для методов серверного интерфейса.
- Реализованы методы для услуг, заявок, состава заявки и пользователей.
- Добавлена фильтрация услуг по `query`.
- Добавлена фильтрация заявок по `status`, `formed_from`, `formed_to`.
- Подключена загрузка изображения и видео услуги в MinIO.
- Реализованы переходы статусов заявки: `draft`, `formed`, `completed`, `rejected`, `deleted`.
- Расчёт `PaO2 / FiO2` выполняется на сервере при формировании заявки.

## Основные методы
- `/api/services` — список и создание услуг.
- `/api/services/{id}` — одна услуга.
- `/api/request-services` — добавление услуги в черновик.
- `/api/request-services/{requestID}/{serviceID}` — изменение и удаление услуги из черновика.
- `/api/oxygenation_request/cart` — состояние черновика.
- `/api/oxygenation_request` — список заявок.
- `/api/oxygenation_request/{id}` — одна заявка.
- `/api/oxygenation_request/{id}/form` — формирование заявки.
- `/api/oxygenation_request/{id}/review` — завершение или отклонение заявки модератором.
- `/api/users/register`, `/api/users/login`, `/api/users/logout` — методы пользователя.

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

Если основной порт занят, сервер перейдёт на запасной порт. Активный адрес выводится в журнале запуска.

## Настройки MinIO
- `MINIO_ENDPOINT`: `localhost:9000`
- `MINIO_ACCESS_KEY`: `root`
- `MINIO_SECRET_KEY`: `rootroot`
- `MINIO_BUCKET`: `images`
- `MINIO_USE_SSL`: `false`
- `MINIO_PUBLIC_BASE_URL`: `http://localhost:9000/images`

## Проверка
1. Получить список заявок с фильтрами.
2. Проверить черновик заявки.
3. Найти услугу через `query`.
4. Создать услугу с изображением и видео.
5. Добавить две услуги в черновик.
6. Изменить данные заявки и состава.
7. Сформировать заявку.
8. Завершить заявку от имени модератора.
9. Проверить изменения в Adminer.
