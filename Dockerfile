FROM node:20-alpine AS web-builder

WORKDIR /app/web

RUN corepack enable

COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY web ./
RUN pnpm build


FROM golang:1.26-alpine AS go-builder

WORKDIR /app

COPY go.mod ./
COPY main.go ./
COPY main_test.go ./
COPY web ./web
COPY --from=web-builder /app/web/dist ./web/dist

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/speedtest-next .


FROM alpine:3.21

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=go-builder /out/speedtest-next /usr/local/bin/speedtest-next

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/speedtest-next"]
