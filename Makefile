.PHONY: dev build contracts format go-check e2e check

dev:
	pnpm dev

build:
	pnpm build
	go build ./apps/api/cmd/...

contracts:
	pnpm contracts:lint
	pnpm contracts:generate

format:
	pnpm exec prettier --write .
	cd apps/api && gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

go-check:
	pnpm go:format:check
	cd apps/api && go vet ./...
	cd apps/api && go test -race ./...

e2e:
	pnpm e2e

check:
	pnpm check
