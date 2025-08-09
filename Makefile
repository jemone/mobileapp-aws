.PHONY: up down logs curl-health curl-me

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

