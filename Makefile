.PHONY: check-sync run-backend run-frontend

check-sync:
	bash scripts/verify_sync.sh

run-backend:
	go run ./backend/cmd/server

run-frontend:
	cd frontend && npm run dev
