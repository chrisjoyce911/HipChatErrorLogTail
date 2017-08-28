.PHONY: all docker deps silent-test format test report clean
SHELL := /bin/sh

all: format build/tailtohip.out build/tailtohip .git/hooks/pre-commit 

docker: silent-test build/.ca-bundle build/docker-tailtohip build/.docker-tailtohip 

build/tailtohip.out: HipChatErrorLogTail.go
	go test -coverprofile=build/tailtohip.out

build/tailtohip: HipChatErrorLogTail.go
	go build -o build/tailtohip .

build/docker-tailtohip: HipChatErrorLogTail.go 
	SCGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o build/docker-tailtohip .

build/.docker-tailtohip: build/tailtohip build/docker-tailtohip Dockerfile
	docker build -t tailtohip -f Dockerfile .
	touch build/.docker-tailtohip

build/.ca-bundle: Dockerfile
	./mk-ca-bundle.pl -u
	mv ca-bundle.crt build/ca-bundle.crt
	touch build/.ca-bundle

.git/hooks/pre-commit: pre-commit
	ln -s ../../pre-commit .git/hooks/pre-commit

test:
	go test -v -cover ./...

silent-test:
	go test ./...

format:
	go fmt ./...

deps:
	go get -v ./...

report:
	go tool cover -html=build/tailtohip.out

clean:
	-rm build/.docker-tailtohip
	-rm build/docker-tailtohip
	-rm build/tailtohip.out
	-rm build/tailtohip