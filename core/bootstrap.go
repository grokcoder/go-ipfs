package core

import (
	"sync"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	"github.com/jbenet/go-ipfs/config"
	inet "github.com/jbenet/go-ipfs/net"
	"github.com/jbenet/go-ipfs/peer"
	"github.com/jbenet/go-ipfs/routing/dht"
)

const period time.Duration = 30 * time.Second
const timeout time.Duration = period / 3

func superviseConnections(parent context.Context,
	n *IpfsNode,
	route *dht.IpfsDHT,
	store peer.Peerstore,
	peers []*config.BootstrapPeer) error {

	for {
		ctx, _ := context.WithTimeout(parent, timeout)
		// TODO get config from disk so |peers| always reflects the latest
		// information
		if err := bootstrap(ctx, n.Network, route, store, peers); err != nil {
			log.Error(err)
		}
		select {
		case <-parent.Done():
			return parent.Err()
		case <-time.Tick(period):
		}
	}
	return nil
}

func bootstrap(ctx context.Context,
	n inet.Network,
	r *dht.IpfsDHT,
	ps peer.Peerstore,
	boots []*config.BootstrapPeer) error {

	var peers []peer.Peer
	for _, bootstrap := range boots {
		p, err := toPeer(ps, bootstrap)
		if err != nil {
			return err
		}
		peers = append(peers, p)
	}

	var notConnected []peer.Peer
	for _, p := range peers {
		if !n.IsConnected(p) {
			notConnected = append(notConnected, p)
		}
	}
	for _, p := range notConnected {
		log.Infof("not connected to %v", p)
	}
	if err := connect(ctx, r, notConnected); err != nil {
		return err
	}
	return nil
}

func connect(ctx context.Context, r *dht.IpfsDHT, peers []peer.Peer) error {
	var wg sync.WaitGroup
	for _, p := range peers {

		// performed asynchronously because when performed synchronously, if
		// one `Connect` call hangs, subsequent calls are more likely to
		// fail/abort due to an expiring context.

		wg.Add(1)
		go func(p peer.Peer) {
			defer wg.Done()
			err := r.Connect(ctx, p)
			if err != nil {
				log.Event(ctx, "bootstrapFailed", p)
				log.Criticalf("failed to bootstrap with %v", p)
				return
			}
			log.Event(ctx, "bootstrapSuccess", p)
			log.Infof("bootstrapped with %v", p)
		}(p)
	}
	wg.Wait()
	return nil
}

func toPeer(ps peer.Peerstore, bootstrap *config.BootstrapPeer) (peer.Peer, error) {
	id, err := peer.DecodePrettyID(bootstrap.PeerID)
	if err != nil {
		return nil, err
	}
	p, err := ps.FindOrCreate(id)
	if err != nil {
		return nil, err
	}
	maddr, err := ma.NewMultiaddr(bootstrap.Address)
	if err != nil {
		return nil, err
	}
	p.AddAddress(maddr)
	return p, nil
}
