# syntax=docker/dockerfile:1


FROM golang:latest AS build-stage

WORKDIR /opt/api-to-parquet

COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -o /app

FROM gcr.io/distroless/base-debian11 AS build-release-stage
WORKDIR /
COPY --from=build-stage /app /app
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app"]