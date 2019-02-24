FROM golang:alpine AS build

WORKDIR /app
COPY . .

RUN apk add --update --no-cache ca-certificates tzdata
RUN GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -o=readss

FROM scratch

WORKDIR /app
COPY --from=build /app/readss .
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
COPY subs.xml template.html manifest.json sw.js ./

EXPOSE 8080/tcp
ENTRYPOINT ["/app/readss"]
