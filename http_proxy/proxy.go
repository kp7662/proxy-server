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
	"sync"
	"time"
)

// --------------------------------------------------------------------
// Headers-related helper functions

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

// copyHeader copies all header values from the 'src' http.Header to the 'dst' http.Header
func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// removeHopHeaders removes hop-by-hop headers from an http.Header
func removeHopHeaders(header http.Header) {
	for _, h := range hopHeaders {
		header.Del(h)
	}
}

// removeConnectionHeaders removes headers listed in the 'Connection' header from the http.Header map
// See RFC 7230, section 6.1
func removeConnectionHeaders(h http.Header) {
	for _, f := range h["Connection"] {
		for _, sf := range strings.Split(f, ",") {
			if sf = strings.TrimSpace(sf); sf != "" {
				h.Del(sf)
			}
		}
	}
}

// appendHostToXForwardHeader updates the 'X-Forwarded-For' header in the http.Header map
// It appends the given 'host' (representing the client IP address) to the 'X-Forwarded-For' header
// This header is used to identify the originating IP addresses of a client connecting to a web server
// through an HTTP proxy or load balancer
func appendHostToXForwardHeader(header http.Header, host string) {
	// Check if the 'X-Forwarded-For' header already exists
	// If the header exists, append the new 'host' to the existing header values

	if prior, ok := header["X-Forwarded-For"]; ok {
		host = strings.Join(prior, ", ") + ", " + host
	}
	header.Set("X-Forwarded-For", host)
}

// --------------------------------------------------------------------

// forwardProxy defines the structure of a forward proxy server, which includes
// functionality for blocking certain domains and caching HTTP responses
type forwardProxy struct {
	blockedSet *BlockedSet
	cache      *HTTPCache
}

// responseWriter is an interface!!!
func (p *forwardProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	startTime := time.Now() // Start time measurement
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

	if req.Method == "CONNECT" {
		p.handleTunneling(w, req)
		return
	}

	if req.Method == "GET" {
		if cachedResponse, found := p.cache.Get(req); found {
			processStartTime := time.Now()
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
			processDuration := time.Since(processStartTime)
			log.Printf("Served from cache in %v\n", processDuration)
			log.Println("Sent from the cache")
			return
		}
	}
	processStartTime := time.Now()
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
	processDuration := time.Since(processStartTime)
	log.Printf("Served from destination server in %v\n", processDuration)
	//io.Copy(w, resp.Body)
	totalDuration := time.Since(startTime)
	log.Printf("Total request processing time: %v\n", totalDuration)
}

// --------------------------------------------------------------------
// Helper functions

// handleTunneling handles the CONNECT method for a forward proxy
// by establishing a secure tunnel for HTTPS connections
func (p *forwardProxy) handleTunneling(w http.ResponseWriter, req *http.Request) {
	log.Printf("Handling CONNECT for %s\n", req.Host)

	// Establish a TCP connection to the requested host
	log.Println("Attempting to connect to the destination host")
	destConn, err := net.DialTimeout("tcp", req.Host, 10*time.Second)
	if err != nil {
		log.Printf("Error connecting to destination host: %v\n", err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer destConn.Close()
	log.Println("Connection to destination host established")

	// Send 200 OK to the client
	log.Println("Sending 200 OK to the client")
	w.WriteHeader(http.StatusOK)

	// Hijack the connection
	log.Println("Attempting to hijack the connection")
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		log.Println("Error: Hijacking not supported")
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		log.Printf("Error hijacking the connection: %v\n", err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer clientConn.Close()
	log.Println("Connection hijacked successfully")

	// Initialize the WaitGroup and add a count of 2 (for the two goroutines)
	var wg sync.WaitGroup
	wg.Add(2)

	// Start the transfer goroutines with the WaitGroup
	go transfer(destConn, clientConn, &wg)
	go transfer(clientConn, destConn, &wg)

	// Wait for both transfers to complete before closing the connections
	wg.Wait()
}

// transfer handles the transfer of data from the source to the destination
// to relay data between a client and a server
func transfer(destination io.WriteCloser, source io.ReadCloser, wg *sync.WaitGroup) {
	defer wg.Done() // Signal completion of this goroutine
	defer destination.Close()
	defer source.Close()

	// Log start of data transfer
	log.Println("Starting data transfer")

	n, err := io.Copy(destination, source)
	if err != nil {
		if err == io.EOF {
			// EOF is expected when the connection is closed normally
			log.Printf("Data transfer completed with %d bytes transferred\n", n)
		} else {
			log.Printf("Error during data transfer: %v\n", err)
		}
	} else {
		log.Printf("Data transfer successful with %d bytes transferred\n", n)
	}

	log.Println("Closing connections")
}

// extractClientIP extracts the IP address from an HTTP request
func extractClientIP(req *http.Request) string {
	ip, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr // Fallback to using the whole RemoteAddr string
	}
	return ip
}

// --------------------------------------------------------------------

func main() {
	var addr = flag.String("addr", " 10.8.75.17:9999", "proxy address")
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
