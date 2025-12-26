# Docker Setup Guide

This guide explains how to run the mini-bank application using Docker and Docker Compose with proper security practices.

## ⚠️ Important Security Notice

**NEVER commit secrets to git!** This project uses environment variables to keep sensitive information secure.

## Quick Start

### 1. Create Environment File

Copy the example environment file and fill in your secrets:

```bash
cp .env.docker.example .env
```

### 2. Generate Secure Secrets

**JWT Secret** (Required):
```bash
# Generate a secure random JWT secret
openssl rand -base64 32
```

Copy the output and set it as `JWT_SECRET` in your `.env` file.

**Database Password** (Required):
```bash
# Generate a secure database password
openssl rand -base64 24
```

Copy the output and set it as `POSTGRES_PASSWORD` in your `.env` file.

### 3. Configure Your .env File

Edit `.env` and set at minimum these required variables:

```bash
# REQUIRED
POSTGRES_PASSWORD=your_generated_password_here
JWT_SECRET=your_generated_jwt_secret_here
METRICS_USERNAME=metrics
METRICS_PASSWORD=your_generated_metrics_password_here

# OPTIONAL (have defaults)
POSTGRES_USER=bank
POSTGRES_DB=bank
REDIS_ADDR=redis:6379

# OPTIONAL (for email functionality)
SMTP_HOST=sandbox.smtp.mailtrap.io
SMTP_PORT=2525
SMTP_USERNAME=your_smtp_username
SMTP_PASSWORD=your_smtp_password
SMTP_SENDER=noreply@minibank.example.com
```

### 4. Start the Services

```bash
docker-compose up -d
```

This will start:
- **app**: The mini-bank application on port 8080
- **db**: PostgreSQL database on port 5432
- **redis**: Redis cache on port 6379

### 5. Check Service Status

```bash
# View running containers
docker-compose ps

# View logs
docker-compose logs -f app

# View all logs
docker-compose logs -f
```

### 6. Access the Application

- **Application**: http://localhost:8080
- **Health check**: http://localhost:8080/health (public, no auth required)
- **Metrics**: http://localhost:8080/metrics (requires Basic Auth)

To access metrics with authentication:
```bash
curl -u metrics:your_metrics_password http://localhost:8080/metrics
```

Or in your browser, you'll be prompted for username and password when accessing the metrics endpoint.

## Running Migrations

Before using the application, run database migrations:

```bash
# Connect to the app container
docker-compose exec app sh

# Inside the container, run migrations
# (You'll need to install migrate tool or run it from host)
```

Alternatively, run migrations from your host machine:

```bash
# Make sure migrate is installed
# brew install golang-migrate (macOS)
# or see: https://github.com/golang-migrate/migrate

migrate -path migrations -database "postgres://bank:YOUR_PASSWORD@localhost:5432/bank?sslmode=disable" up
```

Replace `YOUR_PASSWORD` with the value from your `.env` file.

## Environment Variables Reference

### Required Variables

| Variable | Description | How to Generate |
|----------|-------------|-----------------|
| `JWT_SECRET` | Secret key for JWT token signing | `openssl rand -base64 32` |
| `POSTGRES_PASSWORD` | PostgreSQL database password | `openssl rand -base64 24` |
| `METRICS_USERNAME` | Username for /metrics endpoint | Any string (e.g., `metrics`) |
| `METRICS_PASSWORD` | Password for /metrics endpoint | `openssl rand -base64 16` |

### Optional Variables (with defaults)

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTGRES_USER` | `bank` | PostgreSQL username |
| `POSTGRES_DB` | `bank` | PostgreSQL database name |
| `REDIS_ADDR` | `redis:6379` | Redis connection address |
| `SMTP_HOST` | - | SMTP server hostname |
| `SMTP_PORT` | - | SMTP server port |
| `SMTP_USERNAME` | - | SMTP authentication username |
| `SMTP_PASSWORD` | - | SMTP authentication password |
| `SMTP_SENDER` | - | Email sender address |
| `TRUST_PROXY` | `false` | Trust X-Forwarded-For headers from reverse proxy |

## Reverse Proxy Configuration

### When to Enable TRUST_PROXY

Set `TRUST_PROXY=true` when your application is running behind a reverse proxy or load balancer, such as:

- **nginx** reverse proxy
- **AWS Application Load Balancer (ALB)** or Network Load Balancer
- **Cloudflare** CDN
- **Google Cloud Load Balancer**
- **Azure Application Gateway**
- Any other reverse proxy that sets X-Forwarded-For headers

### Why This Matters

When `TRUST_PROXY=false` (default):
- The app uses `RemoteAddr` for client IP addresses
- Behind a proxy, this logs the proxy's IP, not the actual client IP
- Rate limiting applies to the proxy IP instead of individual clients
- Audit logs show proxy IP instead of real users

When `TRUST_PROXY=true`:
- The app parses X-Forwarded-For headers to extract the real client IP
- Supports multiple proxy header formats:
  - `X-Forwarded-For` (standard, takes leftmost/first IP)
  - `X-Real-IP` (nginx, some proxies)
  - `CF-Connecting-IP` (Cloudflare)
- Rate limiting works per actual client
- Audit logs show accurate user IPs

### Security Warning

⚠️ **ONLY enable TRUST_PROXY if you have a trusted reverse proxy!**

If you enable `TRUST_PROXY=true` without a reverse proxy, attackers can spoof their IP address by setting the X-Forwarded-For header directly, bypassing rate limiting and audit logging.

**Safe scenarios:**
- Application is in a private network behind a load balancer
- Cloud environment with managed proxy (AWS ALB, GCP LB, Azure AG)
- nginx reverse proxy you control

**Unsafe scenarios:**
- Application is directly exposed to the internet
- You're not sure if there's a proxy in front
- Using an untrusted or misconfigured proxy

### Configuration Examples

**Development (no proxy):**
```bash
TRUST_PROXY=false  # Default - use RemoteAddr
```

**Production behind nginx:**
```bash
TRUST_PROXY=true
```

**Production behind AWS ALB:**
```bash
TRUST_PROXY=true
```

**Production behind Cloudflare:**
```bash
TRUST_PROXY=true
```

### Testing Your Configuration

**Without proxy (TRUST_PROXY=false):**
```bash
# Make a request
curl http://localhost:8080/health

# Check logs - should show your actual IP or 127.0.0.1
docker-compose logs app | grep "processed request"
```

**With proxy (TRUST_PROXY=true):**
```bash
# Simulate proxy header
curl -H "X-Forwarded-For: 203.0.113.1" http://localhost:8080/health

# Check logs - should show 203.0.113.1
docker-compose logs app | grep "processed request"
```

**Verify rate limiting uses correct IP:**
```bash
# Make multiple requests from same IP
for i in {1..10}; do
  curl -H "X-Forwarded-For: 203.0.113.1" http://localhost:8080/api/v1/login
done

# All requests should be rate-limited as the same client (203.0.113.1)
```

### nginx Configuration Example

If you're using nginx as a reverse proxy:

```nginx
server {
    listen 80;
    server_name minibank.example.com;

    location / {
        proxy_pass http://app:8080;

        # Set proxy headers
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Then set `TRUST_PROXY=true` in your mini-bank `.env` file.

## Security Best Practices

### ✅ DO

- Use strong, randomly generated secrets
- Keep your `.env` file secure (it's in `.gitignore`)
- Use different secrets for development and production
- Rotate secrets periodically
- Use environment-specific `.env` files (`.env.production`, `.env.staging`)

### ❌ DON'T

- Commit `.env` files to git
- Share secrets in plain text (email, Slack, etc.)
- Reuse secrets across environments
- Use weak or predictable secrets
- Copy production secrets to development

## Stopping Services

```bash
# Stop services but keep volumes
docker-compose down

# Stop services and remove volumes (deletes database data!)
docker-compose down -v
```

## Troubleshooting

### Missing JWT_SECRET Error

If you see `JWT_SECRET is required`:
1. Make sure `.env` file exists
2. Verify `JWT_SECRET` is set in `.env`
3. Restart docker-compose: `docker-compose down && docker-compose up -d`

### Database Connection Error

If the app can't connect to the database:
1. Check database is running: `docker-compose ps`
2. Verify `POSTGRES_PASSWORD` matches in both app and db sections
3. Check logs: `docker-compose logs db`

### Port Already in Use

If ports 8080, 5432, or 6379 are already in use:
1. Stop conflicting services
2. Or modify ports in `docker-compose.yml`:
   ```yaml
   ports:
     - "8081:8080"  # Use port 8081 instead
   ```

## Production Deployment

For production environments:

1. **Use Docker Secrets** (Docker Swarm) or **Kubernetes Secrets**
2. **Never use .env files in production** - use your orchestrator's secret management
3. **Enable TLS/SSL** - configure HTTPS
4. **Use managed database** - don't run PostgreSQL in Docker for production
5. **Set up monitoring** - configure Prometheus and alerting
6. **Implement backups** - regular database backups
7. **Use read-only filesystem** - add `read_only: true` to containers
8. **Scan for vulnerabilities** - use tools like Trivy or Snyk

## Development vs Production

This Docker setup is designed for **development and testing**.

For production, consider:
- Kubernetes with sealed secrets
- AWS ECS with Secrets Manager
- Cloud provider secret management (GCP Secret Manager, Azure Key Vault)
- HashiCorp Vault
- Docker Swarm secrets

---

**Need help?** Check the main README.md or open an issue.
