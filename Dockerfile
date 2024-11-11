FROM golang:1.22.9-alpine as builder

ENV USER=appuser
ENV UID=10001

RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    "${USER}"

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY *.go ./

RUN GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /service

FROM scratch

COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group
COPY --from=builder /service ./

USER appuser:appuser

CMD ["/service"]