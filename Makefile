NAME = awsc
GOBUILD = go build
ALL_GOARCH = amd64 386
ALL_GOOS = windows linux darwin

.PHONY: build
build: bin/awsc ## Build the awsc executable

bin/awsc:
	mkdir -p bin
	go build -o bin/awsc

.PHONY: dist
dist:
	$(eval export NAME)
	$(eval export GOBUILD)
	$(eval export ALL_GOARCH)
	$(eval export ALL_GOOS)
	./dist.sh

.PHONY: clean
clean:
	rm -rf bin

.PHONY: test
test: bin/awsc
	@PATH="./bin:${PATH}" go test $(go list ./... | grep -v /vendor/)
