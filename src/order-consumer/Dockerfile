FROM golang:1.27.3@sha256:e519d3b0a5b285e5aa53ab9eef02bc6bbb9cb04608389b743a136a9ce75f0d1e AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /order-consumer .

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /order-consumer /order-consumer
USER 65532:65532
EXPOSE 9102
ENTRYPOINT ["/order-consumer"]
