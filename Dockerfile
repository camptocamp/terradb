FROM golang:1.11 as builder
WORKDIR /go/src/github.com/camptocamp/terradb
COPY . .
RUN make terradb

FROM scratch
COPY --from=builder /go/src/github.com/camptocamp/terradb/terradb /
ENTRYPOINT ["/terradb"]
CMD [""]
