package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type GatewayInfo struct {
	Token  string
	Model  string
	FwVer  string
	Mac    string
	BleFw  string
	BleMac string
	IP     string
	NetIf  string
	BootAt time.Time
}

func (i GatewayInfo) String() string {
	return fmt.Sprintf(
		"token: %s, Model: %s, Fw: %s, Mac: %s, BtFw: %s, BtMac: %s, Net: %s %s, UpTime: %s",
		i.Token, i.Model, i.FwVer, i.Mac, i.BleFw, i.BleMac,
		i.NetIf, i.IP, time.Since(i.BootAt).Truncate(time.Second))
}

func (i GatewayInfo) Identity() string {
	return fmt.Sprintf("%s-%s", i.Model, i.Token[len(i.Token)-4:])
}

func (i GatewayInfo) UpTime() time.Duration {
	return time.Since(i.BootAt).Truncate(time.Second)
}

type GatewayEntry struct {
	Info   GatewayInfo       // gateway info from SYS
	lock   sync.Mutex        // lock for command execution
	cmd    string            // current executing command
	conn   *net.TCPConn      // connection
	chErr  chan error        // channel for error
	chResp chan *CmdResponse // channel for response
}

// empty channels for new command execution
func (gw *GatewayEntry) emptyCh() {
L:
	for {
		select {
		case <-gw.chErr:
		case <-gw.chResp:
		default:
			break L
		}
	}
}

func (gw *GatewayEntry) Connected() bool {
	return gw.conn != nil
}

func (gw *GatewayEntry) NetworkDesc() string {
	if gw.Connected() {
		return fmt.Sprintf("%s %s", gw.Info.NetIf, gw.Info.IP)
	} else {
		return fmt.Sprintf("%s ---", gw.Info.NetIf)
	}
}

func (gw *GatewayEntry) UpTimeDesc() string {
	if gw.Connected() {
		duration := gw.Info.UpTime()
		if duration.Hours() > 24 {
			return fmt.Sprintf(
				"%d days, %d:%d:%d",
				int(duration.Hours())/24,
				int(duration.Hours())%24,
				int(duration.Minutes())%60,
				int(duration.Seconds())%60,
			)
		} else {
			return fmt.Sprintf(
				"%d:%d:%d",
				int(duration.Hours())%24,
				int(duration.Minutes())%60,
				int(duration.Seconds())%60,
			)
		}
	} else {
		// disconnected, this uptime we stored is not reliable anymore
		return "---"
	}
}

type CmdResponse struct {
	result int
	lines  []string
	data   map[string]interface{}
}

var gatewayList map[string]*GatewayEntry = make(map[string]*GatewayEntry)

func (gw *GatewayEntry) execCmd(cmd string, timeout time.Duration) (*CmdResponse, error) {
	id := GoID()
	gw.lock.Lock()
	gw.cmd = cmd
	defer func() {
		gw.cmd = ""
		gw.lock.Unlock()
	}()
	if gw.conn == nil {
		return nil, fmt.Errorf("no connection")
	}
	log.Printf("[%d] CMD %s\n", id, cmd)

	// empty the channels
	gw.emptyCh()
	// write command
	if _, err := gw.conn.Write([]byte(fmt.Sprintf("%s\n", cmd))); err != nil {
		log.Panicf("[%d] write cmd failed: %s\n", id, err)
		return nil, err
	}
	// default value of timeout
	if timeout == 0 {
		timeout = time.Second * 5
	}
	// wait for response or error
	select {
	case err := <-gw.chErr:
		return nil, err
	case resp := <-gw.chResp:
		if resp.result != 0 {
			return resp, fmt.Errorf("RESULT: %d", resp.result)
		} else {
			return resp, nil
		}
	case <-time.After(timeout):
		// timeout for received response from gateway
		return nil, fmt.Errorf("command timeout")
	}
}

func (gw *GatewayEntry) execOTA() (*CmdResponse, error) {
	id := GoID()
	gw.lock.Lock()
	gw.cmd = "SYS OTA START"
	defer func() {
		gw.cmd = ""
		gw.lock.Unlock()
	}()
	if gw.conn == nil {
		return nil, fmt.Errorf("no connection")
	}
	log.Printf("[%d] CMD %s\n", id, gw.cmd)

	// empty the channels
	gw.emptyCh()
	// write command
	if _, err := gw.conn.Write([]byte("SYS OTA START\n")); err != nil {
		log.Panicf("[%d] write cmd failed: %s\n", id, err)
		return nil, err
	}
	// wait for first response or error
	select {
	case err := <-gw.chErr:
		return nil, err
	case resp := <-gw.chResp:
		if resp.result == 0 && strings.HasPrefix(resp.lines[0], "Found ") {
			break
		}
		return resp, nil
	case <-time.After(time.Second * 5):
		// timeout for received response from gateway
		return nil, fmt.Errorf("command timeout")
	}
	// New version found, wait 10s for error
	// otherwise assume the OTA procedure will success
	select {
	case err := <-gw.chErr:
		return nil, err
	case resp := <-gw.chResp:
		return resp, nil
	case <-time.After(time.Second * 10):
		// timeout for received response from gateway
		return nil, fmt.Errorf("command timeout")
	}
}

func (gw *GatewayEntry) getRCHost() (string, error) {
	resp, err := gw.execCmd("SYS CTRLHOST", time.Second*3)
	if err != nil {
		return "", err
	}
	return resp.data["SYS CTRLHOST"].(string), nil
}

func initCmdResponse() *CmdResponse {
	resp := CmdResponse{
		result: -1,
		data:   map[string]interface{}{},
	}
	return &resp
}

func responseReader(gw *GatewayEntry) {
	id := GoID()
	defer func() {
		gw.lock.Lock()
		gw.conn.Close()
		gw.conn = nil
		gw.lock.Unlock()
	}()
	resp := initCmdResponse()
	scanner := bufio.NewScanner(gw.conn)
	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("[%d|reader] %s\n", id, line)
		if strings.HasPrefix(line, "RESULT") {
			fields := strings.Split(line, ":")
			if len(fields) == 2 {
				if val, err := strconv.Atoi(fields[1]); err == nil {
					// get RESULT, write response to channel, create a new resp instance
					log.Printf("[%d|reader] RESULT: %d of %s\n", id, val, gw.cmd)
					resp.result = val
					gw.chResp <- resp
					resp = initCmdResponse()
				}
			}
		} else if len(line) > 0 {
			fields := strings.Split(line, "=")
			if len(fields) == 2 {
				// key = value
				if v, ok := resp.data[fields[0]]; ok {
					// key already exists, convert value into string array
					if values, ok := v.([]string); ok {
						values = append(values, fields[1])
						resp.data[fields[0]] = values
					} else if val, ok := v.(string); ok {
						resp.data[fields[0]] = []string{val, fields[1]}
					}
				} else {
					resp.data[fields[0]] = fields[1]
				}
			} else {
				// store as normal line response
				resp.lines = append(resp.lines, line)
				// special handling for OTA command ...
				if gw.cmd == "SYS OTA START" && strings.HasPrefix(line, "Found v") {
					// after this line, the gateway will start OTA process
					// and the connection will stock here
					// workaround: let it return success and close this connection
					resp.result = 0
					gw.chResp <- resp
					return
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("[%d|reader] Scan error: %s\n", id, err)
		if len(gw.cmd) > 0 {
			gw.chErr <- err
		}
		return
	}
	// connect closed gracefully?
	// never happen in my test case
	log.Printf("[%d|reader] EOF\n", id)
	gw.chErr <- io.EOF
}

func (gw *GatewayEntry) ingestSysInfo(resp *CmdResponse) {
	gw.Info.BleFw = resp.data["BT_FW"].(string)
	gw.Info.BleMac = resp.data["BT_MAC"].(string)
	if val, ok := resp.data["WIFI_MAC"]; ok {
		gw.Info.Mac = strings.ReplaceAll(val.(string), ":", "")
	} else if val, ok := resp.data["ETH_MAC"]; ok {
		gw.Info.Mac = strings.ReplaceAll(val.(string), ":", "")
	}
	if val, ok := resp.data["FIRMWARE_VERSION"]; ok {
		fields := strings.Split(val.(string), "-")
		if len(fields) > 1 {
			gw.Info.FwVer = fields[len(fields)-1]
			gw.Info.Model = strings.Join(fields[0:len(fields)-1], "-")
		}
	}
	// NETIF handling
	if data, ok := resp.data["NETIF"]; ok {
		regex := regexp.MustCompile(`^(.+)\(([\d]+)\) (([\d]{1,3}\.){3}[\d]{1,3}) .*$`)
		if values, ok := data.([]string); ok {
			// multiple network interfaces ...
			prio := 0
			for _, val := range values {
				matches := regex.FindStringSubmatch(val)
				if len(matches) == 5 {
					if p, e := strconv.Atoi(matches[2]); e == nil && p > prio {
						prio = p
						gw.Info.IP = matches[3]
						gw.Info.NetIf = matches[1]
					}
				}
			}
		} else if val, ok := data.(string); ok {
			matches := regex.FindStringSubmatch(val)
			if len(matches) == 5 {
				gw.Info.IP = matches[3]
				gw.Info.NetIf = matches[1]
			}
		}
	}
	// Up time handling
	if val, ok := resp.data["UPTIME"]; ok {
		regex := regexp.MustCompile(`^([\d]+) days, ([\d]+):([\d]+):([\d]+)$`)
		matches := regex.FindStringSubmatch(val.(string))
		d, _ := strconv.Atoi(matches[1])
		h, _ := strconv.Atoi(matches[2])
		m, _ := strconv.Atoi(matches[3])
		s, _ := strconv.Atoi(matches[4])
		gw.Info.BootAt = time.Now().
			AddDate(0, 0, -1*d).
			Add(time.Hour * time.Duration(-1*h)).
			Add(time.Minute * time.Duration(-1*m)).
			Add(time.Second * time.Duration(-1*s))
	}
}

func gatewayHandshake(conn *net.TCPConn) {
	id := GoID()

	// read token first
	token := make([]byte, 17)
	conn.SetReadDeadline(time.Now().Add(time.Second * 3))
	if _, err := conn.Read(token); err != nil {
		log.Printf("[%d] Read token error: %s\n", id, err)
		conn.Close()
		return
	} else {
		p := regexp.MustCompile("^([0-9A-Fa-f]{2}[:]){5}([0-9A-Fa-f]{2})")
		if !p.Match(token) {
			log.Printf("[%d] Invalid token: %s\n", id, token)
			conn.Close()
			return
		}
	}
	conn.SetReadDeadline(time.Time{})

	// prepare gateway entry, reader routine for SYS command
	gw := GatewayEntry{
		Info:   GatewayInfo{Token: strings.ReplaceAll(string(token), ":", "")},
		conn:   conn,
		chErr:  make(chan error),
		chResp: make(chan *CmdResponse),
	}
	go responseReader(&gw)
	if resp, err := gw.execCmd("SYS", time.Second*3); err == nil {
		if resp.result != 0 {
			// SYS command return FAIL, should not happen
			conn.Close()
			return
		}
		// handshake succeeded
		gw.ingestSysInfo(resp)
		gatewayList[gw.Info.Token] = &gw
		log.Printf("[%d] %s\n", id, gw.Info)
		conn.SetKeepAlivePeriod(time.Second * 5)
		conn.SetKeepAlive(true)
	}
}

func InitialzeRCServer() {
	port := 5000
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: port})
	if err != nil {
		log.Printf("Listener: Listen error: %s\n", err)
		os.Exit(1)
	}
	log.Printf("RCServer Listening on %d...\n", port)
	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			log.Printf("Listener: accept error: %s\n", err)
			continue
		}
		log.Printf("Gateway connected from %s\n", conn.RemoteAddr())
		go gatewayHandshake(conn)
	}
}
