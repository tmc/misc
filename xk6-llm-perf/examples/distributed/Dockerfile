# Build dist/k6-linux from the frozen module source before building this image.
# Pin the base image digest for any qualified deployment.
FROM alpine:3.22
RUN apk add --no-cache ca-certificates
COPY dist/k6-linux /usr/bin/k6
USER 12345
ENTRYPOINT ["/usr/bin/k6"]
