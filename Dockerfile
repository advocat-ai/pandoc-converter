FROM golang:1.17-alpine AS build

RUN mkdir -p /pandoc-converter

WORKDIR /pandoc-converter

COPY . .

ENV CGO=0

RUN go build -o pandoc-converter && \
    chmod a+rx pandoc-converter

FROM pandoc/latex:2.14

COPY --from=build /pandoc-converter/pandoc-converter /usr/local/bin/pandoc-converter

ENTRYPOINT ["/usr/local/bin/pandoc-converter"]
