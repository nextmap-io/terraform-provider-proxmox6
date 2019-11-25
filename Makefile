.PHONY:  build  fmt vet test clean install

all: build


fmt:
	@echo " -> checking code style"
	@! gofmt -d $(shell find . -path ./vendor -prune -o -name '*.go' -print) | grep '^'

vet:
	@echo " -> vetting code"
	@go vet ./...

test:
	@echo " -> testing code"
	@go test -v ./...

build: clean
	@echo " -> Building"
	mkdir -p bin
	CGO_ENABLED=0 go build  -o bin/terraform-provider-proxmox6  
	@echo "Built terraform-provider-proxmox"

install: build 
	cp bin/terraform-provider-proxmox6 $$GOPATH/bin/terraform-provider-proxmox6

clean:
	@git clean -f -d -X

