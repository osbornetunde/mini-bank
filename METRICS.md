# Metrics and Monitoring Guide

This guide explains how to access and use the Prometheus metrics endpoint.

## Accessing Metrics

The `/metrics` endpoint exposes Prometheus-compatible metrics for monitoring the mini-bank application.

### Authentication Required

**Security Note:** The metrics endpoint is protected with HTTP Basic Authentication to prevent unauthorized access to system performance data.

### Credentials Setup

Set metrics credentials in your `.env` file:

```bash
METRICS_USERNAME=metrics
METRICS_PASSWORD=your_secure_password_here
```

Generate a secure password:
```bash
openssl rand -base64 16
```

### Access Methods

#### Using cURL

```bash
curl -u metrics:your_password http://localhost:8080/metrics
```

#### Using Web Browser

Navigate to `http://localhost:8080/metrics` and enter credentials when prompted:
- **Username**: `metrics`
- **Password**: Your configured password

#### Using Prometheus

Configure Prometheus to scrape with Basic Auth in `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'mini-bank'
    static_configs:
      - targets: ['app:8080']
    basic_auth:
      username: 'metrics'
      password: 'your_password_here'
    metrics_path: '/metrics'
    scrape_interval: 15s
```

**Production:** Use Prometheus secrets management instead of plaintext passwords.

## Available Metrics

### HTTP Metrics

**`http_requests_total`** - Counter
- Total number of HTTP requests
- Labels: `method`, `path`, `status`
- Example: `http_requests_total{method="GET",path="/api/v1/accounts",status="200"} 42`

**`http_request_duration_seconds`** - Histogram
- Duration of HTTP requests in seconds
- Labels: `method`, `path`
- Buckets: Default Prometheus buckets (0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10)

### Banking Transaction Metrics

**`bank_transactions_total`** - Counter
- Total number of banking transactions
- Labels: `type` (transfer, deposit, withdraw), `status` (success, failure)
- Example: `bank_transactions_total{type="transfer",status="success"} 156`

**`bank_transaction_amount_total`** - Counter
- Total amount of money moved in transactions
- Labels: `type` (transfer, deposit, withdraw)
- Unit: Cents
- Example: `bank_transaction_amount_total{type="deposit"} 1500000` (= $15,000)

### Audit Log Metrics

**`audit_log_failures_total`** - Counter
- Total number of failed audit log writes
- Labels: `action` (user_created, login_success, login_failed, password_reset_requested, password_reset_success)
- Example: `audit_log_failures_total{action="login_success"} 2`
- **Critical**: Any non-zero value indicates a security concern - audit logs are not being written

### Security Metrics

**`account_lockouts_total`** - Counter
- Total number of account lockouts due to failed login attempts
- No labels
- Example: `account_lockouts_total 15`
- **Important**: Spikes may indicate brute force attacks

**`failed_login_attempts_total`** - Counter
- Total number of failed login attempts (all users)
- No labels
- Example: `failed_login_attempts_total 42`
- **Important**: High rates may indicate credential stuffing or brute force attacks

**`rejected_transactions_total`** - Counter
- Total number of transactions rejected due to validation failures
- Labels: `reason` (amount_exceeds_limit), `type` (transfer, deposit, withdraw)
- Example: `rejected_transactions_total{reason="amount_exceeds_limit",type="transfer"} 15`
- **Important**: Spikes may indicate fraud attempts or system issues

## Example Queries

### PromQL Examples

```promql
# Request rate (requests per second)
rate(http_requests_total[5m])

# Error rate (4xx and 5xx responses)
rate(http_requests_total{status=~"4..|5.."}[5m])

# 95th percentile request duration
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))

# Transaction success rate
sum(rate(bank_transactions_total{status="success"}[5m]))
/
sum(rate(bank_transactions_total[5m]))

# Total money transferred in last hour
increase(bank_transaction_amount_total{type="transfer"}[1h])

# Total audit log failures
sum(audit_log_failures_total)

# Audit log failures by action type
sum by (action) (audit_log_failures_total)

# Failed login rate (per second)
rate(failed_login_attempts_total[5m])

# Account lockout rate (per hour)
increase(account_lockouts_total[1h])

# Failed login attempt ratio (failures per total logins)
rate(failed_login_attempts_total[5m]) / (rate(failed_login_attempts_total[5m]) + rate(http_requests_total{path="/api/v1/login",status="200"}[5m]))

# Total rejected transactions
sum(rejected_transactions_total)

# Rejected transactions by type
sum by (type) (rejected_transactions_total{reason="amount_exceeds_limit"})

# Rejection rate (rejections per second)
rate(rejected_transactions_total[5m])
```

## Grafana Dashboard

### Import Mini-Bank Dashboard

Create a Grafana dashboard with these panels:

1. **Request Rate** - Line graph of `rate(http_requests_total[5m])`
2. **Response Time** - Histogram of request duration percentiles
3. **Error Rate** - Graph showing 4xx/5xx responses
4. **Transaction Volume** - Transaction count by type
5. **Money Flow** - Total amounts by transaction type

### Sample Panel Configuration

```json
{
  "targets": [
    {
      "expr": "rate(http_requests_total[5m])",
      "legendFormat": "{{method}} {{path}}"
    }
  ],
  "title": "HTTP Request Rate",
  "type": "graph"
}
```

## Alerting

### Recommended Alerts

#### High Error Rate
```yaml
- alert: HighErrorRate
  expr: |
    sum(rate(http_requests_total{status=~"5.."}[5m]))
    /
    sum(rate(http_requests_total[5m]))
    > 0.05
  for: 5m
  annotations:
    summary: "High error rate detected"
    description: "Error rate is {{ $value | humanizePercentage }}"
```

#### Slow Requests
```yaml
- alert: SlowRequests
  expr: |
    histogram_quantile(0.95,
      rate(http_request_duration_seconds_bucket[5m])
    ) > 1
  for: 10m
  annotations:
    summary: "95th percentile request duration > 1s"
```

#### High Transaction Failure Rate
```yaml
- alert: HighTransactionFailureRate
  expr: |
    sum(rate(bank_transactions_total{status="failure"}[5m]))
    /
    sum(rate(bank_transactions_total[5m]))
    > 0.1
  for: 5m
  annotations:
    summary: "More than 10% of transactions failing"
```

#### Audit Log Failures (Critical)
```yaml
- alert: AuditLogFailures
  expr: |
    increase(audit_log_failures_total[5m]) > 0
  for: 1m
  annotations:
    summary: "Audit log writes are failing"
    description: "{{ $value }} audit log writes failed in the last 5 minutes for action {{ $labels.action }}"
  labels:
    severity: critical
```

**Important**: Audit log failures are a critical security issue. Any failures should trigger immediate investigation as they indicate:
- Database connectivity issues
- Disk space problems
- Potential security event tampering
- System instability

#### Account Lockout Spike (Security)
```yaml
- alert: AccountLockoutSpike
  expr: |
    increase(account_lockouts_total[15m]) > 10
  for: 1m
  annotations:
    summary: "Unusual number of account lockouts detected"
    description: "{{ $value }} accounts locked in last 15 minutes - possible brute force attack"
  labels:
    severity: warning
```

#### High Failed Login Rate (Security)
```yaml
- alert: HighFailedLoginRate
  expr: |
    rate(failed_login_attempts_total[5m]) > 1
  for: 5m
  annotations:
    summary: "High rate of failed login attempts"
    description: "{{ $value }} failed logins per second - possible credential stuffing attack"
  labels:
    severity: warning
```

**Security Notes**:
- Account lockouts protect against brute force attacks
- Each account locks after 5 failed attempts for 15 minutes
- Monitor for patterns indicating distributed attacks (many different accounts)
- High lockout rates may indicate credential stuffing or password spray attacks

#### High Transaction Rejection Rate (Security)
```yaml
- alert: HighTransactionRejectionRate
  expr: |
    rate(rejected_transactions_total{reason="amount_exceeds_limit"}[5m]) > 0.1
  for: 5m
  annotations:
    summary: "High rate of rejected transactions due to amount limits"
    description: "{{ $value }} transactions per second rejected for exceeding amount limits"
  labels:
    severity: warning
```

**Fraud Detection Notes**:
- Transaction amount limits prevent large-scale fraud
- Rejections may indicate:
  - Legitimate users hitting limits (need limit adjustment)
  - Fraud attempts (trying to move large amounts)
  - Fat-finger errors (accidental extra zeros)
  - System bugs (calculating wrong amounts)
- Investigate accounts with repeated rejections

## Security Considerations

### Why Basic Auth?

- ✅ Simple for Prometheus to consume
- ✅ Standard HTTP authentication
- ✅ Works with all monitoring tools
- ✅ Easy to configure and rotate

### Best Practices

1. **Use Strong Passwords**
   ```bash
   # Generate 20+ character password
   openssl rand -base64 32
   ```

2. **Rotate Credentials Regularly**
   - Update `METRICS_PASSWORD` every 90 days
   - Update Prometheus config after rotation

3. **Restrict Network Access**
   ```yaml
   # docker-compose.yml - Don't expose metrics externally
   # Only Prometheus should access it via Docker network
   ```

4. **Monitor Access**
   - Review logs for unauthorized access attempts
   - Set up alerts for repeated 401 errors

5. **Use HTTPS in Production**
   ```bash
   # Always use TLS for metrics in production
   https://mini-bank.example.com/metrics
   ```

## Troubleshooting

### 401 Unauthorized

**Problem:** `curl http://localhost:8080/metrics` returns 401

**Solution:**
```bash
# Provide credentials
curl -u metrics:your_password http://localhost:8080/metrics
```

### 503 Service Unavailable

**Problem:** "Metrics authentication not configured"

**Solution:** Set environment variables:
```bash
export METRICS_USERNAME=metrics
export METRICS_PASSWORD=$(openssl rand -base64 16)
docker-compose restart app
```

### Prometheus Can't Scrape

**Problem:** Prometheus shows target as DOWN

**Solutions:**
1. Check credentials in `prometheus.yml`
2. Verify network connectivity: `docker-compose exec prometheus ping app`
3. Test manually: `curl -u metrics:pass http://app:8080/metrics`
4. Check logs: `docker-compose logs app | grep metrics`

### Empty Metrics

**Problem:** Metrics endpoint returns no data

**Solution:** Metrics are only generated after requests. Make some API calls first:
```bash
curl http://localhost:8080/health
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/accounts
```

## Production Setup

### Complete Prometheus Stack

```yaml
# docker-compose.prometheus.yml
version: '3.8'

services:
  prometheus:
    image: prom/prometheus:latest
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    ports:
      - "9090:9090"
    networks:
      - monitoring

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=your_secure_password
    volumes:
      - grafana_data:/var/lib/grafana
    networks:
      - monitoring

volumes:
  prometheus_data:
  grafana_data:

networks:
  monitoring:
```

### Prometheus Configuration

```yaml
# prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'mini-bank'
    static_configs:
      - targets: ['app:8080']
    basic_auth:
      username: 'metrics'
      password_file: '/run/secrets/metrics_password'
    metrics_path: '/metrics'
```

---

**Happy Monitoring!** 📊

For more information on Prometheus, visit: https://prometheus.io/docs/
