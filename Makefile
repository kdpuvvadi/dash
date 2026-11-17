.PHONY: ui build release run tidy validate

ui:
	cd web && npm install && npm run build
	rm -rf internal/embedui/dist
	mkdir -p internal/embedui/dist
	cp -r web/dist/* internal/embedui/dist/

tidy:
	go mod tidy

build: ui tidy
	go build -o bin/dash ./cmd/dash

release: ui tidy
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/dash ./cmd/dash

run: build
	DASH_CONFIG=./config ./bin/dash serve -config ./config

validate: build
	./bin/dash validate -config ./config
