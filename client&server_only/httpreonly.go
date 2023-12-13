package main

import (
	//"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"
	//"os"
) /*When creating an HTTP-based API, such as a REST API, a user may need to send more data than can be included in URL length limits, or your page may need to receive data that isn’t about how the data should be interpreted, such as filters or result limits. In these cases, it’s common to include data in the request’s body and to send it with either a POST or a PUT HTTP request.*/

const keyServerAddr = "serverAddr"

func main() {
	// Set up the HTTP server
	mux := http.NewServeMux()

	// Define handlers for different routes
	mux.HandleFunc("/", handleRoot)
	mux.HandleFunc("/hello", handleHello)

	ctx, cancelCtx := context.WithCancel(context.Background())
	serverOne := &http.Server{
		Addr:    ":3333",
		Handler: mux,
		BaseContext: func(l net.Listener) context.Context {
			ctx = context.WithValue(ctx, keyServerAddr, l.Addr().String())
			return ctx
		},
	}
	serverTwo := &http.Server{
		Addr:    ":4444",
		Handler: mux,
		BaseContext: func(l net.Listener) context.Context {
			ctx = context.WithValue(ctx, keyServerAddr, l.Addr().String())
			return ctx
		},
	}

	// Start the HTTP server in a goroutine
	/* go func() {
	        server := http.Server{
				//port, http.Handler
	            Addr:    fmt.Sprintf(":%d", 3333),
	            Handler: mux,
	        }
	        err := server.ListenAndServe()
			if errors.Is(err, http.ErrServerClosed) {
				  fmt.Printf("server closed\n")
			  } else if err != nil {
				  fmt.Printf("error starting server: %s\n", err)
				  os.Exit(1)
	        }
	    }()*/
	go func() {
		err := serverOne.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("server one closed\n")
		} else if err != nil {
			fmt.Printf("error listening for server one: %s\n", err)
		}
		cancelCtx()
	}()
	go func() {
		err := serverTwo.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("server two closed\n")
		} else if err != nil {
			fmt.Printf("error listening for server two: %s\n", err)
		}
		cancelCtx()
	}()

	<-ctx.Done()
	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Make a GET request
	//  makeGetRequest()

	// Make a POST request
	//    makePostRequest()
}

// this function defines the handlerfunc signature
func handleRoot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	//fmt.Printf("%s: got / request\n", ctx.Value(keyServerAddr))
	//io.WriteString(w, "This is my website!\n")
	second := r.URL.Query().Get("second")

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("could not read body: %s\n", err)
	}

	fmt.Printf("%s: got / request. first(%t)=%s, second(%t)=%s, body:\n%s\n",
		ctx.Value(keyServerAddr), second,
		body)
	io.WriteString(w, "This is my website!\n")
}

func handleHello(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	fmt.Printf("%s: got /hello request\n", ctx.Value(keyServerAddr))
	myName := r.PostFormValue("myName")
	if myName == "" {
		w.Header().Set("x-missing-field", "myName")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	io.WriteString(w, "Hello, HTTP!\n")
}

/*
func handlePost(w http.ResponseWriter, r *http.Request) {
    reqBody, err := ioutil.ReadAll(r.Body)
    if err != nil {
        fmt.Fprintf(w, "Error reading request body: %s", err)
        return
    }
    fmt.Printf("Received POST request with body: %s\n", string(reqBody))
    fmt.Fprintf(w, `{"message": "Received!"}`)
}

func makeGetRequest() {
    res, err := http.Get(fmt.Sprintf("http://localhost:%d", serverPort))
    if err != nil {
        fmt.Printf("client: error making http request: %s\n", err)
        return
    }
    defer res.Body.Close()
    fmt.Printf("client: got response from GET request!\n")
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

    resBody, err := ioutil.ReadAll(res.Body)
    if err != nil {
        fmt.Printf("client: could not read response body: %s\n", err)
        return
    }
    fmt.Printf("client: response body from POST request: %s\n", resBody)
}
*/
