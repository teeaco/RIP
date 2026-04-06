# Lab 4 Demo Order (JWT only, no cookies)

Base URL:

`http://localhost:8095`

## 1. Swagger in incognito

1. Open `GET /api/swagger`.
2. Call `POST /api/users/login` in Swagger:
   - creator: `{"login":"creator","password":"creator"}`
   - moderator: `{"login":"moderator","password":"moderator"}`
3. Copy `token` from response.
4. In Swagger click `Authorize` and paste:
   - `Bearer <token>`
5. Call `GET /api/oxygenation_request`.

## Important for Postman

- Do **not** use Postman `Authorization` tab.
- Put tokens only in request headers manually:
  - `Authorization: Bearer {{creator_token}}`
  - `Authorization: Bearer {{moderator_token}}`
- For JSON requests also set:
  - `Content-Type: application/json`

## 2. Postman/Insomnia checks

1. Guest request list (without Authorization):
   - `GET /api/oxygenation_request`
   - expected: `401` (or `403` by policy)

2. Creator login:
   - `POST /api/users/login`
   - body:
     - `{"login":"creator","password":"creator"}`
   - save token as `creator_token`

3. Moderator login:
   - `POST /api/users/login`
   - body:
     - `{"login":"moderator","password":"moderator"}`
   - save token as `moderator_token`

4. Creator sees only own requests:
   - `GET /api/oxygenation_request`
   - header:
     - `Authorization: Bearer {{creator_token}}`

5. Creator creates/updates draft:
   - `POST /api/request-services`
   - header:
     - `Authorization: Bearer {{creator_token}}`
   - body:
     - `{"service_id":1}`
   - take `request_id` from response

6. Fill request fields:
   - `PUT /api/oxygenation_request/{{request_id}}`
   - header:
     - `Authorization: Bearer {{creator_token}}`
   - body:
     - `{"patient_name":"Demo Patient","blood_value_pao2":90.1,"fio2_value":0.5}`

7. Form request:
   - `PUT /api/oxygenation_request/{{request_id}}/form`
   - header:
     - `Authorization: Bearer {{creator_token}}`

8. Try complete by creator (must fail):
   - `PUT /api/oxygenation_request/{{request_id}}/review`
   - header:
     - `Authorization: Bearer {{creator_token}}`
   - body:
     - `{"action":"complete"}`
   - expected: `403`

9. Complete by moderator (must succeed):
   - `PUT /api/oxygenation_request/{{request_id}}/review`
   - header:
     - `Authorization: Bearer {{moderator_token}}`
   - body:
     - `{"action":"complete"}`
   - expected: `200` + updated `completed_at` and `moderator_login`

10. Moderator sees all requests:
   - `GET /api/oxygenation_request`
   - header:
     - `Authorization: Bearer {{moderator_token}}`

## 3. Redis proof (sessions)

Run from terminal:

```bash
docker exec rip-redis-1 redis-cli -a password KEYS "rip:session:*"
docker exec rip-redis-1 redis-cli -a password GET <session_key>
```

`GET` should contain JSON with user fields (`user_id`, `login`, `role`).
