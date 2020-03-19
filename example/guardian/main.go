package main

import (
	"encoding/base64"
	"flag"
	"log"
	"time"

	"github.com/johandry/serfer"
)

var leader string

func init() {
	flag.StringVar(&leader, "join", "", "Address of leader to join")
}

func main() {
	flag.Parse()

	key := []string{
		base64.StdEncoding.EncodeToString([]byte("SuperSecureToken")),
	}

	// panic(serfer.StartSecureAndWait(key, nil, nil, leader))
	s, err := serfer.StartSecure(key, nil, nil, leader)
	if err != nil {
		panic(err)
	}
	log.Printf("Starting guardian on %s:%d", s.BindAddr, s.BindPort)

	tick2s := time.NewTicker(2 * time.Second).C
	tick5s := time.NewTicker(5 * time.Second).C

	for {
		select {
		// Send event every 2 seconds
		case <-tick2s:
			s.Event("useless", []byte("this is a payload"))

		// Send a query every 5 seconds
		case <-tick5s:
			q, err := s.Query("status", []byte("are you ok?"), nil)
			if err != nil {
				log.Printf("[ERROR] sending query 'status'. %s ", err)
				break
			}
			go func(q *serfer.QueryResponse) {
				r, err := q.GetResponse()
				if err != nil {
					log.Printf("[ERROR] getting response from query 'status'. %s ", err)
					return
				}
				log.Printf("Response from query 'status':")
				for from, resp := range r.Responses {
					log.Printf("  From %s: %s", from, resp)
				}
				log.Printf("Total Ack: %d, Total Response: %d", len(r.Acks), len(r.Responses))
			}(q)
		}
	}
}
