# Lab 3 Demo Order

Base URL: http://localhost:8095

Headers for protected methods (set manually in request headers):

- `Authorization: Bearer {{creator_token}}` for creator methods
- `Authorization: Bearer {{moderator_token}}` for moderator review/list
- `Content-Type: application/json` for JSON bodies

1) Run 01
2) Run 02
3) If has_draft=true, set draft_id and run 03
4) Run 04
5) Run 05
6) Run 06, copy request_id from response to variable request_id (and draft_id)
7) Run 07
8) Run 08
9) Run 09
10) Run 10
11) Run 11
12) Run 12 (409 is expected)
13) Run 13
14) Run 14
15) Set unique new_login and run 15
16) Run 16 and copy `token` to `creator_token` (or use creator login before step 02)
