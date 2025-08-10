# ==== Параметры Keycloak / клиента ====
KC_HOST    ?= http://localhost:8081
REALM      ?= mobileapp
TOKEN_URL  := $(KC_HOST)/realms/$(REALM)/protocol/openid-connect/token

CLIENT_ID  ?= mobileapp-mobile
USERNAME   ?= testuser
PASSWORD   ?= testpass
SCOPE      ?= openid profile email

BASE ?= http://localhost:8080
AUTH := -H "Authorization: Bearer $$(cat .token)"

TOKEN_FILE := .token

.PHONY: up down logs curl-health curl-me create-user list-users get-user delete-user


# Создаёт/обновляет файл с токеном (живёт между запусками make)
.token:
	@echo "Fetching access token..."
	@curl -s -X POST "$(TOKEN_URL)" \
	 -H "Content-Type: application/x-www-form-urlencoded" \
	 -d "client_id=$(CLIENT_ID)" \
	 -d "grant_type=password" \
	 -d "username=$(USERNAME)" \
	 -d "password=$(PASSWORD)" \
	 -d "scope=$(SCOPE)" | jq -r '.access_token' > $(TOKEN_FILE)
	@test -s $(TOKEN_FILE) || (echo "ERROR: token not received"; rm -f $(TOKEN_FILE); exit 1)
	@echo "Token saved to $(TOKEN_FILE)"

# Явно получить/обновить токен
token: .token
	@echo "OK"

# Форс-обновление токена (игнорирует кэш)
token-refresh:
	@rm -f $(TOKEN_FILE)
	@$(MAKE) --no-print-directory token


clean:
	@rm -f $(TOKEN_FILE)

up:
	cd infra/docker && docker compose up --build -d

down:
	cd infra/docker && docker compose down -v

logs:
	cd infra/docker && docker compose logs -f

curl-health: .token
	@TMP=$$(mktemp); \
	curl -s -D - $(AUTH) $(BASE)/healthz -o $$TMP | tr -d '\r'; \
	echo; \
	jq . < $$TMP; \
	rm -f $$TMP

curl-me: .token
	@curl -s $(AUTH) $(BASE)/api/v1/me | jq .

create-user: .token
	@curl -s -X POST $(AUTH) $(BASE)/api/v1/users \
	 -H "Content-Type: application/json" \
	 -d '{"email":"test1@example.com","name":"Test User"}' | jq .

list-users: .token
	@curl -s $(AUTH) "$(BASE)/api/v1/users?limit=50" | jq .

get-user: .token
	@if [ -z "$(id)" ]; then echo 'usage: make get-user id=<uuid>'; exit 1; fi
	@curl -s $(AUTH) $(BASE)/api/v1/users/$(id) | jq .

delete-user: .token
	@if [ -z "$(id)" ]; then echo 'usage: make delete-user id=<uuid>'; exit 1; fi
	@curl -s -i -X DELETE $(AUTH) $(BASE)/api/v1/users/$(id)


status:
	cd infra/docker && docker compose ps

.PHONY: helm-template helm-package

helm-template:
	helm template dev ./helm/usersvc \
	  --set image.repository=jemone/usersvc \
	  --set image.tag=dev > /tmp/usersvc.yaml && echo "/tmp/usersvc.yaml generated"

helm-package:
	helm package ./helm/usersvc -d ./helm
	
