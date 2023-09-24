DATABASE_URL=postgres://postgres:postgres@localhost/ent_pgvector?sslmode=disable
DEV_DATABASE_URL="postgres://postgres:postgres@localhost/ent_pgvector_devdb?search_path=public&sslmode=disable"

.PHONY: migrate
migrate:
	atlas migrate apply --dir file://ent/migrate/migrations --url $(DATABASE_URL)

.PHONY: rehash-migrations
rehash-migrations:
	atlas migrate hash --dir file://ent/migrate/migrations

.PHONY: generate
generate:
	go generate ./...

.PHONY: lint-migrations
lint-migrations:
	atlas migrate lint \
	  --dev-url=$(DEV_DATABASE_URL) \
	  --dir="file://ent/migrate/migrations" \
	  --latest=1

.PHONY: migration-status
migration-status:
	atlas migrate status \
	  --url $(DATABASE_URL) \
	  --dir="file://ent/migrate/migrations"

.PHONY: new-migration
new-migration: MIGRATION_NAME?=$(error MIGRATION_NAME is not set)
new-migration:
	atlas migrate diff $(MIGRATION_NAME) \
	  --dir "file://ent/migrate/migrations" \
	  --to "ent://ent/schema" \
	  --dev-url=$(DEV_DATABASE_URL)
