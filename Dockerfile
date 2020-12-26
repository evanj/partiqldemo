FROM golang:1.15.6-buster AS go_java_image
RUN apt-get update && \
  apt-get install --yes --no-install-recommends --no-install-suggests openjdk-11-jdk-headless

FROM go_java_image AS builder
COPY . /go/src/partiqldemo/
WORKDIR /go/src/partiqldemo
RUN make build_binaries

FROM gcr.io/distroless/java-debian10:11-nonroot AS run
COPY --from=builder /go/src/partiqldemo/build/partiqldemo /partiqldemo
COPY --from=builder /go/src/partiqldemo/build/partiqldemo.jar /

# Use a non-root user: slightly more secure (defense in depth)
USER nobody
WORKDIR /
EXPOSE 8080
ENTRYPOINT ["/partiqldemo", "--jar=partiqldemo.jar"]
