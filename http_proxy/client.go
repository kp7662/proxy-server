package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
)

func main() {
	// Define the proxy server URL
	// This is a local proxy server running on the machine at localhost (127.0.0.1) on port 9999
	proxyStr := "http://127.0.0.1:9999"

	// Parse raw proxy URL string into a url.URL struct with fields (e.g. host, fragments, params, etc.)
	proxyURL, err := url.Parse(proxyStr)
	if err != nil {
		log.Fatal(err)
	}

	// Set up an HTTP client that uses proxy
	client := &http.Client{
		// Transport routes HTTP requests through the specified proxy server instead of the end server
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
	}

	// Parsing command line flags for target URLs
	flag.Parse()
	targetURLs := flag.Args()

	if len(targetURLs) == 0 {
		log.Fatal("No URL provided.")
	}

	// Performing GET requests for each provided URL
	for _, target := range targetURLs {
		testRequest(client, "GET", target, nil)
	}

}

func testRequest(client *http.Client, method string, url string, body *bytes.Buffer) {
	var reqBody io.Reader
	if body != nil {
		reqBody = body
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		log.Printf("Failed to create request for %s: %v\n", url, err)
		return
	}
	/*If no proxy is configured, the client extracts the host from the URL for the TCP connection
	and uses the path part in the HTTP request line.If a proxy is configured, the client sends the
	 full URL to the proxy.*/
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error on %s request to %s: %v\n", method, url, err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Response for %s request to %s: %s\n", method, url, resp.Status)
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response body for %s: %v\n", url, err)
		return
	}
	fmt.Println("Response body:", string(content))
}
