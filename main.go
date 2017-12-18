package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	"github.com/brutella/hc/log"
	"github.com/gorilla/websocket"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	var db string
	var ip string
	var mac string
	var pin string
	var verbose bool

	flag.StringVar(&ip, "ip", "", "TV IP address")
	flag.StringVar(&mac, "mac", "", "TV MAC address")
	flag.StringVar(&pin, "pin", "83688190", "HomeKit Accessory PIN code")
	flag.StringVar(&db, "db", "/usr/local/var/db/hksamsungtvremote", "Database path")
	flag.BoolVar(&verbose, "v", false, "Enable verbose debug logging")
	flag.Parse()

	if verbose == true {
		log.Debug.Enable()
	}

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
		for {
			time.Sleep(1 * time.Minute)
			acc.Switch.On.SetValue(state(ip))
		}
	}()

	acc.Switch.On.OnValueRemoteUpdate(func(on bool) {
		if on == true {
			log.Info.Println("Turn on")
			err := wol(mac)
			if err != nil {
				log.Debug.Println(err)
			}
		} else {
			log.Info.Println("Turn off")
			err := power(ip)
			if err != nil {
				log.Debug.Println(err)
			}
		}
	})

	config := hc.Config{Pin: pin, StoragePath: db}
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
	_, err := client.Get(url)

	if err != nil {
		return false
	} else {
		return true
	}
}

func wol(macAddr string) error {
	macBytes, err := hex.DecodeString(strings.Join(strings.Split(macAddr, ":"), ""))
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
	c.Close()

	if written != 102 {
		return err
	}

	return nil
}

func power(ip string) error {
	url := fmt.Sprintf("ws://%s:8001/api/v2/channels/samsung.remote.control?name=U2Ftc3VuZ1R2UmVtb3Rl", ip)
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}
	defer c.Close()

	_, _, err = c.ReadMessage()
	if err != nil {
		return err
	}

	msg := "{\"method\":\"ms.remote.control\",\"params\":{\"Cmd\":\"Click\",\"DataOfCmd\":\"KEY_POWER\",\"Option\":\"false\",\"TypeOfRemote\":\"SendRemoteKey\"}}"
	err = c.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
		return err
	}

	time.Sleep(750 * time.Millisecond)

	err = c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		return err
	}

	return nil
}