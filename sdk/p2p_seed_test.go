package sdk

import (
	"context"
	"strings"
	"testing"
)

// TestConnectToSeedNodeOverLoopback exercises the real seed connection.
//
// This was skipped as "requires network connectivity". It does not: the seed
// runs in this process on the loopback interface, which is how the sync tests
// have always started listeners.
func TestConnectToSeedNodeOverLoopback(t *testing.T) {
	seedChain := forkTestChain(t, 2, uint32(genesisDifficulty))
	seed, seedAddress := startPeer(t, "seed", seedChain)

	// A third node the seed knows about, so the returned list is not empty.
	otherChain := forkTestChain(t, 0, uint32(genesisDifficulty))
	other, otherAddress := startPeer(t, "other", otherChain)
	if err := seed.RegisterNode(&Node{
		ID:     other.Identity().NodeID,
		Config: &Config{P2PHostName: otherAddress},
	}); err != nil {
		t.Fatalf("register other node with the seed: %v", err)
	}

	client := newTestP2P(t, freePort(t))
	client.SetChain(forkTestChain(t, 0, uint32(genesisDifficulty)))

	if err := client.ConnectToSeedNode(context.Background(), seedAddress); err != nil {
		t.Fatalf("connect to seed: %v", err)
	}

	// The seed itself must be remembered, not merely the peers it advertises.
	// Recording only the advertised list meant a node could not talk back to the
	// seed it had just used.
	peers := client.SyncPeers()
	var haveSeed, haveOther bool
	for _, peer := range peers {
		switch peer.ID {
		case seed.Identity().NodeID:
			haveSeed = true
		case other.Identity().NodeID:
			haveOther = true
		}
	}

	if !haveSeed {
		t.Fatalf("the seed itself was not recorded as a peer; known peers: %v", peers)
	}
	if !haveOther {
		t.Fatalf("the peers the seed advertised were not recorded; known peers: %v", peers)
	}
}

// TestConnectToSeedNodeRejectsAnUnreachableAddress: a seed that is not there must
// produce an error, not a silent success that leaves the node isolated.
func TestConnectToSeedNodeRejectsAnUnreachableAddress(t *testing.T) {
	client := newTestP2P(t, freePort(t))
	client.SetChain(forkTestChain(t, 0, uint32(genesisDifficulty)))

	// A port nothing is listening on.
	dead := freePort(t)

	err := client.ConnectToSeedNode(context.Background(), dead)
	if err == nil {
		t.Fatal("connecting to an address with no seed on it reported success")
	}
	if !strings.Contains(err.Error(), "failed to connect to seed node") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestConnectToSeedNodeSkipsItself guards a node adding itself to its own peer
// list, which would make it try to sync with itself.
func TestConnectToSeedNodeSkipsItself(t *testing.T) {
	seedChain := forkTestChain(t, 1, uint32(genesisDifficulty))
	seed, seedAddress := startPeer(t, "seed", seedChain)

	client := newTestP2P(t, freePort(t))
	client.SetChain(forkTestChain(t, 0, uint32(genesisDifficulty)))

	// Tell the seed about the client, so the list it returns includes it.
	if err := seed.RegisterNode(&Node{
		ID:     client.Identity().NodeID,
		Config: &Config{P2PHostName: client.selfInfo().Address},
	}); err != nil {
		t.Fatalf("register client with the seed: %v", err)
	}

	if err := client.ConnectToSeedNode(context.Background(), seedAddress); err != nil {
		t.Fatalf("connect to seed: %v", err)
	}

	for _, peer := range client.SyncPeers() {
		if peer.ID == client.Identity().NodeID {
			t.Fatal("the node recorded itself as a peer")
		}
	}
}
