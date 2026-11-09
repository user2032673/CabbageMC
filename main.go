package main

import (
	"encoding/json"
	"fmt"
	"net"
	"sort"

	"github.com/go-mclib/protocol/java_protocol/net_structures"
)

func main() {
	registries, err := loadRegistries("registries.json")
	tags, err := loadTags("tags.json")
	if err != nil {
		panic(err)
	}
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
		go clientHandler(conn, registries, tags)
	}
}

func clientHandler(conn net.Conn, registries map[string]map[string]json.RawMessage, tags map[string]map[string]json.RawMessage) {
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
					JSONResponse: `{"version":{"name":"26.3","protocol":777},"players":{"max":69,"online":420,"sample":[{"name":"evil-user2032673","id":"4566e69f-c907-48ee-8d71-d7ba5aa00d20"}, {"name":"SIX-SEVENNNNN","id":"4566ed9f-c907-48ee-8d71-d7ba5aa00d20"}, {"name":"UrIPIs` + conn.RemoteAddr().String() + `","id":"3566e69f-c907-48ee-8d71-d7ba5aa00d20"}, {"name":"TimothyPickering","id":"1566e69f-c907-48ee-8d71-d7ba5aa00d20"}]},"description":{"text":"The end times are near!"},"favicon":null,"enforcesSecureChat":false}`,
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
				err = writeKnownPacks(connWriter, &S2CKnownPacks{
					KnownPacks: net_structures.PrefixedArray[net_structures.String]{
						net_structures.String("minecraft"),
						net_structures.String("core"),
						net_structures.String("26.3"),
					},
				})
				if err != nil {
					fmt.Println("knownPacks\n")
					return
				}
			}

		case ProtocolState.Config:
			if packetID == 0x07 {
				// Ignore the client sent packet, this is the Known Packs response and probably has core.

				registryIDs := make([]string, 0, len(registries))

				for id := range registries {
					registryIDs = append(registryIDs, id)
				}

				sort.Strings(registryIDs)

				for _, id := range registryIDs {
					if err := writeRegistryData(connWriter, id, registries[id]); err != nil {
						fmt.Println(err)
						return
					}
				}
				// Tags time
				tagIDs := make([]string, 0, len(tags))

				for id := range tags {
					tagIDs = append(tagIDs, id)
				}

				sort.Strings(tagIDs)

				for _, id := range tagIDs {
					if err := writeTagData(connWriter, id, tags[id]); err != nil {
						fmt.Println(err)
						return
					}
				}
				writeFinishConfiguration(connWriter, &S2CFinishConfiguration{})

			}
			if packetID == 0x03 {
				connectionState = ProtocolState.Play
				// Ignore the client sent packet there's nothing in it
				err := writeLogin(connWriter, &S2CLogin{
					EntityID:            1,
					IsHardcore:          true,
					DimensionNames:      net_structures.PrefixedArray[net_structures.String]{"minecraft:overworld"},
					MaxPlayers:          0,
					ViewDistance:        10,
					SimulationDistance:  10,
					ReducedDebugInfo:    false,
					EnableRespawnScreen: true,
					DoLimitedCrafting:   false,
					DimensionID:         0,
					DimensionName:       "minecraft:overworld",
					HashedSeed:          net_structures.VarLong(0),
					GameMode:            0,
					PreviousGameMode:    0,
					IsDebug:             false,
					IsFlat:              true,
					HasDeathLocation:    false,
					DeathDimensionName:  "cabbage:cabbage_fields",
					DeathLocation:       net_structures.Position{X: 69, Y: 69, Z: 69},
					PortalCooldown:      0,
					SeaLevel:            0,
					OnlineMode:          false,
					EnforcesSecureChat:  false,
				}) // MEGA CHUNGUSSS
				if err != nil {
					fmt.Println("writeLogin")
					return
				}

				err = writeSyncPlayerPosition(connWriter, &S2CSyncPlayerPosition{
					TeleportID: 0,
					X:          0,
					Y:          0,
					Z:          0,
					VelocityX:  0,
					VelocityY:  0,
					VelocityZ:  0,
					Yaw:        0,
					Pitch:      0,
					Flags:      0,
				})
				if err != nil {
					fmt.Println("writeSyncPlayerPos")
					return
				}

			}
		case ProtocolState.Play:
			if packetID == 0x00 { // C2SConfirmTeleportation
				// Discard for now

			}

		}
	}
}
