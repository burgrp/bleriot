BINARY = bleriot
PKG    = .

build:
	go build -o $(BINARY) $(PKG)

run:
	go run $(PKG) hub

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)

.PHONY: build run test vet tidy clean

