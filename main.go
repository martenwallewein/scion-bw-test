// Downloads torrents from the command-line.
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/anacrolix/tagflag"
	log "github.com/inconshreveable/log15"
	"github.com/lucas-clemente/quic-go"
	"github.com/martenwallewein/scion-bw-test/scion_torrent"
	"github.com/netsec-ethz/scion-apps/pkg/appnet"
	"github.com/netsec-ethz/scion-apps/pkg/appnet/appquic"
	"github.com/scionproto/scion/go/lib/snet"
	// "github.com/netsec-ethz/scion-apps/pkg/appnet"
)

var flags = struct {
	IsServer        bool
	NumCons         int
	StartPort       int
	PacketSize      int64
	NumPackets      int64
	NumThreads      int
	UseHSD          bool
	UseUDP          bool
	UseQUIC         bool
	TargetSCIONAddr string
	TargetUdpAddr   string //
	tagflag.StartPos
}{
	IsServer:        true,
	NumCons:         1,
	StartPort:       42522,
	PacketSize:      8700,
	NumPackets:      1200000,
	TargetSCIONAddr: "19-ffaa:1:c3f,[10.0.0.2]",
	UseHSD:          false,
	NumThreads:      1,
	UseQUIC:         false,
	UseUDP:          false,
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
	// runtime.GOMAXPROCS(1)
	startPort := uint16(flags.StartPort)
	var i uint16
	i = 0
	scion_torrent.InitSQUICCerts()
	var wg sync.WaitGroup
	for i < uint16(flags.NumCons) {
		go func(wg *sync.WaitGroup, startPort uint16, i uint16) {
			defer wg.Done()
			if flags.IsServer {
				if flags.UseQUIC {
					err := runServerQUIC(startPort+i, flags.UseHSD)
					fmt.Println(err)
				} else if flags.UseUDP {
					err := runServerUDP(startPort+i, flags.UseHSD)
					fmt.Println(err)
				} else {
					err := runServer(startPort+i, flags.UseHSD)
					fmt.Println(err)
				}

			} else {
				addrStr := fmt.Sprintf("%s:%d", flags.TargetSCIONAddr, startPort+i)
				serverAddr, err := appnet.ResolveUDPAddr(addrStr)
				Check(err)

				if flags.UseQUIC {
					runClientQUIC(serverAddr, flags.UseHSD)
				} else if flags.UseUDP {
					runClientUDP(flags.TargetUdpAddr, flags.UseHSD, flags.NumThreads)
				} else {
					runClient(serverAddr, flags.UseHSD, flags.NumThreads)
				}
			}
		}(&wg, startPort, i)
		i++
	}
	wg.Wait()
	time.Sleep(time.Minute * 5)

	return nil
}

func runClientUDP(serverAddr string, hsd bool, numThreads int) {
	var DCConn *net.UDPConn
	var err error
	raddr, err := net.ResolveUDPAddr("udp", serverAddr)
	Check(err)

	// Although we're not in a connection-oriented transport,
	// the act of `dialing` is analogous to the act of performing
	// a `connect(2)` syscall for a socket of type SOCK_DGRAM:
	// - it forces the underlying socket to only read and write
	//   to and from a specific remote address.
	DCConn, err = net.DialUDP("udp", nil, raddr)

	Check(err)
	// clientCCAddr := CCConn.LocalAddr().(*net.UDPAddr)
	sb := make([]byte, flags.PacketSize)
	// Data channel connection
	// DCConn, err := appnet.DefNetwork().Dial(
	//	context.TODO(), "udp", clientCCAddr, serverAddr, addr.SvcNone)
	// Check(err)
	var i int64 = 0
	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(numThreads)
	for j := 0; j < numThreads; j++ {
		go func() {
			for i < flags.NumPackets {
				// Compute how long to wait

				// PrgFill(bwp.PrgKey, int(i*bwp.PacketSize), sb)
				// Place packet number at the beginning of the packet, overwriting some PRG data
				_, err := DCConn.Write(sb)
				Check(err)
				i++
			}
			wg.Done()
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)
	fmt.Printf("Send took %s\n", elapsed)
}

func runClient(serverAddr *snet.UDPAddr, hsd bool, numThreads int) {
	var DCConn *snet.Conn
	var err error

	if hsd {
		DCConn, err = appnet.DialAddrUDP(serverAddr)
	} else {
		DCConn, err = appnet.DialAddr(serverAddr)
	}

	Check(err)
	// clientCCAddr := CCConn.LocalAddr().(*net.UDPAddr)
	sb := make([]byte, flags.PacketSize)
	// Data channel connection
	// DCConn, err := appnet.DefNetwork().Dial(
	//	context.TODO(), "udp", clientCCAddr, serverAddr, addr.SvcNone)
	// Check(err)
	var i int64 = 0
	start := time.Now()
	fmt.Println("Spawned new goroutine")
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

func runClientQUIC(serverAddr *snet.UDPAddr, hsd bool) {
	var DCConn quic.Stream
	var err error

	if hsd {
		sess, serr := appquic.DialAddrUDP(serverAddr, "127.0.0.1:42425", scion_torrent.TLSCfg, &quic.Config{
			KeepAlive: true,
		})
		Check(serr)

		DCConn, err = sess.OpenStreamSync(context.Background())
		Check(err)
	} else {
		sess, serr := appquic.DialAddr(serverAddr, "127.0.0.1:42425", scion_torrent.TLSCfg, &quic.Config{
			KeepAlive: true,
		})
		Check(serr)

		DCConn, err = sess.OpenStreamSync(context.Background())
		Check(err)
	}

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

func runServerUDP(port uint16, hsd bool) error {

	var conn *net.UDPConn
	var err error

	laddr, err2 := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
	Check(err2)
	conn, err = net.ListenUDP("udp", laddr)
	Check(err)
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
		bw := ((numPacketsReceived - lastNumPacketsReceived) * flags.PacketSize * 8) / 1024 / 1024 / 5
		fmt.Printf("reached bandwidth %dMbit/s\n", bw)
	}()
	for numPacketsReceived < flags.NumPackets {
		_, err := conn.Read(recBuf)

		// Ignore errors, todo: detect type of error and quit if it was because of a SetReadDeadline
		if err != nil {
			fmt.Println(err)
			continue
		}
		/*if int64(n) != flags.PacketSize {
			// The packet has incorrect size, do not count as a correct packet
			fmt.Println("Incorrect size.", n, "bytes instead of", flags.PacketSize)
			continue
		}*/
		// fmt.Printf("Read packet of size %d\n", n)
		numPacketsReceived++
		// fmt.Printf("Received %d packets\n", numPacketsReceived)
	}

	fmt.Printf("Received all packets")
	return nil
}

func runServer(port uint16, hsd bool) error {

	var conn *snet.Conn
	var err error

	if hsd {
		conn, err = appnet.ListenPortUDP(port)
	} else {
		conn, err = appnet.ListenPort(port)
	}

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
		bw := ((numPacketsReceived - lastNumPacketsReceived) * flags.PacketSize * 8) / 1024 / 1024 / 5
		fmt.Printf("reached bandwidth %dMbit/s\n", bw)
	}()
	for numPacketsReceived < flags.NumPackets {
		_, err := conn.Read(recBuf)

		// Ignore errors, todo: detect type of error and quit if it was because of a SetReadDeadline
		if err != nil {
			fmt.Println(err)
			continue
		}
		/*if int64(n) != flags.PacketSize {
			// The packet has incorrect size, do not count as a correct packet
			fmt.Println("Incorrect size.", n, "bytes instead of", flags.PacketSize)
			continue
		}*/
		// fmt.Printf("Read packet of size %d\n", n)
		numPacketsReceived++
		// fmt.Printf("Received %d packets\n", numPacketsReceived)
	}

	fmt.Printf("Received all packets")
	return nil
}

func runServerQUIC(port uint16, hsd bool) error {

	var qConn quic.Listener
	var listenErr error

	if hsd {
		conn, err := appnet.ListenPortUDP(port) // TODO: Packetconn
		Check(err)
		qConn, listenErr = quic.Listen(conn, scion_torrent.TLSCfg, &quic.Config{KeepAlive: true})
		Check(listenErr)
	} else {
		conn, err := appnet.ListenPort(port)
		Check(err)

		qConn, listenErr = quic.Listen(conn, scion_torrent.TLSCfg, &quic.Config{KeepAlive: true})
		Check(listenErr)
	}

	fmt.Println("CONN")
	fmt.Println(qConn.Addr())

	x, err := qConn.Accept(context.Background())
	Check(err)
	DCConn, err := x.AcceptStream(context.Background())
	Check(err)
	var numPacketsReceived int64
	var lastNumPacketsReceived int64
	numPacketsReceived = 0
	lastNumPacketsReceived = 0
	recBuf := make([]byte, flags.PacketSize+1000)
	timer := time.NewTimer(5 * time.Second)
	go func() {
		<-timer.C
		fmt.Printf("Received %d packets\n", numPacketsReceived)
		bw := ((numPacketsReceived - lastNumPacketsReceived) * flags.PacketSize * 8) / 1024 / 1024 / 5
		fmt.Printf("reached bandwidth %dMbit/s\n", bw)
	}()
	for numPacketsReceived < flags.NumPackets {
		_, err := DCConn.Read(recBuf)

		// Ignore errors, todo: detect type of error and quit if it was because of a SetReadDeadline
		if err != nil {
			fmt.Println(err)
			continue
		}
		/*if int64(n) != flags.PacketSize {
			// The packet has incorrect size, do not count as a correct packet
			fmt.Println("Incorrect size.", n, "bytes instead of", flags.PacketSize)
			continue
		}*/
		// fmt.Printf("Read packet of size %d\n", n)
		numPacketsReceived++
		// fmt.Printf("Received %d packets\n", numPacketsReceived)
	}

	fmt.Printf("Received all packets")
	return nil
}
