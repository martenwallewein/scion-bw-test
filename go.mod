module github.com/martenwallewein/scion-bw-test

go 1.15

require (
	github.com/anacrolix/tagflag v1.2.0
	github.com/gammazero/workerpool v1.1.1 // indirect
	github.com/inconshreveable/log15 v0.0.0-20201112154412-8562bdadbbac
	github.com/jandos/gofine v0.0.1
	github.com/lucas-clemente/quic-go v0.19.2
	github.com/netsec-ethz/scion-apps v0.3.0
	github.com/scionproto/scion v0.6.0
)

replace github.com/scionproto/scion => /home/marten/go/src/github.com/martenwallewein/scion
replace github.com/netsec-ethz/scion-apps => /home/marten/go/src/github.com/martenwallewein/scion-apps
