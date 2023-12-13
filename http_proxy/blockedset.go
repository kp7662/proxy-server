package main

import (
	"bufio"
	"os"
	"regexp"
)

// BlockedSet represents a structure for storing a set of blocked domain patterns
// It contains a slice (a dynamically-sized, flexible list) of pointers to regexp
// Each slice stores the compiled regular expressions for the domain patterns that we want to block
// Our blocked sites is written in blocked-domains.txt
type BlockedSet struct {
	blockedDomains []*regexp.Regexp
}

// NewBlockedSet populates an array with sites it reads from a given file and
// returns a pointer to a BlockedSet and any error encountered during the process
func NewBlockedSet(filename string) (*BlockedSet, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var domains []*regexp.Regexp
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		domain, err := regexp.Compile(scanner.Text())
		if err != nil {
			return nil, err
		}
		domains = append(domains, domain)
	}

	return &BlockedSet{blockedDomains: domains}, scanner.Err()
}

// IsBlocked checks if the given domain is in the blocked domains array
// It returns true if the domain matches any of the regular expressions 
// in the BlockedSet, indicating that the domain is blocked
func (bs *BlockedSet) IsBlocked(domain string) bool {
	for _, d := range bs.blockedDomains {
		if d.MatchString(domain) {
			return true
		}
	}
	return false
}
