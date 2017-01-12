package main

import (
	"github.com/docker/docker/client"
	"context"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types"
	"log"
	"strings"
)

type Docker struct {
	cli *client.Client
}

func (d *Docker) Connect() error {
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	d.cli = cli;

	return nil;
}

func (d *Docker) InitSwarm(ctx context.Context, addr string) (*swarm.Swarm, error) {
	_, err := d.cli.SwarmInit(ctx, swarm.InitRequest{
		AdvertiseAddr: addr,
		ListenAddr: addr,
	})

	if err != nil {
		return nil, err;
	}

	swarmData, err := d.cli.SwarmInspect(ctx)
	if err != nil {
		return nil, err;
	}

	return &swarmData, nil;
}

func (d *Docker) IsSwarmActive(ctx context.Context) (bool, error) {
	_, err := d.cli.SwarmInspect(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "connect this node to swarm") {
			return  false, nil;
		}
		return false, err;
	}

	log.Println("exists?")
	return true, nil;
}

func (d * Docker) JoinSwarmAsWorker(ctx context.Context, managers []string, token, addr string) error {
	log.Println("Join to ", managers)
	return d.cli.SwarmJoin(ctx, swarm.JoinRequest{
		JoinToken: token,
		RemoteAddrs: managers,
		ListenAddr: addr,
		AdvertiseAddr: addr,
	})
}

func (d *Docker) PromoteToManager(ctx context.Context, n swarm.Node) error {
	log.Printf("Promotig node %s to Manager", n.ID)

	if err := d.cli.NodeUpdate(ctx, n.ID, n.Version, swarm.NodeSpec{
		Role: swarm.NodeRoleManager,
		Availability: n.Spec.Availability,
	});err != nil {
		if strings.Contains(err.Error(), "update out of sequence") {
			return nil;
		}
		return err;
	}
	return nil;
}

func (d *Docker) GetNodes(ctx context.Context) ([]swarm.Node, error) {
	return d.cli.NodeList(ctx, types.NodeListOptions{})
}
