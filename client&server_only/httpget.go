// Implemented following https://www.digitalocean.com/community/tutorials/how-to-make-http-requests-in-go 
// A simple HTTP client in Go, so that one can understand the way it works 

package main

import (
	"fmt"
	"net/http"
	"os"
	"bytes"
	"io"
	"time"
)

const serverPort = 3333

func main() {

	requestURL := fmt.Sprintf("http://localhost:%d", serverPort)
	res, err := http.Get(requestURL)
	if err != nil {
		fmt.Printf("error making http request: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("client: got response!\n")
	fmt.Printf("client: status code: %d\n", res.StatusCode)
}

func makePostRequest() {
    jsonBody := []byte(`{"client_message": "hello, server!"}`)
    bodyReader := bytes.NewReader(jsonBody)
    req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:%d/post", serverPort), bodyReader)
    if err != nil {
        fmt.Printf("client: could not create request: %s\n", err)
        return
    }
    req.Header.Set("Content-Type", "application/json")

    client := http.Client{
        Timeout: 30 * time.Second,
    }

    res, err := client.Do(req)
    if err != nil {
        fmt.Printf("client: error making http request: %s\n", err)
        return
    }
    defer res.Body.Close()

    resBody, err := io.ReadAll(res.Body)
    if err != nil {
        fmt.Printf("client: could not read response body: %s\n", err)
        return
    }
    fmt.Printf("client: response body from POST request: %s\n", resBody)
}
