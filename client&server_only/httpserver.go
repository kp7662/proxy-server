// Implemented following https://www.digitalocean.com/community/tutorials/how-to-make-an-http-server-in-go 
// A simple HTTP server in Go, so that one can understand the way it works 

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
) 

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
}

// this function defines the handlerfunc signature
func handleRoot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

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
*/