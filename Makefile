.PHONY: all generate build clean install test deps vmlinux

# Output directory
BIN_DIR := bin

# Binaries
DAEMON := $(BIN_DIR)/portmond
CLIENT := $(BIN_DIR)/portmon

# Build flags
LDFLAGS := -s -w

all: vmlinux generate build

# Install dependencies (Debian/Ubuntu)
deps:
	@echo "Installing build dependencies..."
	sudo apt-get update
	sudo apt-get install -y clang llvm libbpf-dev bpftool linux-headers-$$(uname -r)

# Generate vmlinux.h from kernel BTF
vmlinux:
	@echo "Generating vmlinux.h from kernel BTF..."
	@BPFTOOL=$$(which bpftool 2>/dev/null || echo "/usr/sbin/bpftool"); \
	if [ -f /sys/kernel/btf/vmlinux ]; then \
		$$BPFTOOL btf dump file /sys/kernel/btf/vmlinux format c > internal/ebpf/bpf/vmlinux.h; \
		echo "vmlinux.h generated successfully ($$(wc -l < internal/ebpf/bpf/vmlinux.h) lines)"; \
	else \
		echo "ERROR: /sys/kernel/btf/vmlinux not found. Your kernel may not have BTF enabled."; \
		exit 1; \
	fi

# Generate eBPF bytecode (requires Linux with clang + libbpf)
generate:
	@echo "Generating eBPF bytecode..."
	cd internal/ebpf && go generate ./...



# Build binaries
build: $(DAEMON) $(CLIENT)

$(DAEMON):
	@mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DAEMON) ./cmd/portmond

$(CLIENT):
	@mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(CLIENT) ./cmd/portmon

# Install to system
install:
	install -m 755 $(DAEMON) /usr/local/bin/portmond
	install -m 755 $(CLIENT) /usr/local/bin/portmon
	@echo "Installed to /usr/local/bin"

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -rf $(BIN_DIR)
	find . -name '*_bpf*.go' -delete
	find . -name '*_bpf*.o' -delete

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...
