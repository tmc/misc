DATABASE_URL=postgres://postgres:postgres@localhost/ent_pgvector?sslmode=disable

.PHONY: migrate
migrate:
	atlas migrate apply --dir file://ent/migrate/migrations --url $(DATABASE_URL)

.PHONY: rehash-migrations
rehash-migrations:
	atlas migrate hash --dir file://ent/migrate/migrations

