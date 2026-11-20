package v2rayxhttp

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/net/http2/hpack"
)

func (c *config) applyMetadata(request *http.Request, sessionID, sequence string) {
	applyMeta(request, c.sessionPlacement, c.sessionKey, sessionID)
	applyMeta(request, c.seqPlacement, c.seqKey, sequence)
}

func applyMeta(request *http.Request, placement, key, value string) {
	if value == "" {
		return
	}
	switch placement {
	case placementPath:
		path := request.URL.Path
		if !strings.HasSuffix(path, "/") {
			path += "/"
		}
		request.URL.Path = path + value
	case placementQuery:
		request.URL.RawQuery = appendQuery(request.URL.RawQuery, key, value)
	case placementHeader:
		request.Header.Set(key, value)
	case placementCookie:
		request.AddCookie(&http.Cookie{Name: key, Value: value})
	}
}

// appendQuery appends a parameter to a raw query. Escaping matches
// url.Values.Encode for the generated session, sequence, and padding values,
// without parsing and re-encoding the whole query for every request.
func appendQuery(rawQuery, key, value string) string {
	if queryHasKey(rawQuery, key) {
		// url.Values.Set overwrites existing pairs and re-encodes the query;
		// keep that behaviour when the configured query already uses the key.
		query, _ := url.ParseQuery(rawQuery)
		query.Set(key, value)
		return query.Encode()
	}
	key = url.QueryEscape(key)
	value = url.QueryEscape(value)
	if rawQuery == "" {
		return key + "=" + value
	}
	return rawQuery + "&" + key + "=" + value
}

// queryGet returns the first value for key, matching url.Values.Get without
// parsing unrelated parameters. It falls back to url.ParseQuery for queries
// containing escaped keys or semicolons, where the fast scan could diverge.
func queryGet(rawQuery, key string) string {
	if strings.ContainsAny(key, "&=;%+") || strings.IndexByte(rawQuery, ';') >= 0 {
		query, _ := url.ParseQuery(rawQuery)
		return query.Get(key)
	}
	for rawQuery != "" {
		pair := rawQuery
		if index := strings.IndexByte(rawQuery, '&'); index >= 0 {
			pair, rawQuery = rawQuery[:index], rawQuery[index+1:]
		} else {
			rawQuery = ""
		}
		name, value, _ := strings.Cut(pair, "=")
		if !queryNameIs(name, key) {
			continue
		}
		decoded, err := url.QueryUnescape(value)
		if err != nil {
			continue
		}
		return decoded
	}
	return ""
}

// queryHasKey reports whether the raw query contains key, honouring percent
// and plus escapes in parameter names.
func queryHasKey(rawQuery, key string) bool {
	for rawQuery != "" {
		pair := rawQuery
		if index := strings.IndexByte(rawQuery, '&'); index >= 0 {
			pair, rawQuery = rawQuery[:index], rawQuery[index+1:]
		} else {
			rawQuery = ""
		}
		name, _, _ := strings.Cut(pair, "=")
		if queryNameIs(name, key) {
			return true
		}
	}
	return false
}

// queryNameIs compares a raw parameter name with the unescaped key.
func queryNameIs(name, key string) bool {
	if name == key {
		return true
	}
	if !strings.ContainsAny(name, "%+") {
		return false
	}
	decoded, err := url.QueryUnescape(name)
	return err == nil && decoded == key
}

// queryFromReference extracts a query parameter from an absolute URL string,
// avoiding net/url parsing for the Referer header used by queryInHeader
// padding. The second result reports whether the input had the expected shape.
func queryFromReference(reference, key string) (string, bool) {
	if !strings.HasPrefix(reference, "http://") && !strings.HasPrefix(reference, "https://") {
		return "", false
	}
	_, rawQuery, found := strings.Cut(reference, "?")
	if !found {
		return "", true
	}
	if fragment := strings.IndexByte(rawQuery, '#'); fragment >= 0 {
		rawQuery = rawQuery[:fragment]
	}
	return queryGet(rawQuery, key), true
}

func (c *config) extractMetadata(request *http.Request) (string, string) {
	var rest string
	if c.sessionPlacement == placementPath || c.seqPlacement == placementPath {
		rest = strings.Trim(strings.TrimPrefix(request.URL.Path, c.path), "/")
	}
	nextPath := func() string {
		if rest == "" {
			return ""
		}
		index := strings.IndexByte(rest, '/')
		if index == 0 {
			// Preserve the original split behaviour: an empty segment is
			// reported without consuming it.
			return ""
		}
		if index < 0 {
			value := rest
			rest = ""
			return value
		}
		value := rest[:index]
		rest = rest[index+1:]
		return value
	}
	extract := func(placement, key string) string {
		switch placement {
		case placementPath:
			return nextPath()
		case placementQuery:
			return queryGet(request.URL.RawQuery, key)
		case placementHeader:
			return request.Header.Get(key)
		case placementCookie:
			cookie, _ := request.Cookie(key)
			if cookie != nil {
				return cookie.Value
			}
		}
		return ""
	}
	return extract(c.sessionPlacement, c.sessionKey), extract(c.seqPlacement, c.seqKey)
}

func (c *config) applyPadding(request *http.Request) {
	length := c.padding.random()
	method := c.paddingMethod
	placement, key, header := c.paddingPlacement, c.paddingKey, c.paddingHeader
	if !c.paddingObfs {
		placement, key, header, method = placementQueryInHeader, "x_padding", "Referer", "repeat-x"
	}
	value := paddingValue(method, length)
	switch placement {
	case placementHeader:
		request.Header.Set(header, value)
	case placementCookie:
		request.AddCookie(&http.Cookie{Name: key, Value: value})
	case placementQuery:
		request.URL.RawQuery = appendQuery(request.URL.RawQuery, key, value)
	case placementQueryInHeader:
		copyURL := *request.URL
		copyURL.RawQuery = appendQuery(copyURL.RawQuery, key, value)
		request.Header.Set(header, copyURL.String())
	}
}

func (c *config) validPadding(request *http.Request) bool {
	placement, key, header, method := c.paddingPlacement, c.paddingKey, c.paddingHeader, c.paddingMethod
	if !c.paddingObfs {
		placement, key, header, method = placementQueryInHeader, "x_padding", "Referer", "repeat-x"
	}
	var value string
	switch placement {
	case placementHeader:
		value = request.Header.Get(header)
	case placementCookie:
		cookie, _ := request.Cookie(key)
		if cookie != nil {
			value = cookie.Value
		}
	case placementQuery:
		value = queryGet(request.URL.RawQuery, key)
	case placementQueryInHeader:
		reference := request.Header.Get(header)
		extracted, ok := queryFromReference(reference, key)
		if !ok {
			if parsed, err := url.Parse(reference); err == nil {
				extracted = parsed.Query().Get(key)
			}
		}
		value = extracted
	}
	if method == "tokenish" {
		encodedLength := int(hpack.HuffmanEncodeLength(value))
		return encodedLength >= int(c.padding.from)-2 && encodedLength <= int(c.padding.to)+2
	}
	return len(value) >= int(c.padding.from) && len(value) <= int(c.padding.to)
}

func paddingValue(method string, length int) string {
	if method != "tokenish" {
		return strings.Repeat("X", length)
	}
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	// Base62 has a close-to-browser Huffman profile. Select the nearest length
	// so the value falls into the configured compressed-byte range. The
	// Huffman length grows with every byte, so binary search replaces the
	// quadratic prefix scan.
	value := make([]byte, length+length/2+8)
	for index := range value {
		value[index] = alphabet[(index*37+length*17)%len(alphabet)]
	}
	low, high := 1, len(value)
	for low < high {
		middle := int(uint(low+high) >> 1)
		if int(hpack.HuffmanEncodeLength(string(value[:middle]))) >= length {
			high = middle
		} else {
			low = middle + 1
		}
	}
	if int(hpack.HuffmanEncodeLength(string(value[:low]))) >= length {
		return string(value[:low])
	}
	return string(value)
}

func (c *config) fillStreamRequest(request *http.Request, sessionID string) {
	request.Header = c.headers.Clone()
	c.applyPadding(request)
	c.applyMetadata(request, sessionID, "")
	if request.Body != nil && !c.noGRPCHeader {
		request.Header.Set("Content-Type", "application/grpc")
	}
}

func (c *config) fillPacketRequest(request *http.Request, sessionID string, sequence uint64, payload []byte) {
	request.Header = c.headers.Clone()
	switch c.dataPlacement {
	case placementHeader:
		c.fillHeaderPayload(request.Header, c.dataKey, payload)
	case placementCookie:
		for index, chunk := range c.splitEncodedPayload(payload) {
			request.AddCookie(&http.Cookie{Name: fmt.Sprintf("%s_%d", c.dataKey, index), Value: chunk})
		}
	default:
		request.Body = &payloadBody{payload: payload}
		request.ContentLength = int64(len(payload))
	}
	c.applyPadding(request)
	c.applyMetadata(request, sessionID, strconv.FormatUint(sequence, 10))
}

type payloadBody struct {
	payload []byte
	offset  int
}

func (b *payloadBody) Read(buffer []byte) (int, error) {
	if b.offset >= len(b.payload) {
		return 0, io.EOF
	}
	n := copy(buffer, b.payload[b.offset:])
	b.offset += n
	return n, nil
}

func (b *payloadBody) Close() error { return nil }

func (c *config) fillHeaderPayload(header http.Header, key string, payload []byte) {
	for index, chunk := range c.splitEncodedPayload(payload) {
		header.Set(fmt.Sprintf("%s-%d", key, index), chunk)
	}
}

func (c *config) splitEncodedPayload(payload []byte) []string {
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	if encoded == "" {
		return nil
	}
	var chunks []string
	for len(encoded) > 0 {
		size := min(c.uplinkChunk.random(), len(encoded))
		chunks, encoded = append(chunks, encoded[:size]), encoded[size:]
	}
	return chunks
}

func (c *config) applyResponsePadding(writer http.ResponseWriter) {
	length := c.padding.random()
	if !c.paddingObfs {
		writer.Header().Set("X-Padding", paddingValue("repeat-x", length))
		return
	}
	value := paddingValue(c.paddingMethod, length)
	switch c.paddingPlacement {
	case placementHeader:
		writer.Header().Set(c.paddingHeader, value)
	case placementCookie:
		http.SetCookie(writer, &http.Cookie{Name: c.paddingKey, Value: value, Path: "/"})
	}
}

func (c *config) extractPacketPayload(request *http.Request, maxSize int) ([]byte, error) {
	switch c.dataPlacement {
	case placementHeader:
		return decodeHeaderPayload(request, c.dataKey)
	case placementCookie:
		return decodeCookiePayload(request, c.dataKey)
	case placementAuto:
		headerPayload, err := decodeHeaderPayload(request, c.dataKey)
		if err != nil {
			return nil, err
		}
		cookiePayload, err := decodeCookiePayload(request, c.dataKey)
		if err != nil {
			return nil, err
		}
		bodyPayload, err := readRequestBody(request, maxSize)
		if err != nil {
			return nil, err
		}
		if len(headerPayload) == 0 && len(cookiePayload) == 0 {
			// The upload landed in the body; return the pooled read buffer as is
			// instead of copying it into a fresh slice.
			return bodyPayload, nil
		}
		payload := append(append(headerPayload, cookiePayload...), bodyPayload...)
		releasePacketBuffer(bodyPayload)
		return payload, nil
	default:
		return readRequestBody(request, maxSize)
	}
}

func decodeHeaderPayload(request *http.Request, key string) ([]byte, error) {
	var encoded strings.Builder
	for index := 0; ; index++ {
		value := request.Header.Get(fmt.Sprintf("%s-%d", key, index))
		if value == "" {
			break
		}
		encoded.WriteString(value)
	}
	if encoded.Len() == 0 {
		return nil, nil
	}
	return base64.RawURLEncoding.DecodeString(encoded.String())
}

func decodeCookiePayload(request *http.Request, key string) ([]byte, error) {
	var encoded strings.Builder
	for index := 0; ; index++ {
		cookie, _ := request.Cookie(fmt.Sprintf("%s_%d", key, index))
		if cookie == nil {
			break
		}
		encoded.WriteString(cookie.Value)
	}
	if encoded.Len() == 0 {
		return nil, nil
	}
	return base64.RawURLEncoding.DecodeString(encoded.String())
}

// packetBufferPool recycles upload payload buffers between packet-up
// requests. Buffers are released only after the session writer or HTTP client
// has consumed the payload. The pool stores *[]byte so that Put does not box
// a slice header into an interface, which would allocate on every release.
var packetBufferPool = sync.Pool{
	New: func() any {
		buffer := make([]byte, 0, 64*1024)
		return &buffer
	},
}

func acquirePacketBuffer(size int) []byte {
	holder := packetBufferPool.Get().(*[]byte)
	buffer := *holder
	if cap(buffer) < size {
		buffer = make([]byte, size)
	}
	return buffer[:size]
}

func releasePacketBuffer(buffer []byte) {
	if cap(buffer) == 0 || cap(buffer) > 4*1024*1024 {
		return
	}
	buffer = buffer[:0]
	packetBufferPool.Put(&buffer)
}

func readRequestBody(request *http.Request, maxSize int) ([]byte, error) {
	if request.ContentLength > int64(maxSize) {
		return nil, fmt.Errorf("request body exceeds xhttp limit")
	}
	if request.ContentLength >= 0 {
		// A fixed-length body is read with exactly one allocation sized to the
		// declared payload instead of io.ReadAll's doubling growth.
		data := acquirePacketBuffer(int(request.ContentLength))
		if _, err := io.ReadFull(request.Body, data); err != nil {
			releasePacketBuffer(data)
			return nil, err
		}
		return data, nil
	}
	data, err := io.ReadAll(io.LimitReader(request.Body, int64(maxSize)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSize {
		return nil, fmt.Errorf("request body exceeds xhttp limit")
	}
	return data, nil
}
