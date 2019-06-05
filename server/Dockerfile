FROM golang:alpine AS build

WORKDIR /app
COPY . .
RUN GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -o readss-server

FROM scratch

COPY --from=build /app/readss-server /bin/readss-server

ENTRYPOINT ["/bin/readss-server"]
