.PHONY: up down logs curl-health curl-me create-user list-users get-user delete-user

up:
	cd infra/docker && docker compose up --build -d

down:
	cd infra/docker && docker compose down -v

logs:
	cd infra/docker && docker compose logs -f

curl-health:
	curl -s http://localhost:8080/healthz | jq .

curl-me:
	curl -s http://localhost:8080/api/v1/me | jq .

create-user:
	curl -s -X POST http://localhost:8080/api/v1/users \
	 -H "Content-Type: application/json" \
	 -d '{"email":"test1@example.com","name":"Test User"}' | jq .

list-users:
	curl -s "http://localhost:8080/api/v1/users?limit=50" | jq .

get-user:
	@if [ -z "$(id)" ]; then echo 'usage: make get-user id=<uuid>'; exit 1; fi
	curl -s http://localhost:8080/api/v1/users/$(id) | jq .

delete-user:
	@if [ -z "$(id)" ]; then echo 'usage: make delete-user id=<uuid>'; exit 1; fi
	curl -i -X DELETE http://localhost:8080/api/v1/users/$(id)

	

