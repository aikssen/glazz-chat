FROM golang:1.26.5-alpine3.23 AS build
WORKDIR /src

COPY apps/api/go.mod apps/api/go.sum ./apps/api/
RUN cd apps/api && go mod download

COPY apps/api ./apps/api
RUN cd apps/api && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN cd apps/api && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker
RUN cd apps/api && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM alpine:3.23.3
RUN apk add --no-cache ca-certificates tzdata
RUN addgroup -S glazz && adduser -S -G glazz glazz
WORKDIR /app
COPY --from=build /out/api /out/worker /out/migrate /app/
USER glazz
EXPOSE 8080
CMD ["/app/api"]
