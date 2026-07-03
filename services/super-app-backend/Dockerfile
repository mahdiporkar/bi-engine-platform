FROM golang:1.23-alpine

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN go mod tidy && go build -o /usr/local/bin/super-app-backend ./cmd/server

EXPOSE 8090
ENTRYPOINT ["super-app-backend"]
