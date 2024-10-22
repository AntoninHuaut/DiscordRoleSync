FROM golang:1.23-alpine

EXPOSE 8080

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

ADD . .

RUN go build -o /discord_role_sync

CMD [ "/discord_role_sync" ]
