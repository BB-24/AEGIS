FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/aegis-worker ./cmd/aegis-worker

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/aegis-worker /aegis-worker
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/aegis-worker"]
