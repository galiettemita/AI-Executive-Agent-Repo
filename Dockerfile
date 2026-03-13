FROM golang:1.23 AS build
WORKDIR /src
COPY go.mod go.sum* vendor/ ./vendor/
COPY . .
ARG SERVICE=gateway
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -o /out/service ./cmd/${SERVICE}

FROM gcr.io/distroless/static:nonroot
# read-only filesystem is enforced by runtime security context in Helm charts.
USER 65532:65532
WORKDIR /app
COPY --from=build /out/service /app/service
ENTRYPOINT ["/app/service"]
