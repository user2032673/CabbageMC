package main

import (
	"fmt"
	"net"

	"github.com/go-mclib/protocol/java_protocol/net_structures"
)

func main() {
	listener, err := net.Listen("tcp", "0.0.0.0:25565")
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	fmt.Println("Minecraft 26.3 Server listening on port 25565...")
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Connection accept error: %v\n", err)
			continue
		}
		go clientHandler(conn)
	}
}

func clientHandler(conn net.Conn) {
	defer conn.Close()
	connReader := net_structures.NewReaderFrom(conn)
	connWriter := net_structures.NewWriterTo(conn)

	connectionState := ProtocolState.Handshaking

	// Packet reading loop
	for {
		packetData, err := readPacket(connReader)
		if err != nil {
			fmt.Printf("readPacket, %v\n", err)
			return
		}
		packetID, err := getPacketID(packetData)
		if err != nil {
			fmt.Println("getPacketID")
			return
		}
		switch connectionState {

		case ProtocolState.Handshaking:
			if packetID == 0x00 { // C2SHandshake
				packet, err := readC2SHandshake(packetData)
				if err != nil {
					fmt.Println("decodeC2SHandshake")
					return
				}
				if packet.ProtocolVersion != 777 {
					fmt.Println("proto mismatch")
					return
				}
				fmt.Printf("[+] %s:%d intent %d\n", packet.ServerAddress, packet.ServerPort, packet.Intent)
				if packet.Intent == 1 {
					connectionState = ProtocolState.Status
					continue
				}
				if packet.Intent == 2 || packet.Intent == 3 {
					connectionState = ProtocolState.Login
					continue
				}
			}
		case ProtocolState.Status:
			if packetID == 0 { // C2SStatusRequest
				_, err := readStatusRequest(packetData) // It doesn't actually have any fields lmao
				if err != nil {
					fmt.Println("decodeStatusRequest?")
					return
				}
				// Well then send the status response
				// (yes this is single routined, once we're in PLAY it's gonna be better)
				packet := S2CStatusResponse{
					JSONResponse: `{"version":{"name":"26.3","protocol":777},"players":{"max":20,"online":1,"sample":[{"name":"thinkofdeath","id":"4566e69f-c907-48ee-8d71-d7ba5aa00d20"}]},"description":{"text":"Hello, world!"},"favicon":null,"enforcesSecureChat":false}`,
				}
				writeStatusResponse(connWriter, &packet)
			}
			if packetID == 1 { // C2SPingRequest
				ping, err := readPingRequest(packetData)
				if err != nil {
					fmt.Println("readPingRequest")
					return
				}
				packet := S2CPongResponse{
					Timestamp: ping.Timestamp,
				}
				err = writePongResponse(connWriter, &packet)
				if err != nil {
					fmt.Println("writePongResponse")
					return
				}

			}
		case ProtocolState.Login:
			if packetID == 0x00 { // C2SLoginStart
				_, err := readLoginStart(packetData)
				if err != nil {
					fmt.Println("readLoginStart\n")
					return
				}
				
			}
		}

	}
}
