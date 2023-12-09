package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
)

func main() {
	// Proxy URL
	//Proxy String (proxyStr): This string specifies the URL of your proxy server.
	//In your case, "http://127.0.0.1:9999" means that the proxy server is running on your local machine
	//(127.0.0.1 is the loopback IP address, often referred to as "localhost") and listens on port 9999
	proxyStr := "http://127.0.0.1:9999"
	proxyURL, err := url.Parse(proxyStr)
	if err != nil {
		log.Fatal(err)
	}

	// HTTP client with proxy
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
	}

	// Parsing command line flags for target URLs
	flag.Parse()
	targetURLs := flag.Args()

	if len(targetURLs) == 0 {
		log.Fatal("No URL provided. Usage: go run main.go [http://example.com ...]")
	}

	// Performing GET requests for each provided URL
	for _, target := range targetURLs {
		testRequest(client, "GET", target, nil)
	}
	//test POST
	//Note that in the case of the POST method, in addition to the URL,
	// we must define the body of the request and the content-type of the data.
	//The body of the request is the data we send. Why not simply put a JSON string?
	//Why do we use a bytes.Buffer ? That’s because this argument must implement the interface io.Reader !
	postBody := bytes.NewBuffer([]byte("test data"))
	testRequest(client, "POST", "http://httpbin.org/post", postBody)
	myJson := bytes.NewBuffer([]byte(`{"name":"Maximilien"}`))
	testRequest(client, "POST", "http://httpbin.org/post", myJson)
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

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error on %s request to %s: %v\n", method, url, err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Response for %s request to %s: %s\n", method, url, resp.Status)
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response body for %s: %v\n", url, err)
		return
	}
	fmt.Println("Response body:", string(content))
}