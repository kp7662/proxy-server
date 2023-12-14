// To start the server application, run "go run cache_without_lru.go blockedset.go proxy.go"
// If you test locally, make sure client.go is running too on a separate terminal
// If you test with Firefox, make sure you have the right IP addresses set
// See detailed instructions on how to run the proxy server here:
// https://docs.google.com/document/d/1ZWytA7NaqHS3sTXdNedSzkx-AK9Zd-Mdyfyhu_AgLDg/edit

// Citation & Ackowledgement: Some specifications of this proxy server is inspired by
// Stanford’s CS110 on “HTTP Web Proxy and Cache.” We made several modifications in our
// implementation and design choices, including adding a feature to handle HTTPS CONNECT
// requests, implementing a Blocking mechanism, and implementing caching. Furthermore,
// our proxy.go implementation is built upon a base code and tutorials in this website
// https://eli.thegreenplace.net/2022/go-and-proxy-servers-part-1-http-proxies/, which
// includes functionalities of handling Hop-by-hop and X-forwarded headers. While these
// functionalities are not part of our initial project proposal, we decided to include them
// to adhere to the standard HTTP protocols (as we tried to simulate an actual proxy server
// as much as possible.) In our implementation of blocking, caching, and handling HTTP Connect
// requests, we made our own design decisions based on our research about Cache-Control
// headers found online and by aligning our approach with that of a real proxy caching server.

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

// Hop-by-hop headers.
var hopHeaders = []string{
	"Connection",
	"Proxy-Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailer",
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

// removeConnectionHeaders removes headers listed in the 'Connection' header
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
func appendHostToXForwardHeader(header http.Header, host string) {
	// Check if the 'X-Forwarded-For' header already exists
	// If the header exists, append the new 'host' to the existing header values
	if prior, ok := header["X-Forwarded-For"]; ok {
		host = strings.Join(prior, ", ") + ", " + host
	}
	header.Set("X-Forwarded-For", host)
}

// forwardProxy defines the structure of a forward proxy server, which includes
// functionality for blocking certain domains and caching HTTP responses
type forwardProxy struct {
	blockedSet *BlockedSet
	cache      *HTTPCache
}

// ServeHTTP handles incoming HTTP requests by forwarding them to the destination server,
// caching responses if they are cacheable, and adding necessary headers for proxying
// It also performs checks for blocked domains and supports HTTP tunneling for CONNECT requests
// This function implements the http.Handler interface
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

	// Note to Grader: If you wish to configure the server to only handle HTTP
	// but not HTTPS requests, move the following code block to the indicated
	// position below (after validating for http requests)
	if req.Method == "CONNECT" {
		p.handleTunneling(w, req)
		return
	}

	// Check if the protocol is supported
	if req.URL.Scheme != "http" {
		msg := "unsupported protocol scheme " + req.URL.Scheme
		http.Error(w, msg, http.StatusBadRequest)
		log.Println(msg)
		return
	}

	// Note to Grader: You may move CONNECT checks here

	// Only GET requests are getting cached
	if req.Method == "GET" {
		// If the data is cached and not stale, get it from the cache
		if cachedResponse, found := p.cache.Get(req); found {
			processStartTime := time.Now()
			// Copy cached response to the response writer
			removeHopHeaders(cachedResponse.Header)
			removeConnectionHeaders(cachedResponse.Header)
			log.Println("cached header", cachedResponse.Header)
			copyHeader(w.Header(), cachedResponse.Header)
			w.WriteHeader(cachedResponse.StatusCode)
			io.Copy(w, cachedResponse.Body)
			processDuration := time.Since(processStartTime)
			log.Printf("Served from cache in %v\n", processDuration)
			// log.Println("Served from cache")
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
	log.Println("Modified Headers:", req.Header) // Check the modified headers
	client := &http.Client{}
	req.RequestURI = ""
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Server Error", http.StatusInternalServerError)
		log.Fatal("ServeHTTP:", err)
		return
	}
	defer resp.Body.Close()

	// Helps with making sure the resp.Body is not read before sending it to the client while caching it
	var box bytes.Buffer
	// Tracks if the data was cachable and the cache.put function is called
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
			// log.Println("LAST MODIFIED", lastModified)
			// If we do not know when was the web page last-modified, we take a coservative approach by treating it as a stale page
			if lastModified == "" {
				lastModified = "na"
			}

			// Store in cache based on max-age
			if maxAge != -1 {
				box = p.cache.Put(req, resp, maxAge, lastModified)
				cachable = 0

			} else {
				box = p.cache.Put(req, resp, -1, lastModified) // Store without max-age, i.e. always validate the data
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
	// Return the resp.Body based on whether it was cached
	if cachable == 0 {
		io.Copy(w, bytes.NewReader(box.Bytes()))
	} else {
		io.Copy(w, resp.Body)
	}
	processDuration := time.Since(processStartTime)
	log.Printf("Served from destination server in %v\n", processDuration)
	totalDuration := time.Since(startTime)
	log.Printf("Total request processing time: %v\n", totalDuration)
}

// handleTunneling handles the CONNECT method for a forward proxy
// by establishing a secure tunnel for HTTPS connections
func (p *forwardProxy) handleTunneling(w http.ResponseWriter, req *http.Request) {
	log.Printf("Handling CONNECT for %s\n", req.Host)

	// Establish a TCP connection to the requested host
	// log.Println("Attempting to connect to the destination host")
	destConn, err := net.DialTimeout("tcp", req.Host, 10*time.Second)
	if err != nil {
		// log.Printf("Error connecting to destination host: %v\n", err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer destConn.Close()
	// log.Println("Connection to destination host established")

	// Send 200 OK to the client
	// log.Println("Sending 200 OK to the client")
	w.WriteHeader(http.StatusOK)

	// Hijack the connection
	// log.Println("Attempting to hijack the connection")
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		// log.Println("Error: Hijacking not supported")
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		// log.Printf("Error hijacking the connection: %v\n", err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer clientConn.Close()
	// log.Println("Connection hijacked successfully")

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
	// log.Println("Starting data transfer")

	_, err := io.Copy(destination, source)
	if err != nil {
		if err == io.EOF {
			// EOF is expected when the connection is closed normally
			// log.Printf("Data transfer completed with %d bytes transferred\n", n)
		} else {
			// log.Printf("Error during data transfer: %v\n", err)
		}
	} else {
		// log.Printf("Data transfer successful with %d bytes transferred\n", n)
	}

	// log.Println("Closing connections")
}

// extractClientIP extracts the IP address from an HTTP request
func extractClientIP(req *http.Request) string {
	ip, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr
	}
	return ip
}

// --------------------------------------------------------------------

func main() {
	// Note to Grader: If you are running a client application on a brouser on a
	// different IP address, inputthe IP of the server you are running it on
	// Comment it out if you are running the proxy server on localhost
	var addr = flag.String("addr", "10.8.75.17:9999", "proxy address")

	// Note to Grader: Uncomment if you want to test locally with client.go
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
