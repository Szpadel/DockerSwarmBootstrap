package main

import (
	"flag"
	"log"
	"context"
)

/* 1. create structures
   2. register as node ttl 60s, refresh 45s
   3. when number of nodes > managers required select init node
   4. init node initialize swarm and provide worker token
   5. rest of the nodes joins as workers
   6. init node promote rest of the nodes as managers
 */

func main() {
	size := flag.Int("size", 0, "Number of manager nodes to create")
	ip := flag.String("advert-ip", "", "Node ip address with port")
	flag.Parse()

	if (*size < 1 || *ip == "") {
		log.Panic("You have to provide ip and size", *size, *ip);
	}

	etcd := Etcd{}
	if err := etcd.Connect([]string{"http://127.0.0.1:2379"}); err != nil {
		log.Panic(err)
	}
	log.Println("etcd connected")

	docker := Docker{}
	if err := docker.Connect(); err != nil {
		log.Panic(err)
	}
	log.Println("docker connected")

	bootstrap := NewBootstrap(&etcd, &docker)
	ctx := context.Background()

	ok, err := bootstrap.IsSwarmReady(ctx)
	if err != nil {
		log.Panic(err)
	}

	if ok {
		log.Println("Swarm already working, exiting")
		return;
	}

	if _, err := bootstrap.TryInitSwarm(ctx, *size); err != nil {
		log.Panic(err)
	}
	log.Println("init completed")
	if err := bootstrap.JoinSwarm(ctx, *ip, *size); err != nil {
		log.Panic(err)
	}
	log.Println("done")

}
