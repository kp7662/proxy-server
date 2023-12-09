package main

import (
	"io"
	"log"
	"net/http"
	"strings"
)

// Route defines a proxy route
type Route struct {
	Pattern string
	Target  string
}

// ProxyServer struct to hold the proxy server's configuration
type ProxyServer struct {
	Port   string            // Port on which the proxy server listens
	Routes map[string]*Route // Map of routes
}

// NewProxyServer creates a new ProxyServer instance
func NewProxyServer(port string) *ProxyServer {
	return &ProxyServer{
		Port:   port,
		Routes: make(map[string]*Route),
	}
}

// AddProxyRoute adds a new route to the proxy server
func (p *ProxyServer) AddProxyRoute(pattern string, target string) {
	p.Routes[pattern] = &Route{
		Pattern: pattern,
		Target:  target,
	}
}

// ServeHTTP handles incoming requests and forwards them based on the configured routes
func (p *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("Original Request URL: %s", r.URL.String())

	// Check if the request path matches any of the configured routes
	for pattern, route := range p.Routes {
		if strings.HasPrefix(r.URL.Path, pattern) {
			// Modify the request URL to include only the path
			modifiedURL := route.Target + r.URL.Path[len(pattern):]
			log.Printf("Modified Request URL: %s", modifiedURL)

			// Create a new request to forward
			client := &http.Client{}
			req, err := http.NewRequest(r.Method, modifiedURL, r.Body)
			if err != nil {
				http.Error(w, "Error creating request", http.StatusInternalServerError)
				return
			}

			// Copy headers from the original request
			for key, value := range r.Header {
				req.Header.Set(key, strings.Join(value, ","))
			}

			// Remove the protocol and host from the request URL
			req.URL.Scheme = ""
			req.URL.Host = ""
			req.RequestURI = "" // Clear the RequestURI as well to avoid conflicts

			// Forward the request
			resp, err := client.Do(req)
			if err != nil {
				log.Printf("Error forwarding request: %v", err)
				http.Error(w, "Error forwarding request", http.StatusInternalServerError)
				return
			}
			defer resp.Body.Close()

			// Copy the response headers
			for key, value := range resp.Header {
				w.Header().Set(key, strings.Join(value, ","))
			}

			// Set the response status code
			w.WriteHeader(resp.StatusCode)

			// Copy the response body
			io.Copy(w, resp.Body)
			return
		}
	}

	// If no route matches, return a 404 Not Found
	http.NotFound(w, r)
}

func main() {
	// Initialize the proxy server on a specific port (e.g., "8080")
	proxy := NewProxyServer("8080")

	// Add routes to the proxy server
	// Example: proxy.AddProxyRoute("/api", "http://example.com/api")
	// Add more routes as needed
	proxy.AddProxyRoute("/api", "http://example.com/api")
	proxy.AddProxyRoute("/api", "https://en.wikipedia.org/wiki/Cat")

	// Set up an HTTP server and listen on the specified port
	http.Handle("/", proxy)
	log.Printf("Starting proxy server on port %s", proxy.Port)
	log.Fatal(http.ListenAndServe(":"+proxy.Port, nil))
}
