FROM golang:1.23-alpine AS build

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o /out/super-app-backend ./cmd/server

FROM alpine:3.21
RUN adduser -D -H appuser
USER appuser
WORKDIR /app
COPY --from=build /out/super-app-backend /app/super-app-backend
EXPOSE 8090
ENTRYPOINT ["/app/super-app-backend"]

