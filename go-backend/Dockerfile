# DBAWORK go-backend 镜像
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/dbawork ./cmd/dbawork

FROM alpine:3.20
RUN apk add --no-cache tzdata ca-certificates
WORKDIR /app
COPY --from=build /out/dbawork /usr/local/bin/dbawork
ENV DBAWORK_DATA_DIR=/data
EXPOSE 8080
ENTRYPOINT ["dbawork"]
