FROM golang

WORKDIR /go/src/github.com/onlineconf/onlineconf-csi-driver

COPY go.* ./
RUN go mod download

COPY *.go ./
RUN go build -o onlineconf-csi-driver

FROM gcr.io/distroless/base

COPY --from=0 /go/src/github.com/onlineconf/onlineconf-csi-driver/onlineconf-csi-driver /usr/local/bin/onlineconf-csi-driver

ENTRYPOINT ["onlineconf-csi-driver"]
