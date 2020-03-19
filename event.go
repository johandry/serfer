package serfer

import (
	"strings"

	"github.com/hashicorp/serf/serf"
)

// Event is an alias for the Serf's Event type
type Event = serf.Event

// UserEvent is an alias for the Serf's UserEvent type
type UserEvent = serf.UserEvent

// Query is an alias for the Serf's Query type
type Query = serf.Query

// MemberEvent is an alias for the Serf's MemberEvent type
type MemberEvent = serf.MemberEvent

// QueryResponse is an alias for the Serf's QueryResponse type
type QueryResponse struct {
	*serf.QueryResponse
}

// Response is a query response container
type Response struct {
	Acks      []string
	Responses map[string]string
}

// Event send a user event to the cluster
func (s *Serfer) Event(name string, payload []byte) error {
	return s.cluster.UserEvent(name, payload, defCoalesce)
}

// Query send a query to the cluster and waits for an answer that is returned
func (s *Serfer) Query(name string, payload []byte, params *serf.QueryParam) (*QueryResponse, error) {
	if params == nil {
		params = s.cluster.DefaultQueryParams()
		params.RequestAck = true
	}

	r, err := s.cluster.Query(name, payload, params)

	return &QueryResponse{r}, err
}

// GetResponse get a reponse from a query
func (q *QueryResponse) GetResponse() (*Response, error) {
	defer q.Close()
	r := Response{
		Responses: make(map[string]string),
	}

	for !q.Finished() {
		select {
		case ack := <-q.AckCh():
			// log.Printf("[DEBUG] serfer: GetResponse() - Received acknowledge from query channel = %+v", ack)
			if len(ack) != 0 {
				r.Acks = append(r.Acks, ack)
			}
		case resp := <-q.ResponseCh():
			// log.Printf("[DEBUG] serfer: GetResponse() - Received response from query channel = %+v", resp)
			if len(resp.From) != 0 {
				r.Responses[resp.From] = strings.TrimSuffix(string(resp.Payload), "\n")
			}
		}
	}

	return &r, nil
}

// Event2UserEvent casts and returns the event as a User event type
func Event2UserEvent(e Event) (*UserEvent, bool) {
	// log.Printf("[DEBUG] serfer: Event2UserEvent() - %d == %d?", e.EventType(), serf.EventUser)
	if e.EventType() != serf.EventUser {
		return nil, false
	}
	ue, ok := e.(serf.UserEvent)
	return &ue, ok
}

// Event2Query casts and returns the event as a Query event type
func Event2Query(e Event) (*Query, bool) {
	if e.EventType() != serf.EventQuery {
		return nil, false
	}
	q, ok := e.(*serf.Query)
	return q, ok
}

// Event2MemberEvent casts and returns the event as a Member event type
func Event2MemberEvent(e Event) (*MemberEvent, string, bool) {
	// log.Printf("[DEBUG] serfer: Event2MemberEvent() - %d == %d?", e.EventType(), serf.EventMemberJoin)
	t := e.EventType()
	var meType string
	switch t {
	case serf.EventMemberJoin:
		meType = "join"
	case serf.EventMemberLeave:
		meType = "leave"
	case serf.EventMemberFailed:
		meType = "failed"
	case serf.EventMemberUpdate:
		meType = "update"
	case serf.EventMemberReap:
		meType = "reap"
	default:
		return nil, "", false
	}
	me, ok := e.(serf.MemberEvent)
	return &me, meType, ok
}

// validEventType returns true if the given event type is correct.
// The empty string and '*' are considered a type that match any type
func validEventType(etype string) bool {
	switch etype {
	// case "member-join":
	// case "member-leave":
	// case "member-failed":
	// case "member-update":
	// case "member-reap":
	case "member":
	case "user":
	case "query":
	case "*":
	case "":
	default:
		return false
	}
	return true
}
