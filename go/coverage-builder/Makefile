# Makefile for Go Coverage Builder deployment to Cloud Run

# Configuration
PROJECT_ID ?= $(shell gcloud config get-value project)
REGION ?= us-central1
SERVICE_NAME ?= go-coverage-builder
IMAGE_NAME ?= gcr.io/$(PROJECT_ID)/$(SERVICE_NAME)
PORT ?= 8080

# Build and deployment targets
.PHONY: build push deploy all clean

# Build Docker image
build:
	@echo "Building Docker image..."
	docker build -t $(IMAGE_NAME) .

# Push to Google Container Registry
push: build
	@echo "Pushing image to GCR..."
	docker push $(IMAGE_NAME)

# Deploy to Cloud Run
deploy: push
	@echo "Deploying to Cloud Run..."
	gcloud run deploy $(SERVICE_NAME) \
		--image $(IMAGE_NAME) \
		--platform managed \
		--region $(REGION) \
		--port $(PORT) \
		--allow-unauthenticated \
		--memory 1Gi \
		--cpu 1 \
		--max-instances 10 \
		--concurrency 100

# Get service URL
url:
	@echo "Service URL:"
	@gcloud run services describe $(SERVICE_NAME) \
		--platform managed \
		--region $(REGION) \
		--format 'value(status.url)'

# View logs
logs:
	gcloud logs read --service $(SERVICE_NAME) --limit 50

# All in one command
all: deploy
	@echo "Deployment complete!"
	@make url

# Local development
run-local:
	go run main.go

# Build locally
build-local:
	go build -o coverage-builder

# Clean up
clean:
	rm -f coverage-builder
	docker rmi $(IMAGE_NAME) || true

# Setup GCP (run once)
setup-gcp:
	@echo "Enabling required APIs..."
	gcloud services enable containerregistry.googleapis.com
	gcloud services enable run.googleapis.com
	@echo "Setup complete!"

# Update service with new configuration
update:
	gcloud run services update $(SERVICE_NAME) \
		--platform managed \
		--region $(REGION) \
		--memory 1Gi \
		--cpu 1 \
		--max-instances 10 \
		--concurrency 100

# Delete service
delete:
	gcloud run services delete $(SERVICE_NAME) \
		--platform managed \
		--region $(REGION) \
		--quiet

# Show help
help:
	@echo "Go Coverage Builder - Cloud Run Deployment"
	@echo
	@echo "Usage:"
	@echo "  make setup-gcp    - Enable required GCP APIs (run once)"
	@echo "  make build        - Build Docker image"
	@echo "  make push         - Push image to GCR"
	@echo "  make deploy       - Deploy to Cloud Run"
	@echo "  make all          - Build, push, and deploy"
	@echo "  make url          - Get service URL"
	@echo "  make logs         - View service logs"
	@echo "  make update       - Update service configuration"
	@echo "  make delete       - Delete the service"
	@echo "  make run-local    - Run locally for development"
	@echo
	@echo "Environment variables:"
	@echo "  PROJECT_ID        - GCP project ID (default: current gcloud project)"
	@echo "  REGION           - Cloud Run region (default: us-central1)"
	@echo "  SERVICE_NAME     - Service name (default: go-coverage-builder)"