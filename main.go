package main

import (
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	"github.com/brutella/hc/log"
	"github.com/gorilla/websocket"
)

var (
	command = flag.String("command", "start", "[start|on|off]")
	db      = flag.String("db", "/usr/local/var/db/hksamsungtvremote", "Database path")
	ip      = flag.String("ip", "", "TV IP address")
	mac     = flag.String("mac", "", "TV MAC address")
	pin     = flag.String("pin", "83688190", "HomeKit Accessory PIN code")
	verbose = flag.Bool("v", false, "Enable verbose debug logging")
)

func main() {
	flag.Parse()

	if *verbose == true {
		log.Debug.Enable()
	}

	if *ip == "" {
		log.Info.Fatal("missing -ip")
	}

	if *mac == "" {
		log.Info.Fatal("missing -mac")
	}

	switch *command {
	case "start":
		start(*mac, *ip)
	case "on":
		if err := powerOn(*mac, *ip); err != nil {
			log.Debug.Println(err)
			os.Exit(1)
		}
	case "off":
		if err := powerOff(*mac, *ip); err != nil {
			log.Debug.Println(err)
			os.Exit(1)
		}
	case "state":
		if state(*ip) {
			fmt.Println("on")
		} else {
			fmt.Println("off")
		}
	default:
		flag.PrintDefaults()
		os.Exit(2)
	}
}

func start(macAddr string, ip string) {
	info := accessory.Info{
		Name:         "Samsung TV Remote",
		Manufacturer: "Samsung",
		Model:        "BN59-01241A",
	}

	acc := accessory.NewSwitch(info)

	acc.Switch.On.OnValueRemoteGet(func() bool {
		return state(ip)
	})

	go func() {
		for _ = range time.NewTicker(1 * time.Minute).C {
			acc.Switch.On.SetValue(state(ip))
		}
	}()

	acc.Switch.On.OnValueRemoteUpdate(func(on bool) {
		if on == true {
			log.Info.Println("Turn on")
			if err := powerOn(macAddr, ip); err != nil {
				log.Debug.Println(err)
			}
		} else {
			log.Info.Println("Turn off")
			if err := powerOff(macAddr, ip); err != nil {
				log.Debug.Println(err)
			}
		}
	})

	config := hc.Config{Pin: *pin, StoragePath: *db}
	t, err := hc.NewIPTransport(config, acc.Accessory)
	if err != nil {
		log.Info.Panic(err)
	}

	hc.OnTermination(func() {
		t.Stop()
		os.Exit(1)
	})

	t.Start()
}

func state(ip string) bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	url := fmt.Sprintf("http://%s:8001/", ip)

	if _, err := client.Get(url); err != nil {
		return false
	}

	return true
}

func powerOn(macAddr string, ip string) error {
	if err := wol(macAddr); err != nil {
		return err
	}

	time.Sleep(750 * time.Millisecond)
	if state(ip) == false {
		return errors.New("wol: timeout")
	}

	return nil
}

func powerOff(macAddr string, ip string) error {
	return power(ip)
}

func wol(macAddr string) error {
	macBytes, err := hex.DecodeString(strings.Replace(macAddr, ":", "", -1))
	if err != nil {
		return err
	}

	b := []uint8{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	for i := 0; i < 16; i++ {
		b = append(b, macBytes...)
	}

	a, err := net.ResolveUDPAddr("udp", "255.255.255.255:9")
	if err != nil {
		return err
	}

	c, err := net.DialUDP("udp", nil, a)
	if err != nil {
		return err
	}

	written, err := c.Write(b)
	defer c.Close()

	if written != 102 {
		return err
	}

	return nil
}

func power(ip string) error {
	url := fmt.Sprintf("ws://%s:8001/api/v2/channels/samsung.remote.control?name=U2Ftc3VuZ1R2UmVtb3Rl", ip)
	dialer := &websocket.Dialer{
		HandshakeTimeout: 500 * time.Millisecond,
	}
	c, _, err := dialer.Dial(url, nil)

	if err != nil {
		return err
	}
	defer c.Close()

	if _, _, err := c.ReadMessage(); err != nil {
		return err
	}

	if err := c.WriteJSON(&struct {
		Method string                 `json:"method"`
		Params map[string]interface{} `json:"params"`
	}{
		Method: "ms.remote.control",
		Params: map[string]interface{}{
			"Cmd":          "Click",
			"DataOfCmd":    "KEY_POWER",
			"Option":       "false",
			"TypeOfRemote": "SendRemoteKey",
		},
	}); err != nil {
		return err
	}

	time.Sleep(750 * time.Millisecond)

	if err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
		return err
	}

	return nil
}
