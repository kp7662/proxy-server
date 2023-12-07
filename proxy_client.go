package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
)

func main() {
	// Proxy URL
	proxyStr := "http://127.0.0.1:9999"
	proxyURL, err := url.Parse(proxyStr)
	if err != nil {
		log.Fatal(err)
	}

	// HTTP client with proxy
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
	}

	// Test GET request
	testRequest(client, "GET", "http://example.com", nil)

	// Test POST request
	postBody := bytes.NewBuffer([]byte("test data"))
	testRequest(client, "POST", "http://httpbin.org/post", postBody)

	// Test HEAD request
	testRequest(client, "HEAD", "http://example.com", nil)
}

func testRequest(client *http.Client, method string, url string, body *bytes.Buffer) {
	var reqBody io.Reader

	if body != nil {
		reqBody = body
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error on %s request: %v\n", method, err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Response for %s request to %s: %s\n", method, url, resp.Status)
	if method != "HEAD" {
		content, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Response body:", string(content))
	}
}
