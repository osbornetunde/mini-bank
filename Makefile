.PHONY: run build dev clean

# Run the application normally
run:
	go run cmd/bank/main.go

# Build the application
build:
	go build -o bin/bank cmd/bank/main.go

# Run with live reload using air
dev:
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "Air not found in PATH, checking $(HOME)/go/bin/air..."; \
		if [ -f "$(HOME)/go/bin/air" ]; then \
			$(HOME)/go/bin/air; \
		else \
			echo "Air not found. Installing..."; \
			go install github.com/air-verse/air@latest; \
			$(HOME)/go/bin/air; \
		fi \
	fi

# Clean build artifacts
clean:
	rm -rf tmp/ bin/
