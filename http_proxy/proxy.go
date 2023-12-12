// Implements a sample proxy server in Go. Adapted from httputil.ReverseProxy's
// implementation
//
// Sample usage: run this program, then elsewhere run
//
// $ HTTP_PROXY=127.0.0.1:9999 go run http-client-get-url.go <some url>
//
// Then the client will request <some url> through this proxy. Note: if <some
// url> is on localhost, Go clients will ignore HTTP_PROXY; to force them to use
// the proxy, either set up a proxy explicitly in the Transport, or set up an
// alias in /etc/hosts and use that instead of localhost.
//
// Eli Bendersky [https://eli.thegreenplace.net]
// This code is in the public domain.
package main

import (
	"bytes"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
)

// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
// Note: this may be out of date, see RFC 7230 Section 6.1
var hopHeaders = []string{
	"Connection",
	"Proxy-Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",      // canonicalized version of "TE"
	"Trailer", // spelling per https://www.rfc-editor.org/errata_search.php?eid=4522
	"Transfer-Encoding",
	"Upgrade",
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func removeHopHeaders(header http.Header) {
	for _, h := range hopHeaders {
		header.Del(h)
	}
}

// removeConnectionHeaders removes hop-by-hop headers listed in the "Connection"
// header of h. See RFC 7230, section 6.1
func removeConnectionHeaders(h http.Header) {
	for _, f := range h["Connection"] {
		for _, sf := range strings.Split(f, ",") {
			if sf = strings.TrimSpace(sf); sf != "" {
				h.Del(sf)
			}
		}
	}
}

func appendHostToXForwardHeader(header http.Header, host string) {
	// If we aren't the first proxy retain prior
	// X-Forwarded-For information as a comma+space
	// separated list and fold multiple headers into one.
	if prior, ok := header["X-Forwarded-For"]; ok {
		host = strings.Join(prior, ", ") + ", " + host
	}
	header.Set("X-Forwarded-For", host)
}

type forwardProxy struct {
	blockedSet *BlockedSet
	cache      *HTTPCache
}

// responseWriter is an interface!!!
func (p *forwardProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	log.Println(req.RemoteAddr, "\t\t", req.Method, "\t\t", req.URL, "\t\t Host:", req.Host)
	log.Println("Initial Headers:", req.Header)
	// Check for blocked domain
	if p.blockedSet.IsBlocked(req.URL.Hostname()) {
		http.Error(w, "Forbidden Content", http.StatusForbidden)
		log.Println("Forbidden Content")
		return
	}
	// Check if the protocol is supported
	if req.URL.Scheme != "http" {
		msg := "unsupported protocol scheme " + req.URL.Scheme
		http.Error(w, msg, http.StatusBadRequest)
		log.Println(msg)
		return
	}

	if req.Method == "GET" {
		if cachedResponse, found := p.cache.Get(req); found {
			// Copy cached response to the response writer
			removeHopHeaders(cachedResponse.Header)
			removeConnectionHeaders(cachedResponse.Header)
			log.Println("cached header", cachedResponse.Header)
			copyHeader(w.Header(), cachedResponse.Header)
			w.WriteHeader(cachedResponse.StatusCode)
			// body, err := io.ReadAll(cachedResponse.Body)
			// if err != nil {
			// 	// handle error
			// 	return
			// }
			// log.Println("cached body", body)
			io.Copy(w, cachedResponse.Body)
			log.Println("Sent from the cache")
			return
		}
	}

	// Add the X-Forwarded-Proto header
	req.Header.Set("X-Forwarded-Proto", "http")

	// Append the client's IP to the X-Forwarded-For header
	clientIP := extractClientIP(req)
	appendHostToXForwardHeader(req.Header, clientIP)
	removeHopHeaders(req.Header)
	removeConnectionHeaders(req.Header)
	log.Println("Modified Headers:", req.Header) // Added for debugging
	client := &http.Client{}
	// When a http.Request is sent through an http.Client, RequestURI should not
	// be set (see documentation of this field).
	req.RequestURI = ""

	//removeHopHeaders(req.Header)
	//removeConnectionHeaders(req.Header)

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Server Error", http.StatusInternalServerError)
		log.Fatal("ServeHTTP:", err)
		return
	}
	defer resp.Body.Close()

	//removeHopHeaders(resp.Header)
	//removeConnectionHeaders(resp.Header)
	// Copy headers and body to the response writer
	var box bytes.Buffer
	cachable := -1
	if req.Method == "GET" {
		cacheControl := resp.Header.Get("Cache-Control")
		var maxAge int64
		maxAge = -1
		// Default value if max-age is not specified
		lastModified := "na"

		// Check if the response is cacheable
		if strings.Contains(cacheControl, "public") || strings.Contains(cacheControl, "no-cache") ||
			strings.Contains(cacheControl, "max-age") {
			// Parse max-age if present
			if strings.Contains(cacheControl, "max-age") {
				maxAgeDirectives := strings.Split(cacheControl, ",")
				for _, directive := range maxAgeDirectives {
					if strings.Contains(directive, "max-age") {
						maxAgeValue := strings.Split(directive, "=")
						if len(maxAgeValue) == 2 {
							maxAgeParsed, err := strconv.Atoi(strings.TrimSpace(maxAgeValue[1]))
							if err == nil {
								maxAge = int64(maxAgeParsed)
							}
							break
						}
					}
				}
			}

			lastModified = resp.Header.Get("Last-Modified")
			log.Println("LAST MODIFIED", lastModified)
			// If we do not know when was the web page last-modified, we take a coservative approach by treating it as a stale page
			if lastModified == "" {
				lastModified = "na"
			}

			// Store in cache based on max-age
			if maxAge != -1 {
				box = p.cache.Put(req, resp, maxAge, lastModified)
				//p.cache.Put(req, resp, maxAge, lastModified)
				cachable = 0

			} else {
				box = p.cache.Put(req, resp, -1, lastModified) // Store without max-age
				//p.cache.Put(req, resp, -1, lastModified) // Store without max-age

				cachable = 0
			}
		} else {
			log.Println("Not cacheable")
		}
	}
	log.Println(req.RemoteAddr, " ", resp.Status)
	removeHopHeaders(resp.Header)
	removeConnectionHeaders(resp.Header)
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	if cachable == 0 {
		io.Copy(w, bytes.NewReader(box.Bytes()))
	} else {
		io.Copy(w, resp.Body)
	}
	//io.Copy(w, resp.Body)

}

func extractClientIP(req *http.Request) string {
	ip, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr // Fallback to using the whole RemoteAddr string
	}
	return ip
}

// --------------------------------------------------------------------

func main() {
	var addr = flag.String("addr", "10.8.64.73:9999", "proxy address")
	//var addr = flag.String("addr", "127.0.0.1:9999", "proxy address")
	flag.Parse()

	blockedSet, err := NewBlockedSet("blocked-domains.txt")
	if err != nil {
		log.Fatal(err)
	}
	cache := NewHTTPCache()

	proxy := &forwardProxy{
		blockedSet: blockedSet,
		cache:      cache,
	}

	log.Println("Starting proxy server on", *addr)
	if err := http.ListenAndServe(*addr, proxy); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
