# DockerShocker

** WORK IN PROGRESS, NOT YET COMPLETE **

A lightweight middleware for Docker containers that allows them to be automatically shut down after a specified period of inactivity. This tool is especially useful for preserving resources in environments with non-essential containers that don't need to run continuously.

## Features

- **Automatic Shutdown**: Automatically stops containers after a specified period of inactivity.
- **On-the-Fly Startup**: Integrates with Traefik to automatically start containers when they're accessed.
- **Rate Limiting**: Prevents abuse with rate-limited access.
- **Container Monitoring**: View the status, last accessed time, and configuration of managed containers.
- **Health Check**: Provides a simple health check endpoint.

## Configuration

### Labels

Containers you wish to manage with this tool need specific labels:
- `dockershocker.enabled: true`: This indicates the container should be managed by the tool.
- `dockershocker.timeout_minutes: <number_of_minutes>`: Specifies the idle timeout for the container. If not provided, defaults to 15 minutes.

### Startup Arguments

The application can be passed the following startup arguments:

- `-loglevel` (default: `info`), options: `debug`, `info`, `warn`, `error`, e.g. `-loglevel=debug`
  - When run in a container, this is read from the `LOG_LEVEL` environment variable.
- `-port` (default: `8080`), e.g. `-port=8080`
  - When run in a container, this is read from the `PORT` environment variable.
- `-socket` (default: `tcp:///dockerproxy:2375`), e.g. `-socket=unix:///var/run/docker.sock`
  - When run in a container, this is read from the `DOCKER_SOCKET` environment variable.

### Recommendations

- Follow the principle of least privilege.
- Use [docker-socket-proxy](https://github.com/Tecnativa/docker-socket-proxy) rather than passing the Docker socket directly to the middleware.
- Set the container to read_only and no_new_privileges.

### Example:

In your `docker-compose.yml`, you can define:

```yaml
services:
  your_service:
    image: your_image
    labels:
      dockershocker.enabled: true
      dockershocker.timeout_minutes: 60
...
  dockershocker:
    image: sammcj/dockershocker:latest
    restart: unless-stopped
    security_opt:
      - no-new-privileges:true
    read_only: true
    ports:
      - 8080:8080
    environment:
      LOG_LEVEL: debug
      DOCKER_SOCKET: tcp://dockerproxy:2375
      PORT: 8080
...
  traefik:
...
    labels:
      # DockerShocker middleware to stop/start containers on demand
      traefik.http.middlewares.dockershocker.forwardauth.address: http://dockershocker:8080 # or bet yet - put traefik in front of it for https!
      traefik.http.middlewares.dockershocker.forwardauth.trustForwardHeader: true
```

## Limitations

- The tool assumes access to the Docker API, either via a socket or a network endpoint.
- The solution is designed to work in environments that are already secured. Ensure you follow the security best practices outlined below.
- This tool is best suited for environments where you have containers that aren't mission-critical to always be running, like development or testing environments.

## Security Best Practices

- **Docker API Access**: Ensure secure access to the Docker API. If exposing over a network, use proper authentication and encryption.
- **Rate Limiting**: The tool provides rate limiting. Adjust the limits according to your needs to prevent abuse.
- **Network Access**: Restrict network access to the middleware. Ensure only trusted sources can communicate with it.

## Running the Middleware

### Directly:

```bash
go run main.go
```

### In Docker:

1. Build the Docker image:

```bash
docker build -t dockershocker .
```

2. Run the container:

```bash
docker run -d --name dockershocker -v /var/run/docker.sock:/var/run/docker.sock -p 8080:8080 dockershocker
```

## Integration with Traefik

1. Use the [forwardAuth](https://doc.traefik.io/traefik/v2.0/middlewares/forwardauth/) middleware in Traefik.

2. Docker Labels in Your Services:

```yaml
labels:
  traefik.http.middlewares.dockershocker.forwardauth.address: http://dockershocker:8080
  traefik.http.middlewares.dockershocker.forwardauth.trustForwardHeader: true
```

3. Apply the middleware to your routers:

```yaml
labels:
  traefik.http.routers.your_service.middlewares: dockershocker
```

## TODO

- More control with labels such as:
  - start on demand enabled/disabled separately from stop on demand
  - rate limiting

---

## Author

Sam McLeod

## License

- Copyright (c) 2023 Sam McLeod
- Apache License, Version 2.0, see [LICENSE](LICENSE) or <http://www.apache.org/licenses/LICENSE-2.0>