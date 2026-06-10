package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"github.com/andybalholm/brotli"
	"io/ioutil"
	"net/http/httputil"
	"strconv"

	"github.com/buger/goreplay/proto"
)

func prettifyHTTP(p []byte) []byte {

	tEnc := bytes.Equal(proto.Header(p, []byte("Transfer-Encoding")), []byte("chunked"))
	cEnc := bytes.Equal(proto.Header(p, []byte("Content-Encoding")), []byte("gzip"))
	bEnc := bytes.Equal(proto.Header(p, []byte("Content-Encoding")), []byte("br"))

	if !(tEnc || cEnc || bEnc) {
		return p
	}

	headersPos := proto.MIMEHeadersEndPos(p)

	if headersPos < 5 || headersPos > len(p) {
		return p
	}

	headers := p[:headersPos]
	content := p[headersPos:]

	if tEnc {
		buf := bytes.NewReader(content)
		r := httputil.NewChunkedReader(buf)
		content, _ = ioutil.ReadAll(r)

		headers = proto.DeleteHeader(headers, []byte("Transfer-Encoding"))

		newLen := strconv.Itoa(len(content))
		headers = proto.SetHeader(headers, []byte("Content-Length"), []byte(newLen))
	}

	if cEnc {
		buf := bytes.NewReader(content)
		g, err := gzip.NewReader(buf)

		if err != nil {
			Debug(1, "[Prettifier] GZIP encoding error:", err)
			return []byte{}
		}

		content, err = ioutil.ReadAll(g)
		if err != nil {
			Debug(1, fmt.Sprintf("[HTTP-PRETTIFIER] %q", err))
			return p
		}

		headers = proto.DeleteHeader(headers, []byte("Content-Encoding"))

		newLen := strconv.Itoa(len(content))
		headers = proto.SetHeader(headers, []byte("Content-Length"), []byte(newLen))
	}

	if bEnc {
		buf := bytes.NewReader(content)
		reader := brotli.NewReader(buf)

		decodeContent, err := ioutil.ReadAll(reader)
		if err != nil {
			Debug(1, fmt.Sprintf("[HTTP-PRETTIFIER] %q", err))
			return p
		}

		content = decodeContent
		headers = proto.DeleteHeader(headers, []byte("Content-Encoding"))

		newLen := strconv.Itoa(len(content))
		headers = proto.SetHeader(headers, []byte("Content-Length"), []byte(newLen))
	}

	newPayload := append(headers, content...)

	return newPayload
}
