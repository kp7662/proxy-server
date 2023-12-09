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
	"flag"
	"io"
	"log"
	"net"
	"net/http"
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
}

func (p *forwardProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	log.Println(req.RemoteAddr, "\t\t", req.Method, "\t\t", req.URL, "\t\t Host:", req.Host)
	log.Println("Initial Headers:", req.Header)

	// Check if the protocol is supported
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		msg := "unsupported protocol scheme " + req.URL.Scheme
		http.Error(w, msg, http.StatusBadRequest)
		log.Println(msg)
		return
	}

	// Add the X-Forwarded-Proto header
	req.Header.Set("X-Forwarded-Proto", "http")

	// Append the client's IP to the X-Forwarded-For header
	clientIP := extractClientIP(req)
	appendHostToXForwardHeader(req.Header, clientIP)

	log.Println("Modified Headers:", req.Header) // Added for debugging

	// Forward the request based on its method
	switch req.Method {
	case "GET", "POST", "HEAD":
		forwardRequest(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// --------------------------------------------------------------------
// Helper functions

func modifyRequestURL(req *http.Request) {
	req.URL.Scheme = ""
	req.URL.Host = ""
}

func extractClientIP(req *http.Request) string {
	ip, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr // Fallback to using the whole RemoteAddr string
	}
	return ip
}

func forwardRequest(w http.ResponseWriter, req *http.Request) {
	// Extracting only the path and query from the request URL
	newPath := req.URL.Path
	if req.URL.RawQuery != "" {
		newPath += "?" + req.URL.RawQuery
	}

	// Forwarding the request to the destination server
	// Re-create the request with the new path
	modifiedReq, err := http.NewRequest(req.Method, newPath, req.Body)
	if err != nil {
		http.Error(w, "Server Error", http.StatusInternalServerError)
		log.Printf("Error creating request: %v", err)
		return
	}

	// Copy headers from the original request and modify as needed
	copyHeader(modifiedReq.Header, req.Header)
	modifiedReq.Header.Set("Host", req.Host)
	modifiedReq.Header.Set("X-Forwarded-Proto", "http")

	// Use http.Client to forward the modified request
	client := &http.Client{}
	resp, err := client.Do(modifiedReq)
	if err != nil {
		http.Error(w, "Server Error", http.StatusInternalServerError)
		log.Printf("Error forwarding request: %v", err)
		return
	}
	defer resp.Body.Close()

	// Copy response headers and body back to the original client
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// --------------------------------------------------------------------

func main() {
	var addr = flag.String("addr", "127.0.0.1:9999", "proxy address")
	flag.Parse()

	proxy := &forwardProxy{}

	log.Println("Starting proxy server on", *addr)
	if err := http.ListenAndServe(*addr, proxy); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
