up:
	docker compose -f deploy/compose.yaml up -d
down:
	docker compose -f deploy/compose.yaml down

run-%:
	set -a; . ./.env.local; set +a; go run ./cmd/$*
