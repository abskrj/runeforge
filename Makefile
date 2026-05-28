.PHONY: up down build logs seed tidy copy-platform-libs help

## tidy: run go mod tidy to generate/update go.sum (required before first build)
tidy:
	cd services/control-plane && go mod tidy

## up: build images and start all services in detached mode
##     Run "make tidy" first to ensure go.sum is up to date.
up:
	docker compose up --build -d

## down: stop and remove all containers and volumes
down:
	docker compose down -v

## logs: stream control-plane logs
logs:
	docker compose logs -f control-plane

## copy-platform-libs: sync platform-libraries/ into the embedded files directory
copy-platform-libs:
	rm -rf services/control-plane/internal/platformlibs/files/bun \
	       services/control-plane/internal/platformlibs/files/python
	cp -r platform-libraries/bun  services/control-plane/internal/platformlibs/files/
	cp -r platform-libraries/python services/control-plane/internal/platformlibs/files/

## build: compile the control-plane binary locally (requires Go 1.22+ and copy-platform-libs)
build: copy-platform-libs
	cd services/control-plane && go build ./...

## seed: create a demo tenant (no auth required — bootstrap endpoint)
##
## After seeding, create an API key with:
##   curl -s -X POST http://localhost:8080/v1/tenants/demo/api-keys \
##     -H "Authorization: Bearer <admin-key>" \
##     -H "Content-Type: application/json" \
##     -d '{"name":"default","scopes":["admin","manage","invoke"]}' | jq .
seed:
	@echo "--- Creating demo tenant ---"
	@curl -sf -X POST http://localhost:8080/v1/tenants \
	  -H "Content-Type: application/json" \
	  -d '{"name":"Demo Org","slug":"demo"}' | jq .

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed 's/^## //'
