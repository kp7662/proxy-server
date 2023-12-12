package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/gob"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type CacheEntry struct {
	Value        []byte      // The response body
	Header       http.Header // The response headers
	MaxAge       int64       // Max age of the cache file
	LastModified string      // Last modified date of the cache file
	CreationTime time.Time

	StatusCode int // The response status code // Time when the cache file was created
}
type HTTPCache struct {
	cacheDir string
	cacheMap map[string][]byte // Store serialized CacheEntry data
}

func NewHTTPCache() *HTTPCache {
	cacheDir := "./http_cache"         // Directory to store cache
	os.MkdirAll(cacheDir, os.ModePerm) // Ensure directory exists
	return &HTTPCache{
		cacheDir: cacheDir,
		cacheMap: make(map[string][]byte),
	}
}

func (c *HTTPCache) CacheKey(req *http.Request) string {
	// Create a unique key for the request (e.g., URL + headers)
	key := req.URL.String()
	// Using SHA-1 for simplicity; consider a better hash function for production
	h := sha1.New()
	h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil))
}

func (e CacheEntry) Bytes() []byte {
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)
	err := encoder.Encode(e)
	if err != nil {
		panic(err) // Handle error appropriately
	}
	return buffer.Bytes()
}

func CacheEntryFromBytes(data []byte) *CacheEntry {
	var entry CacheEntry
	buffer := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buffer)
	err := decoder.Decode(&entry)
	if err != nil {
		panic(err) // Handle error appropriately
	}
	return &entry
}
func (c *HTTPCache) Put(req *http.Request, resp *http.Response, maxAge int64, lastModified string) (bod bytes.Buffer) {
	key := c.CacheKey(req)
	filePath := filepath.Join(c.cacheDir, key)

	var bodyBuffer bytes.Buffer
	_, err := io.Copy(&bodyBuffer, resp.Body)
	if err != nil {
		log.Printf("Error copying response body: %v", err)
		return
	}
	//???????????????????????????
	b := bodyBuffer
	entry := CacheEntry{
		Value:        b.Bytes(),
		Header:       resp.Header,
		MaxAge:       maxAge,
		LastModified: lastModified,
		CreationTime: time.Now(),
		StatusCode:   resp.StatusCode,
	}

	serializedData := entry.Bytes()

	err = os.WriteFile(filePath, serializedData, 0666)
	if err != nil {
		log.Printf("Error writing cache file: %v", err)
	}
	return b
}
func (c *HTTPCache) Get(req *http.Request) (*http.Response, bool) {
	key := c.CacheKey(req)
	filePath := filepath.Join(c.cacheDir, key)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false
		}
		log.Printf("Error reading cache file: %v", err)
		return nil, false
	}

	entry := CacheEntryFromBytes(data)
	// if time.Now().After(entry.CreationTime.Add(time.Duration(entry.MaxAge) * time.Second)) {
	// 	// Cache is stale
	// 	return nil, false
	// }

	if (entry).isStale() {
		log.Printf("Cache entry for key: %s is stale. Removing...\n", key)
		c.RemoveCache(key)
		return nil, false
	}

	response := &http.Response{
		StatusCode: entry.StatusCode, // Default value, adjust as needed
		Body:       io.NopCloser(bytes.NewBuffer(entry.Value)),
		Header:     entry.Header,
	}

	return response, true
}

func init() {
	gob.Register(http.Header{})
}

// Add a method to remove a cache entry
func (c *HTTPCache) RemoveCache(key string) {
	filePath := filepath.Join(c.cacheDir, key)
	log.Printf("Removing filePath (%s) from Cache...\n", filePath)

	// Remove the cache file
	err := os.Remove(filePath)
	if err != nil {
		log.Printf("Error removing filePath (%s) from Cache: %v\n", filePath, err)
		return
	}

	log.Printf("FilePath (%s) removed from Cache successfully!\n", filePath)

	//delete(c.cacheMap, key) // Remove the entry from the map
}
func (c CacheEntry) isStale() bool {
	if c.MaxAge == 0 {
		// If maxAge is 0, the cache is always considered stale.
		log.Println("Cache is stale: maxAge is 0")
		return true
	} else if c.MaxAge > 0 {
		// Calculate the age of the cache entry.
		age := time.Since(c.CreationTime) // in nanoseconds

		// Convert maxAge to a duration.
		maxAgeDuration := time.Duration(c.MaxAge) * time.Second // in nanoseconds

		// Check if the age of the cache entry exceeds its maxAge.
		if age > maxAgeDuration {
			log.Printf("Cache is stale: age (%v) > maxAge (%v)\n", age, maxAgeDuration)
			return true
		} else {
			log.Printf("Cache is not stale: age (%v) <= maxAge (%v)\n", age, maxAgeDuration)
			return false
		}
	}

	// If maxAge is negative, consider the cache entry stale by default.
	log.Println("Cache is stale: maxAge is negative")
	return true
}
