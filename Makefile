.PHONY: update-api gen

update-api:
	git subtree pull --prefix api-src https://github.com/Bungie-net/api master --squash

gen:
	go run ./generator/ -spec ./api-src/openapi.json > out.go && go fmt .
