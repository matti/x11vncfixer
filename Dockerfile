
FROM golang:1.15.0-alpine3.12 as builder

COPY . /build/
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o x11vncfixer .

FROM scratch
COPY --from=builder /build/x11vncfixer /x11vncfixer
ENTRYPOINT [ "/x11vncfixer" ]
