# CI/CD Integration for Synthetic Coverage with Docker and Sandboxes

This guide provides strategies for integrating synthetic coverage generation into CI/CD pipelines when using Docker containers and sandboxed environments.

## GitHub Actions Integration

### Basic GitHub Actions Workflow

```yaml
name: Synthetic Coverage with Docker

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run standard tests with coverage
        run: go test -coverprofile=coverage.out ./...
      
      - name: Build Docker image for container tests
        run: docker build -t app-under-test:latest .
      
      - name: Run tests in Docker with coverage
        run: |
          mkdir -p docker-coverage
          docker run --rm -v $(pwd)/docker-coverage:/coverage \
            -e GOCOVERDIR=/coverage \
            app-under-test:latest
      
      - name: Generate synthetic coverage
        run: go run ./cmd/synthetic-coverage/main.go \
            -real=coverage.out \
            -docker-logs=docker-coverage \
            -synthetic=synthetic.txt \
            -merged=merged-coverage.out
      
      - name: Generate coverage report
        run: go tool cover -html=merged-coverage.out -o coverage.html
      
      - name: Upload coverage report
        uses: actions/upload-artifact@v3
        with:
          name: coverage-report
          path: coverage.html
```

### Advanced GitHub Actions Workflow with Sandbox Support

```yaml
name: Synthetic Coverage with Docker and Sandbox

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  test:
    runs-on: macos-latest  # For macOS sandbox support
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run standard tests with coverage
        run: go test -coverprofile=standard-coverage.out ./...
      
      - name: Run scripttest tests with sandbox
        run: |
          mkdir -p sandbox-tracking
          go test -tags=sandbox ./scripttest/... -v
      
      - name: Generate synthetic coverage for sandbox tests
        run: |
          go run ./cmd/sandbox-coverage/main.go \
            -tracking-dir=sandbox-tracking \
            -command-map=testdata/command_map.json \
            -output=sandbox-coverage.out
      
      - name: Build and run Docker tests
        run: |
          docker build -t app-docker-test:latest .
          mkdir -p docker-coverage
          docker run --rm -v $(pwd)/docker-coverage:/coverage \
            -e GOCOVERDIR=/coverage \
            app-docker-test:latest
      
      - name: Generate synthetic coverage for Docker tests
        run: |
          go run ./cmd/docker-coverage/main.go \
            -docker-logs=docker-coverage \
            -command-map=testdata/docker_command_map.json \
            -output=docker-coverage.out
      
      - name: Merge all coverage data
        run: |
          go run ./cmd/merge-coverage/main.go \
            -files=standard-coverage.out,sandbox-coverage.out,docker-coverage.out \
            -output=merged-coverage.out
      
      - name: Generate coverage report
        run: go tool cover -html=merged-coverage.out -o coverage.html
      
      - name: Calculate coverage percentage
        id: coverage
        run: |
          COVERAGE=$(go tool cover -func=merged-coverage.out | grep total | awk '{print $3}')
          echo "::set-output name=percentage::$COVERAGE"
          echo "Total coverage: $COVERAGE"
      
      - name: Upload coverage report
        uses: actions/upload-artifact@v3
        with:
          name: coverage-report
          path: coverage.html
      
      - name: Create coverage badge
        uses: schneegans/dynamic-badges-action@v1.6.0
        with:
          auth: ${{ secrets.GIST_SECRET }}
          gistID: ${{ secrets.GIST_ID }}
          filename: coverage.json
          label: coverage
          message: ${{ steps.coverage.outputs.percentage }}
          color: green
```

## GitLab CI Integration

```yaml
stages:
  - test
  - coverage
  - report

variables:
  GO_VERSION: "1.21"

test:standard:
  stage: test
  image: golang:${GO_VERSION}
  script:
    - go test -coverprofile=standard-coverage.out ./...
  artifacts:
    paths:
      - standard-coverage.out

test:docker:
  stage: test
  image: docker:latest
  services:
    - docker:dind
  script:
    - docker build -t app-docker-test:latest .
    - mkdir -p docker-coverage
    - docker run --rm -v $(pwd)/docker-coverage:/coverage -e GOCOVERDIR=/coverage app-docker-test:latest
  artifacts:
    paths:
      - docker-coverage/

test:sandbox:
  stage: test
  # Using macOS runner for sandbox support
  tags:
    - macos
  script:
    - mkdir -p sandbox-tracking
    - go test -tags=sandbox ./scripttest/... -v
  artifacts:
    paths:
      - sandbox-tracking/

coverage:synthetic:
  stage: coverage
  image: golang:${GO_VERSION}
  script:
    - go run ./cmd/docker-coverage/main.go -docker-logs=docker-coverage -command-map=testdata/docker_command_map.json -output=docker-coverage.out
    - go run ./cmd/sandbox-coverage/main.go -tracking-dir=sandbox-tracking -command-map=testdata/command_map.json -output=sandbox-coverage.out
  dependencies:
    - test:docker
    - test:sandbox
  artifacts:
    paths:
      - docker-coverage.out
      - sandbox-coverage.out

coverage:merge:
  stage: coverage
  image: golang:${GO_VERSION}
  script:
    - go run ./cmd/merge-coverage/main.go -files=standard-coverage.out,sandbox-coverage.out,docker-coverage.out -output=merged-coverage.out
    - go tool cover -html=merged-coverage.out -o coverage.html
    - go tool cover -func=merged-coverage.out > coverage-summary.txt
  dependencies:
    - test:standard
    - coverage:synthetic
  artifacts:
    paths:
      - merged-coverage.out
      - coverage.html
      - coverage-summary.txt

pages:
  stage: report
  script:
    - mkdir -p public
    - cp coverage.html public/index.html
    - cp coverage-summary.txt public/
  dependencies:
    - coverage:merge
  artifacts:
    paths:
      - public
  only:
    - main
```

## CircleCI Integration

```yaml
version: 2.1

orbs:
  go: circleci/go@1.7
  docker: circleci/docker@2.1

jobs:
  test-standard:
    docker:
      - image: cimg/go:1.21
    steps:
      - checkout
      - go/mod-download
      - run:
          name: Run standard tests with coverage
          command: go test -coverprofile=standard-coverage.out ./...
      - persist_to_workspace:
          root: .
          paths:
            - standard-coverage.out

  test-docker:
    machine:
      image: ubuntu-2204:current
    steps:
      - checkout
      - run:
          name: Build Docker image
          command: docker build -t app-docker-test:latest .
      - run:
          name: Run tests in Docker with coverage
          command: |
            mkdir -p docker-coverage
            docker run --rm -v $(pwd)/docker-coverage:/coverage \
              -e GOCOVERDIR=/coverage \
              app-docker-test:latest
      - persist_to_workspace:
          root: .
          paths:
            - docker-coverage/

  generate-synthetic-coverage:
    docker:
      - image: cimg/go:1.21
    steps:
      - checkout
      - attach_workspace:
          at: .
      - go/mod-download
      - run:
          name: Generate synthetic coverage for Docker tests
          command: |
            go run ./cmd/docker-coverage/main.go \
              -docker-logs=docker-coverage \
              -command-map=testdata/docker_command_map.json \
              -output=docker-coverage.out
      - persist_to_workspace:
          root: .
          paths:
            - docker-coverage.out

  merge-coverage:
    docker:
      - image: cimg/go:1.21
    steps:
      - checkout
      - attach_workspace:
          at: .
      - go/mod-download
      - run:
          name: Merge coverage data
          command: |
            go run ./cmd/merge-coverage/main.go \
              -files=standard-coverage.out,docker-coverage.out \
              -output=merged-coverage.out
      - run:
          name: Generate coverage report
          command: go tool cover -html=merged-coverage.out -o coverage.html
      - store_artifacts:
          path: coverage.html
          destination: coverage-report.html
      - run:
          name: Calculate coverage percentage
          command: |
            COVERAGE=$(go tool cover -func=merged-coverage.out | grep total | awk '{print $3}')
            echo "Total coverage: $COVERAGE"

workflows:
  version: 2
  build-test-coverage:
    jobs:
      - test-standard
      - test-docker
      - generate-synthetic-coverage:
          requires:
            - test-docker
      - merge-coverage:
          requires:
            - test-standard
            - generate-synthetic-coverage
```

## Jenkins Pipeline Integration

```groovy
pipeline {
    agent any
    
    tools {
        go 'go-1.21'
    }
    
    stages {
        stage('Checkout') {
            steps {
                checkout scm
            }
        }
        
        stage('Test') {
            parallel {
                stage('Standard Tests') {
                    steps {
                        sh 'go test -coverprofile=standard-coverage.out ./...'
                    }
                }
                
                stage('Docker Tests') {
                    steps {
                        sh 'docker build -t app-docker-test:latest .'
                        sh 'mkdir -p docker-coverage'
                        sh 'docker run --rm -v $(pwd)/docker-coverage:/coverage -e GOCOVERDIR=/coverage app-docker-test:latest'
                    }
                }
                
                stage('Sandbox Tests') {
                    agent {
                        label 'macos'  // For macOS sandbox support
                    }
                    steps {
                        sh 'mkdir -p sandbox-tracking'
                        sh 'go test -tags=sandbox ./scripttest/... -v'
                    }
                }
            }
        }
        
        stage('Generate Synthetic Coverage') {
            steps {
                sh 'go run ./cmd/docker-coverage/main.go -docker-logs=docker-coverage -command-map=testdata/docker_command_map.json -output=docker-coverage.out'
                sh 'go run ./cmd/sandbox-coverage/main.go -tracking-dir=sandbox-tracking -command-map=testdata/command_map.json -output=sandbox-coverage.out'
            }
        }
        
        stage('Merge Coverage') {
            steps {
                sh 'go run ./cmd/merge-coverage/main.go -files=standard-coverage.out,docker-coverage.out,sandbox-coverage.out -output=merged-coverage.out'
                sh 'go tool cover -html=merged-coverage.out -o coverage.html'
            }
        }
        
        stage('Coverage Report') {
            steps {
                sh 'go tool cover -func=merged-coverage.out > coverage-summary.txt'
                
                script {
                    def coverageText = readFile('coverage-summary.txt')
                    def coverageLine = coverageText.split('\n').find { it.contains('total:') }
                    def coverageValue = coverageLine.split(':')[1].trim()
                    echo "Total coverage: ${coverageValue}"
                }
                
                publishHTML(target: [
                    allowMissing: false,
                    alwaysLinkToLastBuild: false,
                    keepAll: true,
                    reportDir: '.',
                    reportFiles: 'coverage.html',
                    reportName: 'Coverage Report'
                ])
            }
        }
    }
    
    post {
        always {
            archiveArtifacts artifacts: 'coverage.html,coverage-summary.txt,merged-coverage.out', fingerprint: true
        }
    }
}
```

## Common CI/CD Integration Patterns

### 1. Multi-Stage Coverage Collection

This pattern separates test execution, synthetic coverage generation, and coverage merging into distinct stages:

```
Standard Tests → Docker Tests → Sandbox Tests → Generate Synthetic Coverage → Merge Coverage → Report
```

Benefits:
- Clear separation of concerns
- Easier debugging of failed stages
- Parallel execution where possible

### 2. Coverage Threshold Enforcement

This pattern enforces minimum coverage thresholds:

```yaml
# GitHub Actions example
- name: Check coverage threshold
  run: |
    COVERAGE=$(go tool cover -func=merged-coverage.out | grep total | awk '{print $3}' | sed 's/%//')
    if (( $(echo "$COVERAGE < 80" | bc -l) )); then
      echo "Coverage $COVERAGE% is below threshold of 80%"
      exit 1
    fi
```

### 3. Pull Request Coverage Comparison

This pattern compares coverage between branches:

```yaml
# GitHub Actions example
- name: Compare coverage with base branch
  if: github.event_name == 'pull_request'
  run: |
    git fetch origin ${{ github.base_ref }}
    git checkout origin/${{ github.base_ref }}
    go test -coverprofile=base-coverage.out ./...
    BASE_COVERAGE=$(go tool cover -func=base-coverage.out | grep total | awk '{print $3}' | sed 's/%//')
    
    git checkout ${{ github.sha }}
    PR_COVERAGE=$(go tool cover -func=merged-coverage.out | grep total | awk '{print $3}' | sed 's/%//')
    
    DIFF=$(echo "$PR_COVERAGE - $BASE_COVERAGE" | bc)
    
    echo "Base branch coverage: $BASE_COVERAGE%"
    echo "PR coverage: $PR_COVERAGE%"
    echo "Difference: $DIFF%"
    
    if (( $(echo "$DIFF < 0" | bc -l) )); then
      echo "Warning: Coverage decreased by ${DIFF#-}%"
    else
      echo "Coverage increased by $DIFF%"
    fi
```

### 4. Coverage Badges and Reporting

This pattern generates coverage badges for repositories:

```yaml
# GitHub Actions example
- name: Create coverage badge
  uses: schneegans/dynamic-badges-action@v1.6.0
  with:
    auth: ${{ secrets.GIST_SECRET }}
    gistID: ${{ secrets.GIST_ID }}
    filename: coverage.json
    label: coverage
    message: ${{ steps.coverage.outputs.percentage }}
    color: ${{ steps.coverage.outputs.color }}
```

### 5. Custom Docker Coverage Images

Create specialized Docker images for coverage collection:

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .
RUN go build -o /go/bin/coverage-collector ./cmd/coverage-collector

FROM alpine:latest

COPY --from=builder /go/bin/coverage-collector /usr/local/bin/
COPY scripts/coverage-entrypoint.sh /usr/local/bin/

ENTRYPOINT ["coverage-entrypoint.sh"]
```

## Best Practices for CI/CD Integration

1. **Parallel Testing**: Run different types of tests in parallel to save time
2. **Artifact Sharing**: Use CI artifacts to share coverage data between jobs
3. **Consistent Environments**: Ensure Docker images and sandbox configurations match between development and CI
4. **Fail Fast**: Run standard tests before more complex Docker/sandbox tests
5. **Coverage Trending**: Track coverage metrics over time to identify trends
6. **Synthetic Coverage Validation**: Validate that synthetic coverage matches actual code paths
7. **PR Integration**: Add coverage information to pull request comments
8. **Documentation**: Document the coverage approach in the repository
9. **Incremental Analysis**: Only analyze changed files in PRs for faster feedback
10. **Scheduled Full Analysis**: Run complete coverage analysis on a schedule