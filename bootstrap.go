package main

import (
	"context"
	"log"
)

type Bootstrap struct {
	etcd   *Etcd
	docker *Docker
}

func NewBootstrap(etcd *Etcd, docker *Docker) Bootstrap {
	b := Bootstrap{
		etcd: etcd,
		docker: docker,
	}

	return b;
}

func (b* Bootstrap) IsSwarmReady(ctx context.Context) (bool, error) {
	return b.docker.IsSwarmActive(ctx)
}

func (b *Bootstrap) TryInitSwarm(ctx context.Context, size int) (bool, error) {
	ok, err := b.etcd.TryLockInit(ctx);

	if err != nil {
		return false, err;
	}

	if ok {
		if err = b.etcd.InitStructures(ctx, size); err != nil {
			return false, err;
		}

		return true, nil;
	}

	return false, nil;
}

func (b *Bootstrap) initializeSwarm(ctx context.Context, addr string, size int) error {
	log.Println("Initializing swarm")
	swarmData, err := b.docker.InitSwarm(ctx, addr)
	if err != nil {
		return err
	}

	err = b.etcd.SetWorkerToken(ctx, swarmData.JoinTokens.Worker)
	if err != nil {
		return err;
	}

	mgrCtx, cancel := context.WithCancel(ctx)
	defer cancel();
	b.etcd.AddAsManager(mgrCtx, addr)

	managersCount := 0;

	for managersCount < size {
		managersCount = 0;
		nodeList, err := b.docker.GetNodes(ctx)
		if err != nil {
			return err;
		}
		for _, node := range nodeList {
			if node.ManagerStatus != nil {
				managersCount++;
			}
		}

		if managersCount < size {
			for _, node := range nodeList {
				if node.ManagerStatus == nil {
					err = b.docker.PromoteToManager(ctx, node)
					if err != nil {
						return err;
					}
				}
			}
		}
	}

	return nil;
}

func (b *Bootstrap) joinAsWorker(ctx context.Context, addr string) error {

	token, err := b.etcd.GetWorkerToken(ctx)
	log.Println("Joining as Worker ", token)
	if err != nil {
		return err;
	}

	managers, err := b.etcd.GetManagers(ctx)
	if err != nil {
		return err;
	}

	b.docker.JoinSwarmAsWorker(ctx, managers, token, addr)
	return nil;
}

func (b *Bootstrap) JoinSwarm(ctx context.Context, addr string, size int) error {
	nodeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	b.etcd.AddAsNode(nodeCtx, addr)

	has, err := b.etcd.HasInitNode(ctx)
	if err != nil {
		return err;
	}

	if has {
		return b.joinAsWorker(ctx, addr)
	}

	log.Println("Waiting for other nodes...")
	b.etcd.WaitForEnoughNodes(ctx)
	ok, err := b.etcd.RaceForInitNode(ctx, addr)


	if err != nil {
		return err;
	}

	if ok {
		return b.initializeSwarm(ctx, addr, size)
	} else {
		return b.joinAsWorker(ctx, addr)
	}
}
