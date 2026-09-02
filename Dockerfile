FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/library-sync ./cmd/library-sync

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/library-sync /library-sync
ENTRYPOINT ["/library-sync"]
