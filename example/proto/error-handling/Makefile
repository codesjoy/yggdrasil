.PHONY: all generate clean

GO_OUT_DIR := ../../protogen/error-handling

all: generate

generate:
	@echo "Generating Go code from proto files..."
	@cd ../.. && buf generate --path proto/error-handling/reason.proto --path proto/error-handling/service.proto
	@echo "✓ Code generation complete"

clean:
	@echo "Cleaning generated files..."
	@rm -rf $(GO_OUT_DIR)/*.pb.go
	@echo "✓ Clean complete"
