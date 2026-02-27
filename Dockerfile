FROM golang:1.22 AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/gateway ./cmd/gateway

FROM gcr.io/distroless/static:nonroot
USER 65532:65532
WORKDIR /app
COPY --from=build /out/gateway /app/gateway
ENTRYPOINT ["/app/gateway"]
