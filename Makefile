.PHONY: up down build test test-python test-go test-ts lint clean logs health

up:
	docker compose up -d --build

down:
	docker compose down

build:
	docker compose build

test: test-python test-go test-ts

test-python:
	cd api-gateway && pip install -r requirements.txt -q && pytest -v

test-go:
	cd task-worker && go test -v ./...

test-ts:
	cd dashboard && npm install --silent && npm test

lint: lint-python lint-go lint-ts

lint-python:
	cd api-gateway && flake8 --max-line-length=120 app.py

lint-go:
	cd task-worker && go vet ./...

lint-ts:
	cd dashboard && npx eslint src/

logs:
	docker compose logs -f

health:
	@echo "API Gateway:" && curl -s http://localhost:5000/health | python3 -m json.tool
	@echo "\nTask Worker:" && curl -s http://localhost:5001/health | python3 -m json.tool
	@echo "\nDashboard:" && curl -s http://localhost:3000/health | python3 -m json.tool

clean:
	docker compose down -v --rmi local
	find . -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null || true
	rm -rf dashboard/node_modules dashboard/dist
	rm -rf task-worker/task-worker
