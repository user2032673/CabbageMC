package main

import (
	"fmt"
	"net"

	"github.com/go-mclib/protocol/java_protocol/net_structures"
)

func main() {
	listener, err := net.Listen("tcp", "0.0.0.0:25000")
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	fmt.Println("Minecraft 26.3 Server listening on port 25000...")
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
					JSONResponse: `{"version":{"name":"26.3","protocol":777},"players":{"max":69,"online":420,"sample":[{"name":"evil-user2032673","id":"4566e69f-c907-48ee-8d71-d7ba5aa00d20"}, {"name":"SIX-SEVENNNNN","id":"4566ed9f-c907-48ee-8d71-d7ba5aa00d20"}, {"name":"CultOfFountain","id":"3566e69f-c907-48ee-8d71-d7ba5aa00d20"}, {"name":"TimothyPickering","id":"1566e69f-c907-48ee-8d71-d7ba5aa00d20"}]},"description":{"text":"The end times are near!"},"favicon":null,"enforcesSecureChat":false}`,
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
				loginPacket, err := readLoginStart(packetData)
				if err != nil {
					fmt.Println("readLoginStart\n")
					return
				}
				writeLoginSuccess(connWriter, &S2CLoginSuccess{
					Profile: Profile{
						Uuid:     loginPacket.PlayerUUID,
						Username: loginPacket.Name,
					},
					SessionUUID: UUID([]byte(`0000000000000000`)),
				})
			}
			if packetID == 0x03 { // C2SLoginAcknowledged
				_, err := readLoginAcknowledged(packetData)
				if err != nil {
					fmt.Println("readLoginAcknowledged\n")
					return
				}
				connectionState = ProtocolState.Config

				writeFinishConfiguration(connWriter, &S2CFinishConfiguration{})
			}
		}

	}
}
