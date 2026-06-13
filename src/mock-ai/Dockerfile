# Build a tiny static binary, ship it on a scratch-like base.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 go build -o /out/mock-ai .

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/mock-ai /mock-ai
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/mock-ai"]
