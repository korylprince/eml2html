[![pkg.go.dev](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/korylprince/eml2html)

# About

`eml2html` is a library and command line tool used to transform RFC 822 (eml) files into html files viewable in a browser.


# Installing

Library, using Go Modules:

`go get github.com/korylprince/eml2html`


CLI Tool:

``` bash
mkdir build
cd build
go mod init build
go get -d github.com/korylprince/eml2html/cmd/eml2html@<version>
go build github.com/korylprince/eml2html/cmd/eml2html
./eml2html -h
```

If you have any issues or questions [create an issue](https://github.com/korylprince/eml2html/issues).
