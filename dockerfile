FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o dockershocker

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/dockershocker .
COPY entrypoint.sh .

ENV LOG_LEVEL=${LOG_LEVEL:-debug}
ENV PORT=${PORT:-8080}
ENV DOCKER_SOCKET=${DOCKER_SOCKET:-tcp://dockerproxy:2375}

EXPOSE $PORT
CMD ["./entrypoint.sh"]