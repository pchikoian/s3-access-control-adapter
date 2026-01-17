#!/bin/bash

# Comprehensive test runner
# Runs unit tests, builds the project, and optionally runs integration tests

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Parse arguments
RUN_INTEGRATION=false
RUN_COVERAGE=false
VERBOSE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -i|--integration)
            RUN_INTEGRATION=true
            shift
            ;;
        -c|--coverage)
            RUN_COVERAGE=true
            shift
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -i, --integration    Run integration tests (requires Docker)"
            echo "  -c, --coverage       Generate coverage report"
            echo "  -v, --verbose        Verbose output"
            echo "  -h, --help           Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                   # Run unit tests only"
            echo "  $0 -i                # Run unit + integration tests"
            echo "  $0 -c                # Run unit tests with coverage"
            echo "  $0 -i -c             # Run all tests with coverage"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use -h or --help for usage information"
            exit 1
            ;;
    esac
done

echo -e "${BLUE}=========================================="
echo "S3 Access Control Gateway Test Suite"
echo "==========================================${NC}"
echo ""

# Step 1: Format check
echo -e "${BLUE}[1/4] Checking code formatting...${NC}"
if go fmt ./... | grep -q .; then
    echo -e "${YELLOW}Warning: Code formatting issues found. Run 'make fmt' to fix.${NC}"
else
    echo -e "${GREEN}✓ Code formatting OK${NC}"
fi
echo ""

# Step 2: Unit tests
if [ "$RUN_COVERAGE" = true ]; then
    echo -e "${BLUE}[2/4] Running unit tests with coverage...${NC}"
    if [ "$VERBOSE" = true ]; then
        go test -v -coverprofile=coverage.out ./...
    else
        go test -coverprofile=coverage.out ./...
    fi

    # Generate coverage report
    go tool cover -func=coverage.out | tail -n 1

    # Generate HTML coverage report
    go tool cover -html=coverage.out -o coverage.html
    echo -e "${GREEN}✓ Coverage report generated: coverage.html${NC}"
else
    echo -e "${BLUE}[2/4] Running unit tests...${NC}"
    if [ "$VERBOSE" = true ]; then
        go test -v ./...
    else
        go test ./...
    fi
    echo -e "${GREEN}✓ Unit tests passed${NC}"
fi
echo ""

# Step 3: Build
echo -e "${BLUE}[3/4] Building project...${NC}"
if make build > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Build successful${NC}"
else
    echo -e "${RED}✗ Build failed${NC}"
    exit 1
fi
echo ""

# Step 4: Integration tests (optional)
if [ "$RUN_INTEGRATION" = true ]; then
    echo -e "${BLUE}[4/4] Running integration tests...${NC}"

    # Check if Docker is running
    if ! docker info > /dev/null 2>&1; then
        echo -e "${RED}✗ Docker is not running${NC}"
        echo "Please start Docker and run 'make docker-up' before running integration tests"
        exit 1
    fi

    # Detect docker compose command
    if command -v docker-compose &> /dev/null; then
        DOCKER_COMPOSE="docker-compose"
    else
        DOCKER_COMPOSE="docker compose"
    fi

    # Check if services are running
    if ! $DOCKER_COMPOSE ps 2>/dev/null | grep -q "Up"; then
        echo -e "${YELLOW}Starting Docker services...${NC}"
        make docker-up
        echo "Waiting for services to be ready..."
        sleep 10
    fi

    # Run smoke test first
    echo "Running smoke test..."
    if ./scripts/test-smoke.sh; then
        echo -e "${GREEN}✓ Smoke test passed${NC}"
    else
        echo -e "${RED}✗ Smoke test failed${NC}"
        exit 1
    fi

    # Run full integration tests
    echo ""
    echo "Running full integration test suite..."
    if ./scripts/test-integration.sh; then
        echo -e "${GREEN}✓ Integration tests passed${NC}"
    else
        echo -e "${RED}✗ Integration tests failed${NC}"
        exit 1
    fi
else
    echo -e "${YELLOW}[4/4] Integration tests skipped (use -i to run)${NC}"
fi

echo ""
echo -e "${GREEN}=========================================="
echo "✓ All tests passed!"
echo "==========================================${NC}"

if [ "$RUN_COVERAGE" = true ]; then
    echo ""
    echo "Coverage report: coverage.html"
fi

if [ "$RUN_INTEGRATION" = false ]; then
    echo ""
    echo "Tip: Run with -i flag to include integration tests"
fi
