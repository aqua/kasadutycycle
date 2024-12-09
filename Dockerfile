# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:latest AS build
WORKDIR /src

RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,source=go.sum,target=go.sum \
    --mount=type=bind,source=go.mod,target=go.mod \
    go mod download -x

ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    CGO_ENABLED=0 GOARCH=$TARGETARCH go build -o /bin/kasadutycycle .

FROM alpine:latest AS final

ARG UID=10001
RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    kasadutycycle

RUN install --owner=kasadutycycle -d /run/kasadutycycle
COPY --from=build /bin/kasadutycycle /bin/

USER kasadutycycle

# Expose the port that the application listens on.
EXPOSE 8080

# What the container should run when it is started.
CMD [ "/bin/kasadutycycle", "-interval=10s", "-targets=192.168.86.83", "-checkpoint-file=/run/kasadutycycle/checkpoint.json", "-http-listen-address=:8080"  ]
