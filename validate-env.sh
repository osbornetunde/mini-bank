#!/bin/bash

# Script to validate environment variables are properly set before starting Docker

set -e

echo "🔍 Validating environment variables..."
echo ""

# Check if .env file exists
if [ ! -f .env ]; then
    echo "❌ ERROR: .env file not found!"
    echo ""
    echo "Please create a .env file:"
    echo "  cp .env.docker.example .env"
    echo ""
    echo "Then edit .env and set your secrets."
    exit 1
fi

# Source the .env file
set -a
source .env
set +a

# Validation flags
ERRORS=0

# Required variables
echo "Checking required variables..."

if [ -z "$JWT_SECRET" ]; then
    echo "❌ JWT_SECRET is not set"
    echo "   Generate with: openssl rand -base64 32"
    ERRORS=$((ERRORS + 1))
else
    if [ ${#JWT_SECRET} -lt 32 ]; then
        echo "⚠️  WARNING: JWT_SECRET is too short (less than 32 characters)"
        echo "   Generate a stronger secret with: openssl rand -base64 32"
    else
        echo "✅ JWT_SECRET is set"
    fi
fi

if [ -z "$POSTGRES_PASSWORD" ]; then
    echo "❌ POSTGRES_PASSWORD is not set"
    echo "   Generate with: openssl rand -base64 24"
    ERRORS=$((ERRORS + 1))
else
    if [ ${#POSTGRES_PASSWORD} -lt 16 ]; then
        echo "⚠️  WARNING: POSTGRES_PASSWORD is too short (less than 16 characters)"
        echo "   Generate a stronger password with: openssl rand -base64 24"
    else
        echo "✅ POSTGRES_PASSWORD is set"
    fi
fi

if [ -z "$METRICS_USERNAME" ]; then
    echo "❌ METRICS_USERNAME is not set"
    echo "   Set to: metrics (or any username)"
    ERRORS=$((ERRORS + 1))
else
    echo "✅ METRICS_USERNAME is set"
fi

if [ -z "$METRICS_PASSWORD" ]; then
    echo "❌ METRICS_PASSWORD is not set"
    echo "   Generate with: openssl rand -base64 16"
    ERRORS=$((ERRORS + 1))
else
    if [ ${#METRICS_PASSWORD} -lt 12 ]; then
        echo "⚠️  WARNING: METRICS_PASSWORD is too short (less than 12 characters)"
        echo "   Generate a stronger password with: openssl rand -base64 16"
    else
        echo "✅ METRICS_PASSWORD is set"
    fi
fi

echo ""
echo "Checking optional variables..."

[ -n "$POSTGRES_USER" ] && echo "✅ POSTGRES_USER is set" || echo "ℹ️  POSTGRES_USER not set (will use default: bank)"
[ -n "$POSTGRES_DB" ] && echo "✅ POSTGRES_DB is set" || echo "ℹ️  POSTGRES_DB not set (will use default: bank)"
[ -n "$REDIS_ADDR" ] && echo "✅ REDIS_ADDR is set" || echo "ℹ️  REDIS_ADDR not set (will use default: redis:6379)"

echo ""
echo "Checking SMTP variables (optional)..."
[ -n "$SMTP_HOST" ] && echo "✅ SMTP_HOST is set" || echo "ℹ️  SMTP_HOST not set"
[ -n "$SMTP_PORT" ] && echo "✅ SMTP_PORT is set" || echo "ℹ️  SMTP_PORT not set"
[ -n "$SMTP_USERNAME" ] && echo "✅ SMTP_USERNAME is set" || echo "ℹ️  SMTP_USERNAME not set"
[ -n "$SMTP_PASSWORD" ] && echo "✅ SMTP_PASSWORD is set" || echo "ℹ️  SMTP_PASSWORD not set"
[ -n "$SMTP_SENDER" ] && echo "✅ SMTP_SENDER is set" || echo "ℹ️  SMTP_SENDER not set"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ $ERRORS -eq 0 ]; then
    echo "✅ All required environment variables are set!"
    echo ""
    echo "You can now start Docker:"
    echo "  docker-compose up -d"
    exit 0
else
    echo "❌ Found $ERRORS error(s). Please fix them before starting Docker."
    exit 1
fi
