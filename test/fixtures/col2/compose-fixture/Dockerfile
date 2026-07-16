FROM golang:1.22-alpine AS build
WORKDIR /src
COPY . .
RUN go build -o /out/api ./cmd/api

FROM alpine:3.19
COPY --from=build /out/api /usr/local/bin/api
EXPOSE 8080
