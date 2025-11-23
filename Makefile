.PHONY: build run test lint clean migrate

build:
	docker-compose build

run:
	docker-compose up

test:
	go test ./... -v

lint:
	golangci-lint run

clean:
	docker-compose down -v
	rm -f main

migrate:
	docker-compose run --rm app /app/migrate

db-shell:
	docker-compose exec postgres psql -U postgres -d pr_reviewer