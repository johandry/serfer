package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/johandry/serfer"
)

var leader string

func init() {
	flag.StringVar(&leader, "join", "", "Address of leader to join")
}

func main() {
	flag.Parse()

	serfer.MemberEventHandleFunc("join", join)
	serfer.QueryEventHandleFunc("status", status)
	serfer.UserEventHandleFunc("useless", useless)

	key := []string{
		base64.StdEncoding.EncodeToString([]byte("SuperSecureToken")),
	}

	// panic(serfer.StartSecureAndWait(key, nil, nil, leader))
	s, err := serfer.StartSecure(key, nil, nil, leader)
	if err != nil {
		panic(err)
	}
	log.Printf("Starting serfer node on %s:%d", s.BindAddr, s.BindPort)
	s.Wait()
}

func join(e serfer.Event) {
	m, _, ok := serfer.Event2MemberEvent(e)
	if !ok {
		log.Printf("[ERROR] This event Member type")
		return
	}
	members := []string{}
	for _, memb := range m.Members {
		members = append(members, fmt.Sprintf("{ %s\t%s\t%s:%d\t[%v] }", memb.Name, memb.Status, memb.Addr, memb.Port, memb.Tags))
	}
	log.Printf("members: %s", strings.Join(members, ", "))
}

func status(e serfer.Event) {
	q, ok := serfer.Event2Query(e)
	if !ok {
		log.Printf("[ERROR] This is not a Query type")
		return
	}
	status := "I am OK, thanks for asking"
	if err := q.Respond([]byte(status)); err != nil {
		log.Printf("[ERROR] Failed to respond query 'status'. %s", err)
		return
	}
	log.Printf("[INFO] query %q number %d received with payload %q and the reply was %q", q.Name, q.LTime, q.Payload, status)
}

func useless(e serfer.Event) {
	ue, ok := serfer.Event2UserEvent(e)
	if !ok {
		log.Printf("[ERROR] This event is not an UserEvent type")
		return
	}
	log.Printf("Received user event %q number %d with payload %q", ue.Name, ue.LTime, ue.Payload)
}
