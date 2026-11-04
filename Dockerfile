# Etapa 1: compila y corre vet + tests. Si los tests fallan, la imagen no se construye y no se despliega
FROM golang:1.25.4-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go vet ./... && go test ./...
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/c_trd .
# Runner de los agentes: no corre en el contenedor, el deploy lo copia al host donde esta Claude Code (cron de los viernes)
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/agent_runner ./cmd/agent_runner

# Etapa 2: imagen final solo con el binario, corre con un usuario sin privilegios
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 ctrd

WORKDIR /app
COPY --from=build /out/c_trd /app/c_trd
COPY --from=build /out/agent_runner /app/agent_runner
RUN mkdir -p /app/data/google_trends && chown -R ctrd:ctrd /app/data

USER ctrd
EXPOSE 8080
ENTRYPOINT ["/app/c_trd"]
