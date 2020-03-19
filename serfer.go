package serfer

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"encoding/base64"

	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/serf"
)

// MaxLeaders defines the maximun number of leaders in a cluster. Recommended
// to be more than 3
const MaxLeaders = 5

// Default values
const (
	defBindAddr = "0.0.0.0"
	defBindPort = 7946
	defCoalesce = true
)

// Serfer encapsulate a Serf cluster configuration
// TODO: Replace log.Logger by an internally defined Logger, like in Terraformer
type Serfer struct {
	BindAddr     string
	BindPort     int
	AdvAddr      string
	AdvPort      int
	LeadersAddr  []string
	Keys         []string
	EventHandler EventHandler
	Conf         *serf.Config
	Logger       *log.Logger
	cluster      *serf.Serf
	eventCh      chan serf.Event
	doneCh       chan struct{}
	mu           sync.Mutex
}

// New creates a Serfer cluster
func New(addr string, advAddr string, eventHandler EventHandler, tags map[string]string, keys []string, leadersAddr ...string) (*Serfer, error) {
	s := new(Serfer)

	// Hostname is required to find the bind address. Serf will panic if cannot
	// find it either when assing the default configuration
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("can't find the hostname")
	}

	// Find host and port if they are not given
	host, port, err := splitHostPort(addr, "", 0)
	if err != nil {
		return nil, fmt.Errorf("can't get host and port from bind address %q. %s", addr, err)
	}
	if len(host) == 0 {
		host, err = getHost(hostname, false)
		if err != nil {
			return nil, fmt.Errorf("can't find an IP to bind serf. %s", err)
		}
	}
	if port == 0 {
		port, err = getPort(host)
		if err != nil {
			return nil, fmt.Errorf("can't find a free port to bind serf on %q. %s", host, err)
		}
	}

	// Serf default configuration:
	conf := serf.DefaultConfig()
	conf.MemberlistConfig = memberlist.DefaultLANConfig()
	conf.Init()

	// Assing host (bind address) and port to Serf configuration:
	s.BindAddr = host
	conf.MemberlistConfig.BindAddr = host
	s.BindPort = port
	conf.MemberlistConfig.BindPort = port

	// Find advertise address from bind address and assign them to Serf configuration:
	advHost, advPort, err := splitHostPort(advAddr, host, port)
	if err != nil {
		return nil, fmt.Errorf("can't get host and port from advertise address %q. %s", advAddr, err)
	}
	conf.MemberlistConfig.AdvertiseAddr = advHost
	conf.MemberlistConfig.AdvertisePort = advPort

	if port != defBindPort {
		conf.NodeName = fmt.Sprintf("%s-%d", hostname, port)
	}

	if eventHandler == nil {
		s.EventHandler = DefaultHandler
	}

	// Assing leaders, if any and those that are a valid address:
	if len(leadersAddr) != 0 {
		lAddr := []string{}
		for _, a := range leadersAddr {
			if isValidAddr(a) {
				lAddr = append(lAddr, a)
			}
		}
		if len(lAddr) != 0 {
			s.LeadersAddr = lAddr
		}
	}

	// Assing the key or keys to Serf configuration:
	if len(keys) == 1 {
		secretKey, err := base64.StdEncoding.DecodeString(keys[0])
		if err != nil {
			return nil, fmt.Errorf("failed decoding the given key. %s", err)
		}
		conf.MemberlistConfig.SecretKey = secretKey
	}
	if len(keys) > 1 {
		kr, err := keyring(keys...)
		if err != nil {
			return nil, err
		}
		s.Conf.MemberlistConfig.Keyring = kr
	}

	if tags != nil {
		conf.Tags = tags
	}

	// Other Serf configuration:

	conf.CoalescePeriod = 3 * time.Second
	conf.QuiescentPeriod = time.Second
	conf.UserCoalescePeriod = 3 * time.Second
	conf.UserQuiescentPeriod = time.Second

	// Channels to communicate with Serf
	ch := make(chan serf.Event, 64)
	conf.EventCh = ch
	s.eventCh = ch

	// All set, return the serfer
	s.Conf = conf
	return s, nil
}

// NewDefault creates an insecure Serfer cluster with the default parameters
func NewDefault(tags map[string]string, leadersAddr ...string) (*Serfer, error) {
	return New("", "", nil, tags, nil, leadersAddr...)
}

// NewSecure creates a secure Serfer cluster with the given keys and the default
// parameters
func NewSecure(keys []string, tags map[string]string, leadersAddr ...string) (*Serfer, error) {
	return New("", "", nil, tags, keys, leadersAddr...)
}

// Start creates an insecure Serf cluster and start it
func Start(tags map[string]string, eventHandler EventHandler, leadersAddr ...string) (*Serfer, error) {
	return StartSecure(nil, tags, eventHandler, leadersAddr...)
}

// StartSecure creates a secure Serf cluster and start
func StartSecure(keys []string, tags map[string]string, eventHandler EventHandler, leadersAddr ...string) (*Serfer, error) {
	s, err := NewSecure(keys, tags, leadersAddr...)
	if err != nil {
		return nil, err
	}
	s.EventHandler = eventHandler
	err = s.Start()
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Serfer) getDoneChan() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getDoneChanLocked()
}

func (s *Serfer) getDoneChanLocked() chan struct{} {
	if s.doneCh == nil {
		s.doneCh = make(chan struct{})
	}
	return s.doneCh
}

func (s *Serfer) closeDoneChan() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeDoneChanLocked()
}

func (s *Serfer) closeDoneChanLocked() {
	ch := s.getDoneChanLocked()
	select {
	case <-ch:
		// It is closed
	default:
		close(ch)
	}
}

// Close closes
func (s *Serfer) Close() {
	s.closeDoneChan()
	// TODO: Close open query channels
}

// Wait will wait until the cluster shutdown or the node leaves the cluster
func (s *Serfer) Wait() {
	for {
		select {
		case <-s.doneCh:
			return
		}
	}
}

// StartAndWait creates an insecure Serf cluster, start it and wait until it's
// close or the node leaves the cluster
func StartAndWait(tags map[string]string, eventHandler EventHandler, leadersAddr ...string) error {
	return StartSecureAndWait(nil, tags, eventHandler, leadersAddr...)
}

// StartSecureAndWait creates a secure Serf cluster, start it and wait until it's
// close or the node leaves the cluster
func StartSecureAndWait(keys []string, tags map[string]string, eventHandler EventHandler, leadersAddr ...string) error {
	s, err := StartSecure(keys, tags, eventHandler, leadersAddr...)
	if err != nil {
		return err
	}
	s.Wait()
	return nil
}

// Leave makes this node leave the Serfer cluster
func (s *Serfer) Leave() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeDoneChanLocked()
	if s.cluster == nil {
		return nil
	}
	return s.cluster.Leave()
}

// ForceLeave eject a node from the cluster
func (s *Serfer) ForceLeave(node string) error {
	return s.cluster.RemoveFailedNode(node)
}

// Serf returns the cluster which is a Serf cluster
func (s *Serfer) Serf() *serf.Serf {
	return s.cluster
}

// SetBindAddr set the bind address which could be IP and port or just IP
func (s *Serfer) SetBindAddr(addr string) error {
	host, port, err := splitHostPort(addr, defBindAddr, defBindPort)
	if err != nil {
		return fmt.Errorf("can't get host and port from given bind address %q. %s", addr, err)
	}
	s.BindAddr = host
	s.Conf.MemberlistConfig.BindAddr = host
	s.BindPort = port
	s.Conf.MemberlistConfig.BindPort = port
	return nil
}

// SetAdvertiseAddr set the advertise address which could be IP and port or just IP
func (s *Serfer) SetAdvertiseAddr(addr string) error {
	var defAddr string
	var defPort int
	if len(s.BindAddr) == 0 {
		defAddr = defBindAddr
	} else {
		defAddr = s.BindAddr
	}
	if s.BindPort == 0 {
		defPort = defBindPort
	} else {
		defPort = s.BindPort
	}

	host, port, err := splitHostPort(addr, defAddr, defPort)
	if err != nil {
		return fmt.Errorf("can't get host and port from given advertise address %q. %s", addr, err)
	}

	s.Conf.MemberlistConfig.AdvertiseAddr = host
	s.Conf.MemberlistConfig.AdvertisePort = port

	return nil
}

// Create creates a new serf cluster
func (s *Serfer) Create() error {
	cluster, err := serf.Create(s.Conf)
	if err != nil {
		return err
	}
	s.cluster = cluster

	return nil
}

// Join joins the node to the serf cluster
func (s *Serfer) Join() (int, error) {
	if len(s.LeadersAddr) == 0 {
		return 0, nil
	}
	if s.cluster == nil {
		return 0, fmt.Errorf("there is no cluster to join to")
	}
	// TODO: Are we going to ignore old message? -> second parameter of Join()
	return s.cluster.Join(s.LeadersAddr, true)
}

// Start starts the Serfer cluster
func (s *Serfer) Start() error {
	if s.cluster == nil {
		if err := s.Create(); err != nil {
			return err
		}
	}

	if _, err := s.Join(); err != nil {
		return err
	}

	go s.loop()

	return nil
}

// loop accept incoming events from the event channel creating a new HandleEvent
// gorutine for each.
func (s *Serfer) loop() error {
	for {
		select {
		case e := <-s.eventCh:
			s.mu.Lock()
			eventHandler := s.EventHandler
			if eventHandler == nil {
				eventHandler = DefaultHandler
			}
			s.mu.Unlock()
			// log.Printf("[DEBUG] serfer: Event Handler: %+v", eventHandler)
			go eventHandler.HandleEvent(e)

		case <-s.cluster.ShutdownCh():
			return s.Leave()

		case <-s.doneCh:
			return nil
		}
	}
}

// StartAndWait starts the cluster to handle events and wait until it's close or
// the node leaves the cluster
func (s *Serfer) StartAndWait() error {
	if err := s.Start(); err != nil {
		return err
	}
	s.Wait()
	return nil
}

func getHost(hostname string, fqdnResp bool) (string, error) {
	myaddrs, _ := net.LookupHost(hostname)

	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if isValidIP(ipnet, myaddrs...) {
					if fqdnResp && len(myaddrs) > 0 {
						return hostname, nil
					}
					return ipnet.IP.String(), nil
				}
			}
		}
	}

	return defBindAddr, nil
}

// isValidIP returns true if this is a good IP to be use for binding Serfer
// The rules are:
// (1) cannot be a loopback address
// (2) has to be IPv4 address
// (3) _if there are IP linked to the hostname_ it has to be one of them
func isValidIP(ip *net.IPNet, myaddrs ...string) bool {
	if ip.IP.IsLoopback() || ip.IP.To4() == nil {
		// IP cannot be a loopback and have to be a IPv4 address
		return false
	}
	if len(myaddrs) > 0 {
		for _, addr := range myaddrs {
			if ip.IP.Equal(net.ParseIP(addr)) {
				// If one address is equal to this IP, then it's a good IP
				return true
			}
		}
		// If any address match with this IP, then it's a bad IP
		return false
	}
	// If you get to this point, the IP is not loopback, it's IPv4 and I have no
	// address to match it, so it's a good IP
	return true
}

func isValidAddr(addr string) bool {
	if len(addr) == 0 {
		return false
	}
	if !strings.Contains(addr, ":") {
		addr = fmt.Sprintf("%s:%d", addr, defBindPort)
	}
	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	return true
}

func getPort(host string) (int, error) {
	// Let's try with default port first
	addr := fmt.Sprintf("%s:%d", host, defBindPort)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		// The port is not available, find one
		return getFreePort(host)
	}
	// The port is available, close the listener and return that port
	defer l.Close()
	return defBindPort, nil
}

func getFreePort(host string) (int, error) {
	addr := fmt.Sprintf("%s:%d", host, 0)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// splitHostPort splits the host and port part of an address replacing missing
// values with default ones
func splitHostPort(addr string, defAddr string, defPort int) (string, int, error) {
	if len(addr) == 0 {
		return "", 0, nil
	}
	newAddr := addr
	if !strings.Contains(addr, ":") {
		newAddr = fmt.Sprintf("%s:%d", addr, defPort)
	}
	host, portStr, err := net.SplitHostPort(newAddr)
	if err != nil {
		return "", 0, err
	}

	if len(host) == 0 {
		host = defBindAddr
	}

	var port int
	if len(portStr) == 0 {
		port = defBindPort
	} else {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return "", 0, fmt.Errorf("failed to get the port form address %s, from %s was found port %s. %s", addr, newAddr, portStr, err)
		}
	}

	return host, port, nil
}
