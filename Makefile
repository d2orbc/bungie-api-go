.PHONY: update-api

update-api:
	git subtree pull --prefix api-src https://github.com/Bungie-net/api master --squash