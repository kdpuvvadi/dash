.PHONY: ui build run tidy validate docker

ui:
	cd web && npm install && npm run build
	rm -rf internal/embedui/dist
	mkdir -p internal/embedui/dist
	cp -r web/dist/* internal/embedui/dist/

tidy:
	go mod tidy

build: ui tidy
	go build -o bin/dash ./cmd/dash

docker:
	docker build -t dash:local .

run: build
	DASH_CONFIG=./config ./bin/dash serve -config ./config

validate: build
	./bin/dash validate -config ./config
