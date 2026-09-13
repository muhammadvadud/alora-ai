FROM golang:1.27-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /agent ./cmd/agent

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build /agent /app/agent
COPY configs /app/configs

EXPOSE 9090 8081
# /readyz endpointi HTTP health server bilan birga yoqilganda ishlaydi.
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 CMD ["/app/agent", "-healthcheck", "/readyz"]
ENTRYPOINT ["/app/agent", "-mode", "grpc"]
