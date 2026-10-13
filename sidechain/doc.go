// Package sidechain is everything an application needs to keep its own
// sidechain on go-basic-blockchain, with no dependency beyond the Go standard
// library: it does not link the node (no HTTP router, terminal UI, p2p stack or
// wallet store).
//
//   - Blocks and chains: ID, Block, NewBlock, Chain, Restore, Set.
//   - Anchors: BuildAnchor commits a chain's pending range; SignAnchor signs it
//     with the publisher's Key; VerifyAgainstChain audits blocks against one.
//   - Proofs: ProveRecord / VerifyRecord prove one payload against an anchor
//     alone, in O(log n); ProveBlock, VerifyBlockProof, VerifyPayloadProof.
//   - Codec: blocks, anchors and proofs have canonical JSON forms.
//   - Storage: FileStore, an append-only, crash-safe store per sidechain.
//   - The main chain: Key (ECDSA P-256 wallets and publisher identities),
//     NewPublisherRegistration / NewGameRegistration / NewAnchorTransaction
//     (REGISTER and ANCHOR transactions in the node's exact wire form), and
//     Client, which submits them and reads publishers, anchors, accounts and
//     transaction status from a node.
//
// The node (package sdk) uses these very definitions, so a hash, root, proof or
// transaction an application produces is the one the node computes.
//
// A typical publisher:
//
//	wallet, identity := ... // two *Key (or one used for both)
//	c, _ := sidechain.NewClient(nodeURL, apiKey)
//	acct, _ := c.Account(ctx, wallet.Address())
//	reg, _ := sidechain.NewPublisherRegistration(wallet, identity, "Studio", bond, sidechain.TxOptions{Nonce: acct.NextNonce()})
//	c.Submit(ctx, reg)                          // then FindPublisherByKey until it is mined
//	game, _ := sidechain.NewGameRegistration(wallet, identity, publisherID, "Game", ...)
//	...
//	anchor, _ := sidechain.BuildAnchor(chain)   // nil when nothing is pending
//	sig, _ := sidechain.SignAnchor(identity, *anchor)
//	tx, _ := sidechain.NewAnchorTransaction(wallet, *anchor, sig, ...)
//	c.Submit(ctx, tx)
//	info, _ := c.WaitConfirmed(ctx, tx.ID(), time.Second)
//	chain.MarkAnchored(anchor.ToHeight)
package sidechain
