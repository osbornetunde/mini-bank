# Security Guidelines

This document outlines security best practices for the mini-bank application.

## 🔐 Secrets Management

### Docker & Docker Compose

**CRITICAL:** Never commit secrets to git!

1. **Development**: Use `.env` file (already in `.gitignore`)
   ```bash
   cp .env.docker.example .env
   # Edit .env and add your secrets
   ```

2. **Production**: Use proper secret management
   - **Kubernetes**: Use Sealed Secrets or external secret operators
   - **Docker Swarm**: Use Docker Secrets
   - **Cloud Providers**:
     - AWS: Use AWS Secrets Manager or Parameter Store
     - GCP: Use Secret Manager
     - Azure: Use Key Vault

### Secret Generation

Always use cryptographically secure random generation:

```bash
# JWT Secret (256-bit minimum)
openssl rand -base64 32

# Database Password
openssl rand -base64 24

# API Keys
openssl rand -hex 32
```

**Never use:**
- Predictable values ("password123", "secret", etc.)
- Short secrets (< 16 characters)
- Secrets from online generators (they may be logged)
- Reused secrets across environments

## 🛡️ Security Checklist

### Before Deploying

- [ ] All secrets are randomly generated
- [ ] `.env` file is in `.gitignore`
- [ ] No hardcoded secrets in code or config files
- [ ] Different secrets for dev/staging/production
- [ ] HTTPS/TLS is enabled
- [ ] Database uses strong password
- [ ] JWT secret is at least 32 characters
- [ ] SMTP credentials are secured
- [ ] Metrics endpoint credentials are set
- [ ] TRUST_PROXY correctly configured (true if behind proxy, false otherwise)

### Production Requirements

- [ ] Use managed secret storage (not `.env` files)
- [ ] Enable rate limiting on all endpoints
- [ ] Use HTTPS only (no HTTP)
- [ ] Enable HSTS header (uncomment in middleware.go)
- [ ] Enable CORS with strict origin rules
- [ ] Use managed database service with encryption at rest
- [ ] Enable database connection SSL/TLS
- [ ] Set up WAF (Web Application Firewall)
- [ ] Enable DDoS protection
- [ ] Implement proper logging and monitoring
- [ ] Set up security alerts
- [ ] Regular security audits and penetration testing
- [ ] Keep dependencies updated (automated scanning)

### API Security

- [ ] All sensitive endpoints require authentication
- [ ] Use HTTPS for all API calls
- [ ] Implement rate limiting
- [ ] Validate all input
- [ ] Use parameterized queries (prevent SQL injection)
- [ ] Sanitize user input (prevent XSS)
- [ ] Set proper CORS headers
- [ ] Use secure session management
- [ ] Implement CSRF protection for web interfaces

## 🔒 Current Security Features

### Implemented ✅

1. **Authentication**
   - JWT-based authentication
   - Secure password hashing (bcrypt)
   - Session management with Redis
   - Token refresh mechanism

2. **Authorization**
   - User-based access control
   - Account ownership verification
   - Protected API endpoints

3. **Input Validation**
   - Request body validation
   - Password strength requirements
   - Amount validation (min/max limits)
   - Email format validation

4. **Rate Limiting**
   - Password reset endpoints (5 requests/hour)
   - Prevents brute force attacks

5. **Audit Logging**
   - User actions tracked
   - IP address logging
   - User agent logging
   - Login attempt tracking

6. **Secure Defaults**
   - Overdraft limits with maximum caps
   - Transaction amount limits
   - No default credentials

7. **Metrics Protection**
   - HTTP Basic Authentication on `/metrics` endpoint
   - Constant-time comparison (prevents timing attacks)
   - Configurable credentials via environment variables

8. **Reverse Proxy Support**
   - X-Forwarded-For header parsing (opt-in via TRUST_PROXY)
   - Support for nginx, AWS ALB, Cloudflare
   - Accurate IP logging behind load balancers

9. **Audit Log Error Handling**
   - Structured logging of audit failures
   - Prometheus metrics for failed audit writes
   - Monitoring and alerting on audit system health

10. **Account Lockout Protection**
   - Automatic lockout after 5 failed login attempts
   - 15-minute lockout duration
   - Failed attempt tracking per email address
   - Prometheus metrics for monitoring lockout events
   - Automatic reset on successful login

11. **HTTP Security Headers**
   - X-Frame-Options: DENY (prevents clickjacking)
   - X-Content-Type-Options: nosniff (prevents MIME sniffing)
   - X-XSS-Protection: enabled (legacy XSS protection)
   - Content-Security-Policy: restrictive (limits resource loading)
   - Referrer-Policy: no-referrer (privacy protection)
   - Permissions-Policy: restricts browser features
   - HSTS ready for HTTPS deployments

12. **Transaction Amount Limits**
   - Maximum transfer: $1,000,000 per transaction
   - Maximum deposit: $1,000,000 per transaction
   - Maximum withdrawal: $100,000 per transaction
   - Prevents fraud, money laundering, and fat-finger errors
   - Prometheus metrics for rejected transactions

### Known Limitations ⚠️

Based on security review, these items have been addressed or need attention:

## 🛡️ Security Headers Explained

The application sets the following HTTP security headers on all responses:

### X-Frame-Options: DENY
**Prevents:** Clickjacking attacks
**What it does:** Prevents the page from being loaded in an iframe, frame, or object tag
**Why:** Attackers can't trick users by overlaying invisible iframes over legitimate UI elements

### X-Content-Type-Options: nosniff
**Prevents:** MIME-sniffing attacks
**What it does:** Forces browsers to respect the Content-Type header
**Why:** Prevents browsers from interpreting files as a different MIME type than declared, which could execute malicious scripts

### X-XSS-Protection: 1; mode=block
**Prevents:** Reflected XSS attacks
**What it does:** Enables browser's built-in XSS filter
**Why:** Provides legacy protection for older browsers (modern browsers rely on CSP)

### Content-Security-Policy
**Prevents:** XSS, injection attacks, unauthorized resource loading
**What it does:** Restricts what resources the browser can load
**Current policy:** `default-src 'none'; frame-ancestors 'none'`
**Why:** Since this is an API, we don't load any resources. This prevents any malicious script injection.

### Referrer-Policy: no-referrer
**Prevents:** Information leakage
**What it does:** Prevents sending referrer information to other sites
**Why:** Protects user privacy and prevents sensitive URL parameters from leaking

### Permissions-Policy
**Prevents:** Unnecessary browser feature access
**What it does:** Disables geolocation, microphone, and camera access
**Why:** API doesn't need these features; restricting them reduces attack surface

### Strict-Transport-Security (HSTS)
**Prevents:** Man-in-the-middle attacks, protocol downgrade attacks
**What it does:** Forces browsers to use HTTPS for all future requests
**Status:** Commented out by default (enable when serving over HTTPS)
**Production setup:**
```go
// Uncomment in production with HTTPS:
w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
```

### Testing Security Headers

You can verify that security headers are properly set using curl:

```bash
# Test security headers on any endpoint
curl -I http://localhost:8080/health

# Expected headers in response:
# X-Frame-Options: DENY
# X-Content-Type-Options: nosniff
# X-XSS-Protection: 1; mode=block
# Content-Security-Policy: default-src 'none'; frame-ancestors 'none'
# Referrer-Policy: no-referrer
# Permissions-Policy: geolocation=(), microphone=(), camera=()
```

Or use online security header scanners:
- [securityheaders.com](https://securityheaders.com)
- [Mozilla Observatory](https://observatory.mozilla.org)

**Production checklist:**
1. Deploy application with HTTPS
2. Uncomment HSTS header in `internal/api/middleware.go`
3. Run security header scanner
4. Verify A+ rating

## 💰 Transaction Amount Limits

The application enforces maximum transaction limits to prevent fraud, money laundering, and accidental errors.

### Current Limits

| Transaction Type | Maximum Amount | Purpose |
|-----------------|----------------|---------|
| Transfer | $1,000,000 | Balance transfer between accounts |
| Deposit | $1,000,000 | Adding funds to account |
| Withdrawal | $100,000 | Removing funds from account |
| Overdraft Limit | $100,000 | Maximum negative balance allowed |

### Why These Limits?

**Fraud Prevention:**
- Limits exposure in case of compromised accounts
- Restricts damage from unauthorized access
- Makes large-scale theft more difficult

**Money Laundering Prevention:**
- Complies with AML (Anti-Money Laundering) regulations
- Forces suspicious large transactions to be split (easier to detect)
- Provides audit trail for high-value movements

**Fat-Finger Protection:**
- Prevents accidental extra zeros ($100 vs $10,000,000)
- Catches data entry errors before execution
- Requires intentional confirmation for large amounts

**Operational Safety:**
- Protects against bugs that might calculate wrong amounts
- Limits impact of potential SQL injection or parameter tampering
- Provides predictable system behavior

### Monitoring

Rejected transactions are tracked via Prometheus metric:
```
rejected_transactions_total{reason="amount_exceeds_limit", type="transfer|deposit|withdraw"}
```

Monitor this metric for:
- Unusual spikes (possible attack or system issue)
- Patterns of rejection (users hitting limits frequently)
- Specific accounts repeatedly rejected (fraud investigation)

### Adjusting Limits

To modify transaction limits, edit `internal/api/validation.go`:

```go
const (
    maxTransferAmount  = 1_000_000_00 // $1,000,000 in cents
    maxDepositAmount   = 1_000_000_00 // $1,000,000 in cents
    maxWithdrawAmount  = 100_000_00   // $100,000 in cents
)
```

**Note:** Changes require application restart. Consider your regulatory requirements before increasing limits.

## 🔑 Password Policy

### User Passwords

Requirements enforced by API validation:
- Minimum 8 characters
- Maximum 128 characters
- Must contain:
  - At least one uppercase letter
  - At least one lowercase letter
  - At least one digit
  - At least one special character

### Database Passwords

Recommendations:
- Minimum 16 characters
- Random alphanumeric + special characters
- Rotate every 90 days in production
- Never reuse old passwords

## 🌐 Network Security

### Development

```yaml
# docker-compose.yml exposes ports for development
ports:
  - "8080:8080"  # Application
  - "5432:5432"  # PostgreSQL - REMOVE IN PRODUCTION
  - "6379:6379"  # Redis - REMOVE IN PRODUCTION
```

### Production

```yaml
# Only expose application port
ports:
  - "8080:8080"

# Database and Redis should NOT be exposed
# They should only be accessible within Docker network
```

### Reverse Proxy Security

When deploying behind a reverse proxy or load balancer, proper configuration is critical for security:

#### TRUST_PROXY Configuration

**Default (TRUST_PROXY=false):**
- App uses `RemoteAddr` for client IP
- Safe for direct internet exposure
- Ignores X-Forwarded-For headers (prevents spoofing)

**Behind Proxy (TRUST_PROXY=true):**
- App parses X-Forwarded-For headers
- **ONLY enable if you have a trusted reverse proxy**
- Required for accurate rate limiting and logging

#### Security Implications

**If TRUST_PROXY=true without a proxy:**
- ❌ Attackers can spoof IP addresses
- ❌ Rate limiting can be bypassed
- ❌ Audit logs show fake IPs
- ❌ Malicious users can frame others

**If TRUST_PROXY=false behind a proxy:**
- ⚠️ All requests show proxy IP
- ⚠️ Rate limiting affects all users as one
- ⚠️ Audit logs don't show real users
- ⚠️ Can't block individual malicious IPs

#### Safe Deployment Patterns

**✅ Correct: TRUST_PROXY=true behind nginx**
```
Internet → nginx (sets X-Forwarded-For) → App (TRUST_PROXY=true)
```

**✅ Correct: TRUST_PROXY=true behind AWS ALB**
```
Internet → AWS ALB (sets X-Forwarded-For) → App (TRUST_PROXY=true)
```

**✅ Correct: TRUST_PROXY=false for direct exposure**
```
Internet → App (TRUST_PROXY=false, ignores headers)
```

**❌ DANGEROUS: TRUST_PROXY=true with direct exposure**
```
Internet → App (TRUST_PROXY=true) ← Attacker can spoof IPs!
```

#### Checklist for Reverse Proxy Setup

- [ ] Verify you have a trusted reverse proxy (nginx, ALB, Cloudflare, etc.)
- [ ] Confirm proxy is properly configured to set X-Forwarded-For
- [ ] Set TRUST_PROXY=true in your .env file
- [ ] Test that client IPs are correctly logged
- [ ] Verify rate limiting works per client (not per proxy)
- [ ] Ensure proxy strips client-provided X-Forwarded-For headers (prevents injection)

#### Testing Your Configuration

```bash
# 1. Make a request and check logs
curl http://your-app.com/health
docker-compose logs app | grep "processed request"

# 2. Verify IP is correct (should be YOUR IP, not proxy IP)

# 3. Test rate limiting
for i in {1..10}; do curl http://your-app.com/api/v1/login; done

# 4. Should see rate limit error after configured limit
```

## 📊 Monitoring & Alerting

### Security Events to Monitor

1. **Failed Login Attempts**
   - Alert on > 5 failures from same IP in 5 minutes
   - Alert on > 10 failures for same user in 1 hour
   - Monitor `failed_login_attempts_total` metric

2. **Account Lockouts**
   - Alert on any account lockout events
   - Monitor `account_lockouts_total` metric
   - Investigate patterns of lockouts (possible brute force attack)
   - Track lockout frequency to detect distributed attacks

3. **Unusual Transaction Patterns**
   - Large withdrawals
   - Rapid successive transactions
   - Transactions near overdraft limits

4. **Audit Log Failures**
   - Any failure to write audit logs
   - Missing audit logs for sensitive operations

5. **API Abuse**
   - Rate limit violations
   - Unusual request patterns
   - Requests to non-existent endpoints

## 🔄 Incident Response

### If Secrets Are Compromised

1. **Immediate Actions**
   - Rotate ALL secrets immediately
   - Invalidate all existing JWT tokens
   - Force password reset for all users
   - Review audit logs for suspicious activity

2. **Investigation**
   - Check git history for exposed secrets
   - Review recent deployments
   - Analyze access logs
   - Identify scope of exposure

3. **Recovery**
   - Deploy with new secrets
   - Notify affected users if personal data exposed
   - Document incident and lessons learned
   - Update security practices

### If Database Is Compromised

1. Take database offline immediately
2. Assess data breach scope
3. Notify users (required by GDPR/CCPA if PII exposed)
4. Restore from clean backup
5. Patch vulnerability
6. Conduct security audit

## 📚 Resources

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [OWASP API Security Top 10](https://owasp.org/www-project-api-security/)
- [Docker Security Best Practices](https://docs.docker.com/engine/security/)
- [Go Security Checklist](https://github.com/OWASP/Go-SCP)

---

**Security is everyone's responsibility. When in doubt, ask!**
