package main

import (
	"bufio"
	"os"
	"regexp"
)

type BlockedSet struct {
	blockedDomains []*regexp.Regexp
	//This field is a slice (a dynamically-sized, flexible list) of pointers to regexp.
	//Regexp objects. Each regexp.Regexp object represents a compiled regular expression.
	//This slice will store the compiled regular expressions for the domains that you want to block.
}

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

func (bs *BlockedSet) IsBlocked(domain string) bool {
	for _, d := range bs.blockedDomains {
		if d.MatchString(domain) {
			return true
		}
	}
	return false
}
