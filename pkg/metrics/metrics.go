package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HttpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests.",
	}, []string{"method", "path", "status"})

	HttpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Duration of HTTP requests.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	TransactionTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bank_transactions_total",
		Help: "Total number of banking transactions.",
	}, []string{"type", "status"})

	TransactionAmount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bank_transaction_amount_total",
		Help: "Total amount of banking transactions.",
	}, []string{"type"})

	AuditLogFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "audit_log_failures_total",
		Help: "Total number of failed audit log writes.",
	}, []string{"action"})

	AccountLockouts = promauto.NewCounter(prometheus.CounterOpts{
		Name: "account_lockouts_total",
		Help: "Total number of account lockouts due to failed login attempts.",
	})

	FailedLoginAttempts = promauto.NewCounter(prometheus.CounterOpts{
		Name: "failed_login_attempts_total",
		Help: "Total number of failed login attempts.",
	})

	RejectedTransactions = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rejected_transactions_total",
		Help: "Total number of transactions rejected due to validation failures.",
	}, []string{"reason", "type"})

	FeeRevenue = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bank_fee_revenue_total",
		Help: "Total fee revenue collected in cents.",
	}, []string{"transaction_type"})

	FeeTransactionsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bank_fee_transactions_total",
		Help: "Total number of fee transactions created.",
	})
)
