# Lab 4 Demo Order (Manual Headers, No Postman Variables)

Base URL for all requests:

`http://localhost:8095`

## Swagger (incognito)

1. Open `http://localhost:8095/api/swagger`.
2. Run `POST /api/users/login` for creator:
   `{"login":"creator","password":"creator"}`
3. Run `POST /api/users/login` for moderator:
   `{"login":"moderator","password":"moderator"}`
4. Copy `token` values from both responses.
5. In Swagger `Authorize` insert:
   `Bearer <token>`
6. Run `GET /api/oxygenation_request`.

## Postman setup (strictly manual headers)

1. In every protected request set `Authorization` tab to `No Auth`.
2. Open `Headers` tab and fill keys manually (white editable rows):
   - `Authorization: Bearer <PASTE_TOKEN_HERE>`
   - `Content-Type: application/json` (for POST/PUT with JSON body)
   - `Accept: application/json`
3. Do not use `{{...}}` variables.

## Demo flow in Postman

1. Guest check (without Authorization header)
   - `GET http://localhost:8095/api/oxygenation_request`
   - Expected: `401` (or `403` by policy)

2. Login creator
   - `POST http://localhost:8095/api/users/login`
   - Headers:
     - `Content-Type: application/json`
     - `Accept: application/json`
   - Body:
     - `{"login":"creator","password":"creator"}`
   - Copy `token` from response.

3. Login moderator
   - `POST http://localhost:8095/api/users/login`
   - Headers:
     - `Content-Type: application/json`
     - `Accept: application/json`
   - Body:
     - `{"login":"moderator","password":"moderator"}`
   - Copy `token` from response.

4. Creator sees only own requests
   - `GET http://localhost:8095/api/oxygenation_request`
   - Headers:
     - `Authorization: Bearer <CREATOR_TOKEN_FROM_LOGIN>`
     - `Accept: application/json`

5. Create draft (creator)
   - `POST http://localhost:8095/api/request-services`
   - Headers:
     - `Authorization: Bearer <CREATOR_TOKEN_FROM_LOGIN>`
     - `Content-Type: application/json`
     - `Accept: application/json`
   - Body:
     - `{"service_id":1}`
   - Take `request_id` from response and manually paste it into next URLs.

6. Update request fields (creator)
   - `PUT http://localhost:8095/api/oxygenation_request/101`
   - Headers:
     - `Authorization: Bearer <CREATOR_TOKEN_FROM_LOGIN>`
     - `Content-Type: application/json`
     - `Accept: application/json`
   - Body:
     - `{"patient_name":"Demo Patient","blood_value_pao2":90.1,"fio2_value":0.5}`

7. Form request (creator)
   - `PUT http://localhost:8095/api/oxygenation_request/101/form`
   - Headers:
     - `Authorization: Bearer <CREATOR_TOKEN_FROM_LOGIN>`
     - `Accept: application/json`

8. Try complete by creator (must fail)
   - `PUT http://localhost:8095/api/oxygenation_request/101/review`
   - Headers:
     - `Authorization: Bearer <CREATOR_TOKEN_FROM_LOGIN>`
     - `Content-Type: application/json`
     - `Accept: application/json`
   - Body:
     - `{"action":"complete"}`
   - Expected: `403`

9. Complete by moderator (must succeed)
   - `PUT http://localhost:8095/api/oxygenation_request/101/review`
   - Headers:
     - `Authorization: Bearer <MODERATOR_TOKEN_FROM_LOGIN>`
     - `Content-Type: application/json`
     - `Accept: application/json`
   - Body:
     - `{"action":"complete"}`
   - Expected: `200` and updated `completed_at`, `moderator_login`.

10. Moderator sees all requests
   - `GET http://localhost:8095/api/oxygenation_request`
   - Headers:
     - `Authorization: Bearer <MODERATOR_TOKEN_FROM_LOGIN>`
     - `Accept: application/json`

## Redis sessions check

```bash
docker exec rip-redis-1 redis-cli -a password KEYS "rip:session:*"
docker exec rip-redis-1 redis-cli -a password GET <session_key>
```

The `GET` value must include user fields: `user_id`, `login`, `role`.
