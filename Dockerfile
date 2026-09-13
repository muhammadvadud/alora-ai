# --- Build bosqichi ---
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /agent ./cmd/agent

# --- Run bosqichi ---
FROM gcr.io/distroless/static-debian12
WORKDIR /app

COPY --from=build /agent /app/agent
COPY configs ./configs

EXPOSE 8080
# API kalitni muhit orqali bering: -e DEEPSEEK_API_KEY=...
ENTRYPOINT ["/app/agent", "-mode", "http"]
