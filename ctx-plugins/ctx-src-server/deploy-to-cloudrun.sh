#!/bin/bash
# Deploy ctx-src-server to Cloud Run
set -e

# Default values
PROJECT_ID=$(gcloud config get-value project 2>/dev/null)
REGION="us-central1"
SERVICE_NAME="ctx-src-server"
CACHE_DIR="/tmp/ctx-src-cache"
CREATE_GCS_BUCKET=false
GCS_BUCKET=""
GCS_MOUNT="/mnt/ctx-src-cache"
MAX_CONCURRENT=3
CLONE_TIMEOUT="5m"
VERBOSE=true
DEFAULT_BRANCH="main"

# Parse command-line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --project)
            PROJECT_ID="$2"
            shift 2
            ;;
        --region)
            REGION="$2"
            shift 2
            ;;
        --service-name)
            SERVICE_NAME="$2"
            shift 2
            ;;
        --cache-dir)
            CACHE_DIR="$2"
            shift 2
            ;;
        --create-gcs-bucket)
            CREATE_GCS_BUCKET=true
            shift
            ;;
        --gcs-bucket)
            GCS_BUCKET="$2"
            shift 2
            ;;
        --gcs-mount)
            GCS_MOUNT="$2"
            shift 2
            ;;
        --max-concurrent)
            MAX_CONCURRENT="$2"
            shift 2
            ;;
        --clone-timeout)
            CLONE_TIMEOUT="$2"
            shift 2
            ;;
        --default-branch)
            DEFAULT_BRANCH="$2"
            shift 2
            ;;
        --no-verbose)
            VERBOSE=false
            shift
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo "Options:"
            echo "  --project ID           Google Cloud project ID"
            echo "  --region REGION        Cloud Run region (default: us-central1)"
            echo "  --service-name NAME    Service name (default: ctx-src-server)"
            echo "  --cache-dir DIR        Local cache directory (default: /tmp/ctx-src-cache)"
            echo "  --create-gcs-bucket    Create a GCS bucket for caching"
            echo "  --gcs-bucket NAME      Use existing GCS bucket for caching"
            echo "  --gcs-mount PATH       GCS mount point (default: /mnt/ctx-src-cache)"
            echo "  --max-concurrent N     Max concurrent git operations (default: 3)"
            echo "  --clone-timeout TIME   Timeout for clone operations (default: 5m)"
            echo "  --default-branch NAME  Default branch name (default: main)"
            echo "  --no-verbose           Disable verbose output"
            echo "  --help                 Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Check if project ID is set
if [ -z "$PROJECT_ID" ]; then
    echo "Project ID is not set. Please specify with --project flag or configure gcloud."
    exit 1
fi

# Handle GCS bucket setup
if [ "$CREATE_GCS_BUCKET" = true ] && [ -z "$GCS_BUCKET" ]; then
    # Generate a unique bucket name using project ID
    GCS_BUCKET="${PROJECT_ID}-ctx-src-cache"
    echo "=== Creating GCS bucket $GCS_BUCKET ==="
    
    # Check if bucket already exists
    if gsutil ls -b "gs://${GCS_BUCKET}" &>/dev/null; then
        echo "Bucket gs://${GCS_BUCKET} already exists, using it"
    else
        gsutil mb -p "$PROJECT_ID" -l "$REGION" "gs://${GCS_BUCKET}"
        echo "Created bucket gs://${GCS_BUCKET}"
        
        # Set lifecycle policy to expire objects after 30 days
        echo '{
            "lifecycle": {
                "rule": [{
                    "action": {"type": "Delete"},
                    "condition": {"age": 30}
                }]
            }
        }' > /tmp/lifecycle.json
        
        gsutil lifecycle set /tmp/lifecycle.json "gs://${GCS_BUCKET}"
        echo "Set lifecycle policy: objects expire after 30 days"
    fi
elif [ -n "$GCS_BUCKET" ]; then
    echo "=== Using existing GCS bucket: $GCS_BUCKET ==="
    
    # Check if bucket exists
    if ! gsutil ls -b "gs://${GCS_BUCKET}" &>/dev/null; then
        echo "Error: Bucket gs://${GCS_BUCKET} does not exist"
        exit 1
    fi
fi

# Prepare command-line args for the service
SERVICE_ARGS=(
    "--addr=:8080"
    "--max-concurrent=$MAX_CONCURRENT"
    "--clone-timeout=$CLONE_TIMEOUT"
    "--default-branch=$DEFAULT_BRANCH"
)

# Add verbosity flag if enabled
if [ "$VERBOSE" = true ]; then
    SERVICE_ARGS+=("--verbose")
fi

# Configure cache storage
if [ -n "$GCS_BUCKET" ]; then
    SERVICE_ARGS+=("--gcs-bucket=$GCS_BUCKET" "--gcs-mount=$GCS_MOUNT")
    
    # Grant additional permissions for GCS access
    echo "=== Granting additional GCS permissions ==="
    gcloud projects add-iam-policy-binding "$PROJECT_ID" \
        --member="serviceAccount:$SERVICE_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
        --role="roles/storage.objectAdmin"
else
    SERVICE_ARGS+=("--cache-dir=$CACHE_DIR")
fi

# Build the Docker image
echo "=== Building Docker image ==="
DOCKER_IMAGE="gcr.io/$PROJECT_ID/$SERVICE_NAME:latest"
SCRIPT_DIR="$(dirname "$0")"
docker build -t "$DOCKER_IMAGE" "$SCRIPT_DIR"

# Push the Docker image
echo "=== Pushing Docker image to Google Container Registry ==="
docker push "$DOCKER_IMAGE"

# Ensure service account exists
echo "=== Checking service account ==="
SA_NAME="$SERVICE_NAME"
SA_EMAIL="$SA_NAME@$PROJECT_ID.iam.gserviceaccount.com"
if ! gcloud iam service-accounts describe "$SA_EMAIL" &>/dev/null; then
    echo "Creating service account $SA_EMAIL"
    gcloud iam service-accounts create "$SA_NAME" \
        --description="Service account for $SERVICE_NAME" \
        --display-name="$SERVICE_NAME"
fi

# Grant necessary permissions
echo "=== Granting IAM permissions ==="
# Grant permissions for Cloud Storage (for accessing GCS buckets)
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
    --member="serviceAccount:$SA_EMAIL" \
    --role="roles/storage.objectViewer"

# Update Cloud Run configuration
echo "=== Updating Cloud Run YAML configuration ==="
SCRIPT_DIR="$(dirname "$0")"
CLOUDRUN_CONFIG="$SCRIPT_DIR/cloudrun/service.yaml"
sed -i "s|ctx-src-server@PROJECT_ID.iam.gserviceaccount.com|$SA_EMAIL|g" "$CLOUDRUN_CONFIG"
sed -i "s|gcr.io/PROJECT_ID/ctx-src-server:latest|$DOCKER_IMAGE|g" "$CLOUDRUN_CONFIG"

# Update arguments in the Cloud Run config
args_json=$(printf '%s\n' "${SERVICE_ARGS[@]}" | jq -R . | jq -s .)
tmp_file=$(mktemp)
jq --argjson args "$args_json" '.spec.template.spec.containers[0].args = $args' "$CLOUDRUN_CONFIG" > "$tmp_file"
mv "$tmp_file" "$CLOUDRUN_CONFIG"

# Deploy to Cloud Run
echo "=== Deploying to Cloud Run ==="
gcloud run services replace "$CLOUDRUN_CONFIG" \
    --project="$PROJECT_ID" \
    --region="$REGION" \
    --platform=managed \
    --quiet

# Allow unauthenticated invocations (optional, comment out if not needed)
echo "=== Configuring public access ==="
gcloud run services add-iam-policy-binding "$SERVICE_NAME" \
    --project="$PROJECT_ID" \
    --region="$REGION" \
    --member="allUsers" \
    --role="roles/run.invoker"

# Get the deployed service URL
SERVICE_URL=$(gcloud run services describe "$SERVICE_NAME" \
    --project="$PROJECT_ID" \
    --region="$REGION" \
    --format="value(status.url)")

echo "=== Deployment Complete ==="
echo "Service URL: $SERVICE_URL"
echo ""
echo "Test the service with:"
echo "curl -X POST $SERVICE_URL/src -H \"Content-Type: application/json\" -d '{\"owner\":\"tmc\",\"repo\":\"misc\",\"paths\":[\"ctx-plugins/**/*.go\"]}'"

# Test the metrics endpoint
echo ""
echo "View metrics with:"
echo "curl $SERVICE_URL/metrics | jq ."