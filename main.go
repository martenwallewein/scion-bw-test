// Downloads torrents from the command-line.
package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/anacrolix/tagflag"
	log "github.com/inconshreveable/log15"
	"github.com/netsec-ethz/scion-apps/pkg/appnet"
	"github.com/scionproto/scion/go/lib/snet"
	// "github.com/netsec-ethz/scion-apps/pkg/appnet"
)

var flags = struct {
	IsServer        bool
	NumCons         int
	StartPort       int
	PacketSize      int64
	NumPackets      int64
	TargetSCIONAddr string // 19-ffaa:1:c3f,[10.0.0.2]
	tagflag.StartPos
}{
	IsServer:   true,
	NumCons:    1,
	StartPort:  42522,
	PacketSize: 1208,
	NumPackets: 1200000,
}

func LogFatal(msg string, a ...interface{}) {
	log.Crit(msg, a...)
	os.Exit(1)
}

func Check(e error) {
	if e != nil {
		LogFatal("Fatal error. Exiting.", "err", e)
	}
}

func main() {
	if err := mainErr(); err != nil {
		log.Info("error in main: %v", err)
		os.Exit(1)
	}
}

func mainErr() error {
	tagflag.Parse(&flags)

	startPort := uint16(flags.StartPort)
	var i uint16
	i = 0
	var wg sync.WaitGroup
	for i < uint16(flags.NumCons) {
		go func(wg *sync.WaitGroup, startPort uint16, i uint16) {
			defer wg.Done()
			if flags.IsServer {
				err := runServer(startPort + i)
				fmt.Println(err)
			} else {
				addrStr := fmt.Sprintf("%s:%d", flags.TargetSCIONAddr, startPort+i)
				serverAddr, err := appnet.ResolveUDPAddr(addrStr)
				Check(err)
				runClient(serverAddr)
			}
		}(&wg, startPort, i)
		i++
	}
	wg.Wait()
	time.Sleep(time.Minute * 5)

	return nil
}

func runClient(serverAddr *snet.UDPAddr) {
	DCConn, err := appnet.DialAddr(serverAddr)
	Check(err)
	// clientCCAddr := CCConn.LocalAddr().(*net.UDPAddr)
	sb := make([]byte, flags.PacketSize)
	// Data channel connection
	// DCConn, err := appnet.DefNetwork().Dial(
	//	context.TODO(), "udp", clientCCAddr, serverAddr, addr.SvcNone)
	// Check(err)
	var i int64 = 0
	start := time.Now()
	for i < flags.NumPackets {
		// Compute how long to wait

		// PrgFill(bwp.PrgKey, int(i*bwp.PacketSize), sb)
		// Place packet number at the beginning of the packet, overwriting some PRG data
		_, err := DCConn.Write(sb)
		Check(err)
		i++
	}
	elapsed := time.Since(start)
	fmt.Printf("Send took %s\n", elapsed)
}

func runServer(port uint16) error {

	conn, err := appnet.ListenPort(port)
	fmt.Println("CONN")
	fmt.Println(conn.LocalAddr())
	if err != nil {
		return err
	}
	var numPacketsReceived int64
	var lastNumPacketsReceived int64
	numPacketsReceived = 0
	lastNumPacketsReceived = 0
	recBuf := make([]byte, flags.PacketSize+1000)
	timer := time.NewTimer(5 * time.Second)
	go func() {
		<-timer.C
		fmt.Printf("Received %d packets\n", numPacketsReceived)
		bw := ((numPacketsReceived - lastNumPacketsReceived) * flags.PacketSize * 8) / 1024 / 1024 / 10
		fmt.Printf("reached bandwidth %dMbit/s\n", bw)
	}()
	for numPacketsReceived < flags.NumPackets {
		n, err := conn.Read(recBuf)

		// Ignore errors, todo: detect type of error and quit if it was because of a SetReadDeadline
		if err != nil {
			fmt.Println(err)
			continue
		}
		if int64(n) != flags.PacketSize {
			// The packet has incorrect size, do not count as a correct packet
			fmt.Println("Incorrect size.", n, "bytes instead of", flags.PacketSize)
			continue
		}
		// fmt.Printf("Read packet of size %d\n", n)
		numPacketsReceived++
		// fmt.Printf("Received %d packets\n", numPacketsReceived)
	}

	fmt.Printf("Received all packets")
	return nil
}
