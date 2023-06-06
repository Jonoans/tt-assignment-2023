package main

import (
	"fmt"
	"log"
	"net"
	"os"

	rpc "github.com/TikTokTechImmersion/assignment_demo_2023/rpc-server/kitex_gen/rpc/imservice"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	etcd "github.com/kitex-contrib/registry-etcd"
)

func main() {
	// Initialise connection to database
	InitDatabase()
	defer CloseDatabase()
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	r, err := etcd.NewEtcdRegistry([]string{"etcd:2379"}) // r should not be reused.
	if err != nil {
		log.Fatal(err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		panic("error acquiring hostname information")
	}

	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:8888", hostname))
	if err != nil {
		panic("error resolving service address")
	} else if addr.IP.IsLoopback() {
		panic("service address is loopback address")
	} else if addr.IP.IsUnspecified() {
		panic("service address is unspecified")
	}

	svr := rpc.NewServer(new(IMServiceImpl), server.WithRegistry(r), server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{
		ServiceName: "demo.rpc.server",
	}), server.WithServiceAddr(addr))

	err = svr.Run()
	if err != nil {
		log.Println(err.Error())
	}
}
