# Serfer

Go module to use Serf from Go. It's not a Serf wrapper.

## Install

go get -u github.com/johandry/serfer

## Use

Using Serfer is like using `net/http` to create an HTTP server for a Web application. To create a simple HTTP server to handle all the request to the web root ("/") we create a handler function _rootHandler(w, r)_ and link it to the path, like this:

    func rootHandler(w http.ResponseWriter, r *http.Request) {
      fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
    }
    func main() {
      http.HandleFunc("/", rootHandler)
      log.Fatal(http.ListenAndServe(":8080", nil))
    }

When a HTTP client (i.e. `curl`) gets `http://localhost:8080/some/path` it will print `Hello, "/some/path"`.

With Serfer we will do something similar to start a Serfer node as leader that will log any User Event received with the event handler function _anyUserEvent(e)_:

    func anyUserEvent(e serfer.Event) {
      ue, ok := serfer.Event2UserEvent(e)
      if !ok {
        log.Printf("[ERROR] This is not a UserEvent")
        return
      }
      log.Printf("[DEBUG] UserEvent %q received with payload %q", ue.Name, ue.Payload)
    }

    func main() {
      serfer.UserEventHandleFunc("", anyUserEvent)
      log.Fatal(serfer.StartAndWait(nil, nil))
    }

When a node join to the cluster and trigger a User Event, the leader will catch it and print the event name and payload.

Read the [Simple Example Code](#simple_example_code) below to see a more useful example.

## Event Types

Serfer can handle 3 type of events: **Membership Events**, **User Events** and **Queries**. For every event we have to implement a handler to deal with that event when it is received. There are two functions per event type to link an event name to a handler: one linking the event name to a function and other linking the event name to a handler.

Every handler function receive an Event interface. In order to use the event information, it is required to convert that event to the expected event type. There are 3 functions, one for each type, to do this conversion. Each function returns the expected event and a boolean which will be true if the event was sucessfully casted.

    func Event2UserEvent(e Event) (*UserEvent, bool)
    func Event2Query(e Event) (*Query, bool)
    func Event2MemberEvent(e Event) (*MemberEvent, string, bool)

There are also a functions to send User Events and Queries but not to send Membership Event as they are triggered automatically when there is a change in the cluster.

    func (s *Serfer) Event(name string, payload []byte) error
    func (s *Serfer) Query(name string, payload []byte, params *serf.QueryParam) (*QueryResponse, error)

### Membership Event

Membership Events are built-in events (from Serf) and are the most important events, they are: **Join**, **Leave**, **Failed**, **Update** and **Reap**. To know more about them read the Serf documentation [about types of events](https://www.serf.io/intro/getting-started/event-handlers.html#types-of-events)

With Serfer we define a membership event handler with:

    func MemberEventHandle(name string, eventHandler EventHandler)
    func MemberEventHandleFunc(name string, eventHandler func(Event))

And to convert the received event interface to the Member Event, use the function:

    func Event2MemberEvent(e Event) (*MemberEvent, string, bool)

Examples:

Create a `join()` function to list the existing members (including the new member) of the cluster, then link that function to the `join` event:

    func join(e serfer.Event) {
      m, _, ok := serfer.Event2MemberEvent(e)
      if !ok {
        log.Printf("[ERROR] This is not a Member type")
        return
      }
      members := []string{}
      for _, memb := range m.Members {
        members = append(members, fmt.Sprintf("{ %s\t%s\t%s:%d\t[%v] }", memb.Name, memb.Status, memb.Addr, memb.Port, memb.Tags))
      }
      log.Printf("members: %s", strings.Join(members, ", "))
    }

    serfer.MemberEventHandleFunc("join", join)

Link the `leave` event to an anonymous function that will notify it's leaving:

    serfer.MemberEventHandleFunc("leave", func(e serfer.Event){
      // Not interested in the event, just validate it's a Member type
      _, _, ok := serfer.Event2MemberEvent(e)
      if !ok {
        log.Printf("[ERROR] This is not a Member type")
        return
      }
      log.Print("Leaving the cluster, good bye everyone!")
    })

### User Event

User Events are custom events defined by the users. With User Events it's possible to execute actions in every node of the cluster such as deploy a code, install or upgrade a package, add an IP to the load balancer, among others. An event receive a payload, a text (`[]byte`) with less than 512 Bytes. The payload can be used to send information to the event, for example, what package to upgrade or what version to deploy.

To link a user event name to a handler function we use the functions:

    func UserEventHandle(name string, eventHandler EventHandler)
    func UserEventHandleFunc(name string, eventHandler func(Event))

To convert the reveiced event interface to a User Event, use the function:

    func Event2UserEvent(e Event) (*UserEvent, bool)

And, to make a node to send user events, use the function:

    func (s *Serfer) Event(name string, payload []byte) error

To use the `Event()` function is required the serfer instance so, to start the cluster use `Start()` or `StartSecure()` to get the serfer instance, do not use any `Start[Secure]AndWait()` function. Read the [Leaders and Followers](#leaders_and_followers) and [Security](#security) section below.

Examples:

React to an event named "5secEvent" with a anonymous function that will print the event name, payload and the [lamport order number](https://en.wikipedia.org/wiki/Lamport_timestamps) or the times this event was sent to the cluster.

    serfer.UserEventHandleFunc("5secEvent", func(e serfer.Event){
      ue, ok := serfer.Event2UserEvent(e)
      if !ok {
        log.Printf("[ERROR] This event is not an UserEvent type")
        return
      }
      log.Printf("Received user event %q number %d with payload %q", ue.Name, ue.LTime, ue.Payload)
    })

This is a serfer node that join to the cluster and send an event every 5 second:

    s, _ := serfer.Start(nil, nil, "192.168.0.1", "192.168.0.2")
    tick5s := time.NewTicker(5 * time.Second).C
    for {
      select {
      case <-tick5s:
        s.Event("5secEvent", []byte("With this payload"))
      }
    }

### Query

A Query is like a User Event but replying a response to the node that send the query. A query also contain a payload and reply the response using the function `Respond(resp []byte)` of the Query.

The part that send the query receive a `serfer.QueryResponse` struct that is used to get the reponse using the function `GetResponse()`. `GetResponse()` will return the response from every node in the cluster when all the responses are received or when the query timeout. Notice that this function will stop the flow of your program so may be a good practice to get the response in a gorutine.

To link a query name to a function that will handle and reply to the query use the following functions:

    func QueryEventHandle(name string, eventHandler EventHandler)
    func QueryEventHandleFunc(name string, eventHandler func(Event))

And to convert the event interface recevied in the query handler function to a Query, use the function:

    func Event2Query(e Event) (*Query, bool)

Example:

Every node in the cluster will handle to the query "status" with a query handler anonymous function. In this query handler function we send the response using `Respond(resp []byte)`:

    serfer.QueryEventHandleFunc("status", func(e serfer.Event) {
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
    })

In the query sender we get the `serfer.QueryResponse` and use its `GetResponse()` function to get the response from every node.

    s, _ := serfer.Start(nil, nil, "192.168.0.1", "192.168.0.2")
    tick30s := time.NewTicker(30 * time.Second).C
    for {
      select {
      case <-tick30s:
        q, err := s.Query("status", "Are you OK?")
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

The responses are collected with a gorutine to not stop the flow of the program.

## Leaders and Followers

In a Serfer cluster there is no master and slaves or workers, it's an impartial cluster. However, to join into a cluster the newborn node needs to know at least one node in the cluster, so it can "ask" to this node to join it to the cluster. 

A **Leader** is a node that its IP address is public so every new node can request it to join to the cluster. There could be more than one leader (that's a suggested practice in case a leader goes down) and a new node can request them all (at same time) to join into the cluster.

A **Follower** or just **Node** is a node in the cluster that could be a leader whenever it's required by just let others know its address.

Different functions can be used to start a node, in every function the lastest parameter is the leaders address list to request a join. These functions are:

    func Start(tags map[string]string, eventHandler EventHandler, leadersAddr ...string) (*Serfer, error)
    func StartAndWait(tags map[string]string, eventHandler EventHandler, leadersAddr ...string) error

Examples:

Start a cluster as a leader and return the serfer service:

    s, err := serfer.Start(nil, nil)

Start a node and request to join into the cluster to one leader:

    s, err := serfer.Start(nil, nil, "192.168.0.1:7947")

Start a node and request to join into the cluster to multiple leaders and keep it running until it ends:

    err := serfer.StartAndWait(nil, nil, "192.168.0.1", "192.168.0.2")

## Security

Same as with `net/http`, we can start a server using TLS certificates, we can do the same with `serfer`, start a cluster using an encrypted token. Those nodes that don't have such tokens cannot join to the cluster neither send or receive events.

The token needs to be an encrypted string of 16, 24 or 32 characters. To generate a token from the terminal use the command `base64` sending a predefined string or a random one:

    echo -n "1234567890123456" | base64
    head -c16 /dev/urandom | base64

(_It's important the `-n` flag in the echo, otherwise the text will include a new line and will be 17 characters instead of 16._)

Or, generate the token using Go with:

    import (
      "crypto/rand"
      "encoding/base64"
    )

    func randomBytes(n int) []byte {
      b := make([]byte, n)
      _, err := rand.Read(b)
      if err != nil {
        panic(err)
      }
      return b
    }

    func token(n int) string {
      return base64.StdEncoding.EncodeToString(randomBytes(n))
    }

It's possible to start the node with more than one token, if one of them match with one of the cluster tokens then the node is allowed to join and send or receive events. A list of tokens is passed as parameter to the following Start functions:

    func StartSecure(keys []string, tags map[string]string, eventHandler EventHandler, leadersAddr ...string) (*Serfer, error)
    func StartSecureAndWait(keys []string, tags map[string]string, eventHandler EventHandler, leadersAddr ...string) error

Examples:

Start a secure cluster as a leader:

    keys := []string{
      token(16),
      token(32),
      base64.StdEncoding.EncodeToString([]byte("1234567890123456")),
    }
    serfer.Start(keys, nil, nil)

Start a node and request to join into the cluster to one leader:

    key := []string{
      base64.StdEncoding.EncodeToString([]byte("1234567890123456")),
    }
    serfer.StartSecure(key, nil, nil, "192.168.0.1:7947")

Start a node and request to join into the cluster to multiple leaders and keep it running until it ends:

    key := []string{
      base64.StdEncoding.EncodeToString([]byte("1234567890123456")),
    }
    serfer.StartSecureAndWait(key, nil, nil, "192.168.0.1", "192.168.0.2")

## Settings

When a node starts it will set some default parameters such as the address to bind and advertise. These parameters can be set using the `New()` function which will return a serfer instance, then call the `Start()` or `StartAndWait()` function to start the service.

    func New(addr string, advAddr string, eventHandler EventHandler, tags map[string]string, keys []string, leadersAddr ...string) (*Serfer, error)

After we receive the server instance, many other parameters can be modified or obtained from the default configuration. These parameters are:
    
* **BindAddr**:     IP address bind to the service
* **BindPort**:     Port used by the service
* **AdvAddr**:      IP address advertised or exposed to provide the service
* **AdvPort**:      Port advertised
* **LeadersAddr**:  List of leaders address
* **Keys**:         List of encrypted keys
* **EventHandler**: Event Handler, if nil the Default Handler will be used
* **Conf**:         Serf configuration, use or modify it before start the service

## Tags

Every node may have roles and responsibilities in a cluster. The Tags are used to assign a role or function in a cluster to a node.

Tags can be assigned when the cluster is started, it's the first parameter in a `Start()` function and the second in a `StartSecure()` function (with `AndWait` or without it).

Tags is a map of string keys to string values, where the key is the tag name. Lets say a node has the role Coordinator in a development Presto cluster located in AWS for the client/user ClientNumeroUno, then I assing the tags: 

    tags := map[string]string{
      "role": "coordinator",
      "environment": "development",
      "cluster": "presto",
      "infrastructure": "AWS",
      "user": "ClientNumeroUno",
    }

Example:

Start a cluster with the role `leader` if there is no node to join, otherwise join to the cluster with the role `minion`:

    func start(joinAddr string) (*serfer.Serfer, error) {
      var role string
      if len(joinAddr) == 0 {
        role = "leader"
      } else {
        role = "minion"
      }
      tags := map[string]string{
        "role": role,
      }
      return serfer.Start(tags, nil, joinAddr)
    }

The Tags can also be assigned before starting the cluster (and after create the Serfer instance) or when the service is running. If the tags are changed when the service is running, every node will be updated of the new tag.

## Simple Example Code

Let's code a serfer node that will be a leader (first node in the cluster) if there is no leader address provided by the user, otherwise will be a regular node and will join to the cluster asking to the leader to join.

node/main.go: [Full Code](#example/node/main.go)

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

Running this code will start a node that will responde to the 'status' query, print useless information when the event 'useless' is received and print the list of nodes in the cluster when a new node join to the cluster.

In a terminal run:

    go build .
    ./node 

The program will display something like `Starting serfer node on 192.168.0.1:7946`, that address (IP and port) is the leader address. In a second terminal session, same directory, run:

    ./node --join 192.168.0.1:7946

Where the address is the leader address from the first terminal session.

Let's make another program to only send events to the cluster and collect the responses to the queries.

guardian/main.go: [Full Code](#example/guardian/main.go)

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

In a 3rd terminal session build it and run it passing the leader address (from first terminal session):

    go build .
    ./guardian --join 192.168.0.1:7946

Then we'll see every 2 seconds the first 2 nodes are printing the 'useless' event information and replying to the 'status' query. The 3rd node, the `guardian`, is printing the response from the other 2 nodes when it ask them their status.

The `node` program also works with `serf`, they speak the same language. For example, while the leader is still running on the first terminal session open a new terminal session, and using `serf` as client, lets connect it to the cluster.

As the cluster is using the key 'SuperSecureToken' we have to encrypt it and pass it as parameter to serf. Try it without the key and it won't be able to connect.

    key=$(echo -n "SuperSecureToken" | base64)
    serf agent -node=foo -bind=127.0.0.1:5000 -rpc-addr=127.0.0.1:7373 -encrypt=${key} -join=192.168.0.15:7946

In another terminal session check the 2 nodes are in the cluster with the `members` subcommand, then send the event and the query:

    serf members
    serf event useless 'with this payload'
    serf query status 'how are you?'

We'll see response from the two nodes but no response from the serf node because it's not setup to reply to the 'status' query.

## TODO

- [ ] Create a QueryParam alias of serf.QueryParam to assign parameters to the query. This is a way to filter the queries by sending it to a list of nodes or to receive response from nodes with specific tags.
- [ ] Create a Members alias to serf.Members to store the list of members.
- [ ] Create Logger to filter Serf output and send Serfer output. There are some anoying debug messages send by Serf.
- [X] Documentation
- [ ] More test files
- [ ] Check if it's necessary to split the code in more files
- [ ] Check if it's necessary to create functions to assign Tags
- [ ] Check if it's necessary to create functions to create QueryParams
- [ ] Evaluate if it is useful events URI's and where to implement them (Serfer or Reeve). 
      An URI is like: 
        serf://[ host[, host ...] ]/[ event type ]/[ pool ]/[ path ]/name?[payload=<payload>][;tag1=value1][;tag2=value2]...
      Examples:
        serf://host1/event/kubekit/deploy?payload=wave
        serf://host01,host02,host1?/query/kubekit-*/stats?role=worker
        serf://*/query/kubekit-*/stats?role=master