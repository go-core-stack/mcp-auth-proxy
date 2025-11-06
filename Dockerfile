FROM golang:1.24 as builder
ARG GIT_TOKEN

WORKDIR /workspace

COPY ./ .

RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -o auth-proxy main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=builder /workspace/auth-proxy .
USER 65532:65532

ENTRYPOINT ["/auth-proxy"]
