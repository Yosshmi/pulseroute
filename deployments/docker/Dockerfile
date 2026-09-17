FROM node:24-alpine AS frontend
WORKDIR /frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.27-alpine AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations
RUN set -e; for app in api worker receiver migrate demo seed loadtest; do CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/$app ./cmd/$app; done

FROM alpine:3.22 AS runtime
RUN apk add --no-cache ca-certificates && addgroup -S app && adduser -S app -G app
WORKDIR /app
COPY --from=backend /out/ /app/bin/
COPY --from=frontend /frontend/dist/ /app/frontend/dist/
USER app
EXPOSE 8080 8081 8090
CMD ["/app/bin/api"]
