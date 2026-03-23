FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags "-X main.version=$(git describe --tags --always 2>/dev/null || echo docker)" -o /postDash .

FROM alpine:3.21
RUN apk add --no-cache bash
COPY --from=build /postDash /usr/local/bin/postDash
COPY scripts/ /usr/local/share/postDash/scripts/
EXPOSE 6060
ENTRYPOINT ["postDash"]
CMD ["--bind", "0.0.0.0", "--port", "6060"]
