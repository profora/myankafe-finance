FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/bootstrap ./cmd/bootstrap

FROM alpine:3.20
RUN adduser -D -u 10001 app
USER app
COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/bootstrap /usr/local/bin/bootstrap
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
