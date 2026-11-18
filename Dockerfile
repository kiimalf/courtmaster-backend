FROM golang:alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Kompilasi binary statis Linux tanpa dependensi library C (-trimpath membersihkan path lokal)
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/courtmaster .

FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
# Aplikasi TIDAK dijalankan sebagai root demi keamanan (Modul 12 Langkah 13)
RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=build /out/courtmaster ./courtmaster

RUN mkdir logs && chown -R app:app /app
USER app
EXPOSE 8080
ENTRYPOINT ["./courtmaster"]
CMD ["-role=api"]