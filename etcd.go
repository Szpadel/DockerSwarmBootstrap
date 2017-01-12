package main

import (
	"github.com/coreos/etcd/client"
	"time"
	"context"
	"strconv"
	"log"
)

const rootDir = "/docker-swarm";

const managersDir = rootDir + "/managers";
const nodesDir = rootDir + "/nodes";

const workerTokenName = rootDir + "/workerToken"
const sizeName = rootDir + "/size"
const initNodeName = rootDir + "/initNode"

type Etcd struct {
	cfg    client.Config
	client client.Client
	kApi   client.KeysAPI
}

func (e *Etcd) Connect(endpoints []string) error {
	e.cfg = client.Config{
		Endpoints:               endpoints,
		Transport:               client.DefaultTransport,
		HeaderTimeoutPerRequest: 5 * time.Second,
	}
	c, err := client.New(e.cfg)

	if err != nil {
		return err;
	}

	e.client = c;
	e.kApi = client.NewKeysAPI(c);

	return nil;
}

func (e *Etcd) TryLockInit(ctx context.Context) (bool, error) {
	_, err := e.kApi.Set(ctx, rootDir, "", &client.SetOptions{
		PrevExist: client.PrevNoExist,
		Dir: true,
	});

	if err != nil {
		if err.(client.Error).Code == client.ErrorCodeNodeExist {
			return false, nil
		}

		return false, err;
	}

	return true, nil
}

func (e *Etcd) AddAsManager(ctx context.Context, addr string) {
	go e.addAndKeepNode(ctx, managersDir, addr);
}

func (e *Etcd) AddAsNode(ctx context.Context, addr string) {
	go e.addAndKeepNode(ctx, nodesDir, addr);
}

func (e *Etcd) addAndKeepNode(ctx context.Context, dir, addr string) error {
	var key = dir + "/" + addr
	if _, err := e.kApi.Set(ctx, key, addr, &client.SetOptions{
		TTL: 60 * time.Second,
	}); err != nil {
		return err;
	}

	for true {
		select {
		// refresh
		case <-time.Tick(45 * time.Second):
			if _, err := e.kApi.Set(ctx, key, addr, &client.SetOptions{
				TTL: 60 * time.Second,
			}); err != nil {
				return err;
			}
		case <-ctx.Done():
			e.kApi.Delete(context.Background(), key, &client.DeleteOptions{});
			return nil;
		}
	}
	return nil;
}

func (e *Etcd) SetWorkerToken(ctx context.Context, token string) error {
	_, err := e.kApi.Set(ctx, workerTokenName, token, &client.SetOptions{
		PrevExist: client.PrevNoExist,
		Dir: false,
	})

	return err;
}

func (e *Etcd) GetWorkerToken(ctx context.Context) (string, error) {
	resp, err := e.kApi.Get(ctx, workerTokenName, &client.GetOptions{})
	if err != nil {
		if err.(client.Error).Code == client.ErrorCodeKeyNotFound {
			// race condition, try again
			return e.GetWorkerToken(ctx)
		}
		return "", err;
	}
	return resp.Node.Value, nil;
}

func (e *Etcd) GetManagers(ctx context.Context) ([]string, error) {
	resp, err := e.kApi.Get(ctx, managersDir, &client.GetOptions{
		Recursive: true,
	});
	if err != nil {
		if err.(client.Error).Code == client.ErrorCodeKeyNotFound {
			// race condition, try again
			return e.GetManagers(ctx)
		}
		return nil, err;
	}

	managers := make([]string, 0);
	for _, n := range resp.Node.Nodes {
		managers = append(managers, n.Value)
	}

	if len(managers) == 0 {
		// managers not available yet
		log.Println("Managers not yet available, waiting...")
		time.Sleep(5 * time.Second);
		return e.GetManagers(ctx)
	}

	return managers, nil;
}

func (e *Etcd) WaitForEnoughNodes(ctx context.Context) error {
	waitCtx, cancel := context.WithCancel(ctx);
	defer cancel();

	resp, err := e.kApi.Get(waitCtx, sizeName, &client.GetOptions{});
	if err != nil {
		return err;
	}

	size, err := strconv.ParseInt(resp.Node.Value, 10, 64);
	if err != nil {
		return err;
	}

	watcher := e.kApi.Watcher(nodesDir, &client.WatcherOptions{
		Recursive: true,
	})

	for true {
		_, err := watcher.Next(waitCtx)
		if err != nil {
			return err;
		}
		nodesList, err := e.kApi.Get(waitCtx, nodesDir, &client.GetOptions{})

		if err != nil {
			return err;
		}

		if nodesList.Node.Nodes.Len() >= int(size) {
			log.Println("All nodes available")
			return nil
		}
		log.Printf("Waiting for %d other nodes, now( %d )", int(size) - nodesList.Node.Nodes.Len(), nodesList.Node.Nodes.Len())
	}

	return nil;
}

func (e *Etcd) RaceForInitNode(ctx context.Context, addr string) (bool, error) {
	if _, err := e.kApi.Set(ctx, initNodeName, addr, &client.SetOptions{
		PrevExist: client.PrevNoExist,
	}); err != nil {
		if(err.(client.Error).Code != client.ErrorCodeNodeExist) {
			return false, err;
		}
		// we lost
		return false, nil;
	}
	// we won
	return true, nil;
}

func (e *Etcd) InitStructures(ctx context.Context, size int) error {

	if _, err := e.kApi.Set(ctx, managersDir, "", &client.SetOptions{
		PrevExist: client.PrevNoExist,
		Dir: true,
	}); err != nil {
		e.RevertInit(ctx)
		return err;
	}

	if _, err := e.kApi.Set(ctx, nodesDir, "", &client.SetOptions{
		PrevExist: client.PrevNoExist,
		Dir: true,
	}); err != nil {
		e.RevertInit(ctx)
		return err;
	}

	if _, err := e.kApi.Set(ctx, sizeName, strconv.FormatInt(int64(size), 10), &client.SetOptions{
		PrevExist: client.PrevNoExist,
		Dir: false,
	}); err != nil {
		e.RevertInit(ctx)
		return err;
	}

	return nil;
}

func (e *Etcd) HasInitNode(ctx context.Context) (bool, error) {
	if _, err := e.kApi.Get(ctx, initNodeName, &client.GetOptions{
		Quorum: true,
	}); err != nil {
		if err.(client.Error).Code == client.ErrorCodeKeyNotFound {
			return false, nil;
		}
		return false, err;
	}

	return true, nil;
}

func (e *Etcd) RevertInit(ctx context.Context) {
	e.kApi.Delete(ctx, rootDir, &client.DeleteOptions{
		Recursive: true,
	});
}
