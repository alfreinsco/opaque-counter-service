FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/counter ./cmd/server

FROM scratch
COPY --from=build /out/counter /counter
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/counter"]
