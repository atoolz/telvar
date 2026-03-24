FROM golang:1.24-alpine AS build

ARG VERSION=dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}" -o /telvar ./cmd/telvar

FROM gcr.io/distroless/static-debian12
COPY --from=build /telvar /telvar
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
EXPOSE 7007
ENTRYPOINT ["/telvar", "serve"]
