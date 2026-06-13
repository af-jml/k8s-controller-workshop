FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
COPY api/ ./api/
COPY controllers/ ./controllers/
RUN CGO_ENABLED=0 go build -o /out/controller .

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/controller /controller
USER nonroot:nonroot
ENTRYPOINT ["/controller"]
