FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags "-X main.version=$(git describe --tags --always 2>/dev/null || echo docker)" -o /eventrelay .

FROM alpine:3.21
RUN apk add --no-cache bash
COPY --from=build /eventrelay /usr/local/bin/eventrelay
COPY scripts/ /usr/local/share/eventrelay/scripts/
EXPOSE 6060
ENTRYPOINT ["eventrelay"]
CMD ["--bind", "0.0.0.0", "--port", "6060"]
