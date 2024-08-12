# syntax=docker/dockerfile:1
FROM golang:1-bookworm AS build

RUN go install github.com/tmc/mkprog@latest
RUN go install github.com/tmc/cgpt/cmd/cgpt@latest
RUN go install github.com/tmc/misc/ctx-plugins/ctx-exec@latest

FROM golang:1-bookworm AS final

RUN apt-get update && apt-get install -y \
		file

COPY --from=docker /usr/local/bin/docker /usr/local/bin/docker
COPY --from=docker/buildx-bin /buildx /usr/libexec/docker/cli-plugins/docker-buildx
COPY --from=build /go/bin/mkprog /usr/local/bin/mkprog
COPY --from=build /go/bin/cgpt /usr/local/bin/cgpt
COPY --from=build /go/bin/ctx-exec /usr/local/bin/ctx-exec
RUN curl -fsSL https://github.com/tmc/misc/raw/master/code-to-gpt/code-to-gpt.sh -o /root/code-to-gpt.sh && chmod +x /root/code-to-gpt.sh
