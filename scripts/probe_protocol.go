//go:build ignore

// probe_protocol.go — M0.3 Socket.IO protocol explorer
//
// Connects raw WebSocket to Uptime Kuma, performs Engine.IO handshake,
// then logs ALL frames received before and after a login event.
// Run with: go run ./scripts/probe_protocol.go
//
// Required env vars (or edit defaults below):
//   KUMA_URL      e.g. http://localhost:3002
//   KUMA_USER     admin username
//   KUMA_PASS     admin password

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"nhooyr.io/websocket"
)

const (
	defaultURL  = "http://localhost:3002"
	defaultUser = "admin"
	defaultPass = "admin"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Engine.IO handshake response
type eioHandshake struct {
	SID          string   `json:"sid"`
	Upgrades     []string `json:"upgrades"`
	PingInterval int      `json:"pingInterval"`
	PingTimeout  int      `json:"pingTimeout"`
}

func main() {
	kumaURL := env("KUMA_URL", defaultURL)
	user := env("KUMA_USER", defaultUser)
	pass := env("KUMA_PASS", defaultPass)

	// Convert http → ws URL and append Engine.IO path
	wsURL := strings.Replace(kumaURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL += "/socket.io/?EIO=4&transport=websocket"

	log.Printf("→ Connecting to %s", wsURL)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Origin": []string{kumaURL},
		},
	})
	if err != nil {
		log.Fatalf("dial error: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "probe done")

	log.Println("✓ WebSocket connected")

	frameCount := 0
	sentLogin := false

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			log.Printf("read error: %v", err)
			break
		}

		frameCount++
		raw := string(data)
		fmt.Printf("\n─── Frame #%d ───────────────────────────────\n", frameCount)
		fmt.Printf("RAW: %s\n", raw)

		// Parse Engine.IO prefix
		if len(raw) == 0 {
			continue
		}

		eioType := string(raw[0])
		payload := raw[1:]

		switch eioType {
		case "0": // EIO open - server handshake
			var hs eioHandshake
			if err := json.Unmarshal([]byte(payload), &hs); err == nil {
				fmt.Printf("EIO OPEN → sid=%s pingInterval=%dms pingTimeout=%dms\n",
					hs.SID, hs.PingInterval, hs.PingTimeout)
			}
			// Send EIO upgrade / SIO connect
			log.Println("→ Sending Socket.IO CONNECT (40)")
			if err := conn.Write(ctx, websocket.MessageText, []byte("40")); err != nil {
				log.Printf("write error: %v", err)
				return
			}

		case "2": // EIO ping
			fmt.Println("EIO PING received → sending PONG")
			if err := conn.Write(ctx, websocket.MessageText, []byte("3")); err != nil {
				log.Printf("pong error: %v", err)
				return
			}

		case "4": // EIO message — contains Socket.IO packet
			if len(payload) == 0 {
				continue
			}
			sioType := string(payload[0])
			sioPayload := payload[1:]

			switch sioType {
			case "0": // SIO CONNECT
				fmt.Printf("SIO CONNECT: %s\n", sioPayload)

				// Now send login event after SIO connect
				// ACK ID "0" → server will reply with "430[result]"
				if !sentLogin {
					sentLogin = true
					loginPayload := fmt.Sprintf(
						`420["login",{"username":%q,"password":%q,"token":""}]`,
						user, pass,
					)
					log.Printf("→ Sending login event: %s", loginPayload)
					time.Sleep(200 * time.Millisecond)
					if err := conn.Write(ctx, websocket.MessageText, []byte(loginPayload)); err != nil {
						log.Printf("login write error: %v", err)
						return
					}
				}

			case "2": // SIO EVENT
				// Try to parse event name
				var parts []json.RawMessage
				if err := json.Unmarshal([]byte(sioPayload), &parts); err == nil && len(parts) > 0 {
					var eventName string
					if err := json.Unmarshal(parts[0], &eventName); err == nil {
						fmt.Printf("SIO EVENT name=%q args=%d\n", eventName, len(parts)-1)
						if len(parts) > 1 {
							// Pretty print first arg (truncated)
							argStr := string(parts[1])
							if len(argStr) > 300 {
								argStr = argStr[:300] + "... [truncated]"
							}
							fmt.Printf("  arg[0]: %s\n", argStr)
						}
					}
				} else {
					fmt.Printf("SIO EVENT (raw): %s\n", sioPayload)
				}

			case "3": // SIO ACK (callback response)
				fmt.Printf("SIO ACK: %s\n", sioPayload)

			case "4": // SIO CONNECT_ERROR
				fmt.Printf("SIO CONNECT_ERROR: %s\n", sioPayload)

			default:
				fmt.Printf("SIO type=%s payload=%s\n", sioType, sioPayload)
			}

		default:
			fmt.Printf("EIO type=%s payload=%s\n", eioType, payload)
		}

		// Stop after 80 frames or on loginByToken ACK
		if frameCount >= 80 {
			log.Println("\n→ Reached 80 frames, stopping probe.")
			break
		}
	}

	fmt.Printf("\n═══ Probe complete. Total frames: %d ═══\n", frameCount)
}
