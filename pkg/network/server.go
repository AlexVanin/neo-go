package network

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/CityOfZion/neo-go/pkg/consensus"
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/core/block"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/CityOfZion/neo-go/pkg/util"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

const (
	// peer numbers are arbitrary at the moment.
	defaultMinPeers         = 5
	defaultAttemptConnPeers = 20
	defaultMaxPeers         = 100
	maxBlockBatch           = 200
	maxAddrsToSend          = 200
	minPoolCount            = 30
)

var (
	errAlreadyConnected = errors.New("already connected")
	errIdenticalID      = errors.New("identical node id")
	errInvalidHandshake = errors.New("invalid handshake")
	errInvalidNetwork   = errors.New("invalid network")
	errMaxPeers         = errors.New("max peers reached")
	errServerShutdown   = errors.New("server shutdown")
	errInvalidInvType   = errors.New("invalid inventory type")
	errInvalidHashStart = errors.New("invalid requested HashStart")
)

type (
	// Server represents the local Node in the network. Its transport could
	// be of any kind.
	Server struct {
		// ServerConfig holds the Server configuration.
		ServerConfig

		// id also known as the nonce of the server.
		id uint32

		transport Transporter
		discovery Discoverer
		chain     core.Blockchainer
		bQueue    *blockQueue
		consensus consensus.Service

		lock  sync.RWMutex
		peers map[Peer]bool

		register   chan Peer
		unregister chan peerDrop
		quit       chan struct{}

		connected *atomic.Bool

		log *zap.Logger
	}

	peerDrop struct {
		peer   Peer
		reason error
	}
)

func randomID() uint32 {
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	return binary.BigEndian.Uint32(buf)
}

// NewServer returns a new Server, initialized with the given configuration.
func NewServer(config ServerConfig, chain core.Blockchainer, log *zap.Logger) (*Server, error) {
	if log == nil {
		return nil, errors.New("logger is a required parameter")
	}

	s := &Server{
		ServerConfig: config,
		chain:        chain,
		bQueue:       newBlockQueue(maxBlockBatch, chain, log),
		id:           randomID(),
		quit:         make(chan struct{}),
		register:     make(chan Peer),
		unregister:   make(chan peerDrop),
		peers:        make(map[Peer]bool),
		connected:    atomic.NewBool(false),
		log:          log,
	}

	srv, err := consensus.NewService(consensus.Config{
		Logger:     log,
		Broadcast:  s.handleNewPayload,
		RelayBlock: s.relayBlock,
		Chain:      chain,
		RequestTx:  s.requestTx,
		Wallet:     config.Wallet,

		TimePerBlock: config.TimePerBlock,
	})
	if err != nil {
		return nil, err
	}

	s.consensus = srv

	if s.MinPeers < 0 {
		s.log.Info("bad MinPeers configured, using the default value",
			zap.Int("configured", s.MinPeers),
			zap.Int("actual", defaultMinPeers))
		s.MinPeers = defaultMinPeers
	}

	if s.MaxPeers <= 0 {
		s.log.Info("bad MaxPeers configured, using the default value",
			zap.Int("configured", s.MaxPeers),
			zap.Int("actual", defaultMaxPeers))
		s.MaxPeers = defaultMaxPeers
	}

	if s.AttemptConnPeers <= 0 {
		s.log.Info("bad AttemptConnPeers configured, using the default value",
			zap.Int("configured", s.AttemptConnPeers),
			zap.Int("actual", defaultAttemptConnPeers))
		s.AttemptConnPeers = defaultAttemptConnPeers
	}

	s.transport = NewTCPTransport(s, fmt.Sprintf("%s:%d", config.Address, config.Port), s.log)
	s.discovery = NewDefaultDiscovery(
		s.DialTimeout,
		s.transport,
	)

	return s, nil
}

// MkMsg creates a new message based on the server configured network and given
// parameters.
func (s *Server) MkMsg(cmd CommandType, p payload.Payload) *Message {
	return NewMessage(s.Net, cmd, p)
}

// ID returns the servers ID.
func (s *Server) ID() uint32 {
	return s.id
}

// Start will start the server and its underlying transport.
func (s *Server) Start(errChan chan error) {
	s.log.Info("node started",
		zap.Uint32("blockHeight", s.chain.BlockHeight()),
		zap.Uint32("headerHeight", s.chain.HeaderHeight()))

	s.tryStartConsensus()

	s.discovery.BackFill(s.Seeds...)

	go s.bQueue.run()
	go s.transport.Accept()
	setServerAndNodeVersions(s.UserAgent, strconv.FormatUint(uint64(s.id), 10))
	s.run()
}

// Shutdown disconnects all peers and stops listening.
func (s *Server) Shutdown() {
	s.log.Info("shutting down server", zap.Int("peers", s.PeerCount()))
	s.bQueue.discard()
	close(s.quit)
}

// UnconnectedPeers returns a list of peers that are in the discovery peer list
// but are not connected to the server.
func (s *Server) UnconnectedPeers() []string {
	return []string{}
}

// BadPeers returns a list of peers the are flagged as "bad" peers.
func (s *Server) BadPeers() []string {
	return []string{}
}

// run is a goroutine that starts another goroutine to manage protocol specifics
// while itself dealing with peers management (handling connects/disconnects).
func (s *Server) run() {
	go s.runProto()
	for {
		if s.PeerCount() < s.MinPeers {
			s.discovery.RequestRemote(s.AttemptConnPeers)
		}
		if s.discovery.PoolCount() < minPoolCount {
			s.broadcastHPMessage(s.MkMsg(CMDGetAddr, payload.NewNullPayload()))
		}
		select {
		case <-s.quit:
			s.transport.Close()
			for p := range s.peers {
				p.Disconnect(errServerShutdown)
			}
			return
		case p := <-s.register:
			s.lock.Lock()
			s.peers[p] = true
			s.lock.Unlock()
			peerCount := s.PeerCount()
			s.log.Info("new peer connected", zap.Stringer("addr", p.RemoteAddr()), zap.Int("peerCount", peerCount))
			if peerCount > s.MaxPeers {
				s.lock.RLock()
				// Pick a random peer and drop connection to it.
				for peer := range s.peers {
					peer.Disconnect(errMaxPeers)
					break
				}
				s.lock.RUnlock()
			}
			updatePeersConnectedMetric(s.PeerCount())

		case drop := <-s.unregister:
			s.lock.Lock()
			if s.peers[drop.peer] {
				delete(s.peers, drop.peer)
				s.lock.Unlock()
				s.log.Warn("peer disconnected",
					zap.Stringer("addr", drop.peer.RemoteAddr()),
					zap.String("reason", drop.reason.Error()),
					zap.Int("peerCount", s.PeerCount()))
				addr := drop.peer.PeerAddr().String()
				if drop.reason == errIdenticalID {
					s.discovery.RegisterBadAddr(addr)
				} else if drop.reason != errAlreadyConnected {
					s.discovery.UnregisterConnectedAddr(addr)
					s.discovery.BackFill(addr)
				}
				updatePeersConnectedMetric(s.PeerCount())
			} else {
				// else the peer is already gone, which can happen
				// because we have two goroutines sending signals here
				s.lock.Unlock()
			}

		}
	}
}

// runProto is a goroutine that manages server-wide protocol events.
func (s *Server) runProto() {
	pingTimer := time.NewTimer(s.PingInterval)
	for {
		prevHeight := s.chain.BlockHeight()
		select {
		case <-s.quit:
			return
		case <-pingTimer.C:
			if s.chain.BlockHeight() == prevHeight {
				// Get a copy of s.peers to avoid holding a lock while sending.
				for peer := range s.Peers() {
					_ = peer.SendPing(s.MkMsg(CMDPing, payload.NewPing(s.id, s.chain.HeaderHeight())))
				}
			}
			pingTimer.Reset(s.PingInterval)
		}
	}
}

func (s *Server) tryStartConsensus() {
	if s.Wallet == nil || s.connected.Load() {
		return
	}

	if s.HandshakedPeersCount() >= s.MinPeers {
		s.log.Info("minimum amount of peers were connected to")
		if s.connected.CAS(false, true) {
			s.consensus.Start()
		}
	}
}

// Peers returns the current list of peers connected to
// the server.
func (s *Server) Peers() map[Peer]bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	peers := make(map[Peer]bool, len(s.peers))
	for k, v := range s.peers {
		peers[k] = v
	}

	return peers
}

// PeerCount returns the number of current connected peers.
func (s *Server) PeerCount() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return len(s.peers)
}

// HandshakedPeersCount returns the number of connected peers
// which have already performed handshake.
func (s *Server) HandshakedPeersCount() int {
	s.lock.RLock()
	defer s.lock.RUnlock()

	var count int

	for p := range s.peers {
		if p.Handshaked() {
			count++
		}
	}

	return count
}

// getVersionMsg returns current version message.
func (s *Server) getVersionMsg() *Message {
	payload := payload.NewVersion(
		s.id,
		s.Port,
		s.UserAgent,
		s.chain.BlockHeight(),
		s.Relay,
	)
	return s.MkMsg(CMDVersion, payload)
}

// When a peer sends out his version we reply with verack after validating
// the version.
func (s *Server) handleVersionCmd(p Peer, version *payload.Version) error {
	err := p.HandleVersion(version)
	if err != nil {
		return err
	}
	if s.id == version.Nonce {
		return errIdenticalID
	}
	peerAddr := p.PeerAddr().String()
	s.discovery.RegisterConnectedAddr(peerAddr)
	s.lock.RLock()
	for peer := range s.peers {
		if p == peer {
			continue
		}
		ver := peer.Version()
		// Already connected, drop this connection.
		if ver != nil && ver.Nonce == version.Nonce && peer.PeerAddr().String() == peerAddr {
			s.lock.RUnlock()
			return errAlreadyConnected
		}
	}
	s.lock.RUnlock()
	return p.SendVersionAck(s.MkMsg(CMDVerack, nil))
}

// handleHeadersCmd processes the headers received from its peer.
// If the headerHeight of the blockchain still smaller then the peer
// the server will request more headers.
// This method could best be called in a separate routine.
func (s *Server) handleHeadersCmd(p Peer, headers *payload.Headers) {
	if err := s.chain.AddHeaders(headers.Hdrs...); err != nil {
		s.log.Warn("failed processing headers", zap.Error(err))
		return
	}
	// The peer will respond with a maximum of 2000 headers in one batch.
	// We will ask one more batch here if needed. Eventually we will get synced
	// due to the startProtocol routine that will ask headers every protoTick.
	if s.chain.HeaderHeight() < p.LastBlockIndex() {
		s.requestHeaders(p)
	}
}

// handleBlockCmd processes the received block received from its peer.
func (s *Server) handleBlockCmd(p Peer, block *block.Block) error {
	return s.bQueue.putBlock(block)
}

// handlePing processes ping request.
func (s *Server) handlePing(p Peer, ping *payload.Ping) error {
	return p.EnqueueP2PMessage(s.MkMsg(CMDPong, payload.NewPing(s.chain.BlockHeight(), s.id)))
}

// handlePing processes pong request.
func (s *Server) handlePong(p Peer, pong *payload.Ping) error {
	err := p.HandlePong(pong)
	if err != nil {
		return err
	}
	if s.chain.HeaderHeight() < pong.LastBlockIndex {
		return s.requestHeaders(p)
	}
	return nil
}

// handleInvCmd processes the received inventory.
func (s *Server) handleInvCmd(p Peer, inv *payload.Inventory) error {
	reqHashes := make([]util.Uint256, 0)
	var typExists = map[payload.InventoryType]func(util.Uint256) bool{
		payload.TXType:    s.chain.HasTransaction,
		payload.BlockType: s.chain.HasBlock,
		payload.ConsensusType: func(h util.Uint256) bool {
			cp := s.consensus.GetPayload(h)
			return cp != nil
		},
	}
	if exists := typExists[inv.Type]; exists != nil {
		for _, hash := range inv.Hashes {
			if !exists(hash) {
				reqHashes = append(reqHashes, hash)
			}
		}
	}
	if len(reqHashes) > 0 {
		msg := s.MkMsg(CMDGetData, payload.NewInventory(inv.Type, reqHashes))
		pkt, err := msg.Bytes()
		if err != nil {
			return err
		}
		if inv.Type == payload.ConsensusType {
			return p.EnqueueHPPacket(pkt)
		}
		return p.EnqueueP2PPacket(pkt)
	}
	return nil
}

// handleInvCmd processes the received inventory.
func (s *Server) handleGetDataCmd(p Peer, inv *payload.Inventory) error {
	for _, hash := range inv.Hashes {
		var msg *Message

		switch inv.Type {
		case payload.TXType:
			tx, _, err := s.chain.GetTransaction(hash)
			if err == nil {
				msg = s.MkMsg(CMDTX, tx)
			}
		case payload.BlockType:
			b, err := s.chain.GetBlock(hash)
			if err == nil {
				msg = s.MkMsg(CMDBlock, b)
			}
		case payload.ConsensusType:
			if cp := s.consensus.GetPayload(hash); cp != nil {
				msg = s.MkMsg(CMDConsensus, cp)
			}
		}
		if msg != nil {
			pkt, err := msg.Bytes()
			if err == nil {
				if inv.Type == payload.ConsensusType {
					err = p.EnqueueHPPacket(pkt)
				} else {
					err = p.EnqueueP2PPacket(pkt)
				}
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// handleGetBlocksCmd processes the getblocks request.
func (s *Server) handleGetBlocksCmd(p Peer, gb *payload.GetBlocks) error {
	if len(gb.HashStart) < 1 {
		return errInvalidHashStart
	}
	startHash := gb.HashStart[0]
	if startHash.Equals(gb.HashStop) {
		return nil
	}
	start, err := s.chain.GetHeader(startHash)
	if err != nil {
		return err
	}
	blockHashes := make([]util.Uint256, 0)
	for i := start.Index + 1; i < start.Index+1+payload.MaxHashesCount; i++ {
		hash := s.chain.GetHeaderHash(int(i))
		if hash.Equals(util.Uint256{}) || hash.Equals(gb.HashStop) {
			break
		}
		blockHashes = append(blockHashes, hash)
	}

	if len(blockHashes) == 0 {
		return nil
	}
	payload := payload.NewInventory(payload.BlockType, blockHashes)
	msg := s.MkMsg(CMDInv, payload)
	return p.EnqueueP2PMessage(msg)
}

// handleGetHeadersCmd processes the getheaders request.
func (s *Server) handleGetHeadersCmd(p Peer, gh *payload.GetBlocks) error {
	if len(gh.HashStart) < 1 {
		return errInvalidHashStart
	}
	startHash := gh.HashStart[0]
	start, err := s.chain.GetHeader(startHash)
	if err != nil {
		return err
	}
	resp := payload.Headers{}
	resp.Hdrs = make([]*block.Header, 0, payload.MaxHeadersAllowed)
	for i := start.Index + 1; i < start.Index+1+payload.MaxHeadersAllowed; i++ {
		hash := s.chain.GetHeaderHash(int(i))
		if hash.Equals(util.Uint256{}) || hash.Equals(gh.HashStop) {
			break
		}
		header, err := s.chain.GetHeader(hash)
		if err != nil {
			break
		}
		resp.Hdrs = append(resp.Hdrs, header)
	}
	if len(resp.Hdrs) == 0 {
		return nil
	}
	msg := s.MkMsg(CMDHeaders, &resp)
	return p.EnqueueP2PMessage(msg)
}

// handleConsensusCmd processes received consensus payload.
// It never returns an error.
func (s *Server) handleConsensusCmd(cp *consensus.Payload) error {
	s.consensus.OnPayload(cp)
	return nil
}

// handleTxCmd processes received transaction.
// It never returns an error.
func (s *Server) handleTxCmd(tx *transaction.Transaction) error {
	// It's OK for it to fail for various reasons like tx already existing
	// in the pool.
	if s.verifyAndPoolTX(tx) == RelaySucceed {
		s.consensus.OnTransaction(tx)
		go s.broadcastTX(tx)
	}
	return nil
}

// handleAddrCmd will process received addresses.
func (s *Server) handleAddrCmd(p Peer, addrs *payload.AddressList) error {
	for _, a := range addrs.Addrs {
		s.discovery.BackFill(a.IPPortString())
	}
	return nil
}

// handleGetAddrCmd sends to the peer some good addresses that we know of.
func (s *Server) handleGetAddrCmd(p Peer) error {
	addrs := s.discovery.GoodPeers()
	if len(addrs) > maxAddrsToSend {
		addrs = addrs[:maxAddrsToSend]
	}
	alist := payload.NewAddressList(len(addrs))
	ts := time.Now()
	for i, addr := range addrs {
		// we know it's a good address, so it can't fail
		netaddr, _ := net.ResolveTCPAddr("tcp", addr)
		alist.Addrs[i] = payload.NewAddressAndTime(netaddr, ts)
	}
	return p.EnqueueP2PMessage(s.MkMsg(CMDAddr, alist))
}

// requestHeaders sends a getheaders message to the peer.
// The peer will respond with headers op to a count of 2000.
func (s *Server) requestHeaders(p Peer) error {
	start := []util.Uint256{s.chain.CurrentHeaderHash()}
	payload := payload.NewGetBlocks(start, util.Uint256{})
	return p.EnqueueP2PMessage(s.MkMsg(CMDGetHeaders, payload))
}

// requestBlocks sends a getdata message to the peer
// to sync up in blocks. A maximum of maxBlockBatch will
// send at once.
func (s *Server) requestBlocks(p Peer) error {
	var (
		hashes       []util.Uint256
		hashStart    = s.chain.BlockHeight() + 1
		headerHeight = s.chain.HeaderHeight()
	)
	for hashStart <= headerHeight && len(hashes) < maxBlockBatch {
		hash := s.chain.GetHeaderHash(int(hashStart))
		hashes = append(hashes, hash)
		hashStart++
	}
	if len(hashes) > 0 {
		payload := payload.NewInventory(payload.BlockType, hashes)
		return p.EnqueueP2PMessage(s.MkMsg(CMDGetData, payload))
	} else if s.chain.HeaderHeight() < p.LastBlockIndex() {
		return s.requestHeaders(p)
	}
	return nil
}

// handleMessage processes the given message.
func (s *Server) handleMessage(peer Peer, msg *Message) error {
	s.log.Debug("got msg",
		zap.Stringer("addr", peer.RemoteAddr()),
		zap.String("type", string(msg.CommandType())))

	// Make sure both server and peer are operating on
	// the same network.
	if msg.Magic != s.Net {
		return errInvalidNetwork
	}

	if peer.Handshaked() {
		if inv, ok := msg.Payload.(*payload.Inventory); ok {
			if !inv.Type.Valid() || len(inv.Hashes) == 0 {
				return errInvalidInvType
			}
		}
		switch msg.CommandType() {
		case CMDAddr:
			addrs := msg.Payload.(*payload.AddressList)
			return s.handleAddrCmd(peer, addrs)
		case CMDGetAddr:
			// it has no payload
			return s.handleGetAddrCmd(peer)
		case CMDGetBlocks:
			gb := msg.Payload.(*payload.GetBlocks)
			return s.handleGetBlocksCmd(peer, gb)
		case CMDGetData:
			inv := msg.Payload.(*payload.Inventory)
			return s.handleGetDataCmd(peer, inv)
		case CMDGetHeaders:
			gh := msg.Payload.(*payload.GetBlocks)
			return s.handleGetHeadersCmd(peer, gh)
		case CMDHeaders:
			headers := msg.Payload.(*payload.Headers)
			go s.handleHeadersCmd(peer, headers)
		case CMDInv:
			inventory := msg.Payload.(*payload.Inventory)
			return s.handleInvCmd(peer, inventory)
		case CMDBlock:
			block := msg.Payload.(*block.Block)
			return s.handleBlockCmd(peer, block)
		case CMDConsensus:
			cp := msg.Payload.(*consensus.Payload)
			return s.handleConsensusCmd(cp)
		case CMDTX:
			tx := msg.Payload.(*transaction.Transaction)
			return s.handleTxCmd(tx)
		case CMDPing:
			ping := msg.Payload.(*payload.Ping)
			return s.handlePing(peer, ping)
		case CMDPong:
			pong := msg.Payload.(*payload.Ping)
			return s.handlePong(peer, pong)
		case CMDVersion, CMDVerack:
			return fmt.Errorf("received '%s' after the handshake", msg.CommandType())
		}
	} else {
		switch msg.CommandType() {
		case CMDVersion:
			version := msg.Payload.(*payload.Version)
			return s.handleVersionCmd(peer, version)
		case CMDVerack:
			err := peer.HandleVersionAck()
			if err != nil {
				return err
			}
			go peer.StartProtocol()

			s.tryStartConsensus()
		default:
			return fmt.Errorf("received '%s' during handshake", msg.CommandType())
		}
	}
	return nil
}

func (s *Server) handleNewPayload(p *consensus.Payload) {
	msg := s.MkMsg(CMDInv, payload.NewInventory(payload.ConsensusType, []util.Uint256{p.Hash()}))
	// It's high priority because it directly affects consensus process,
	// even though it's just an inv.
	s.broadcastHPMessage(msg)
}

func (s *Server) requestTx(hashes ...util.Uint256) {
	if len(hashes) == 0 {
		return
	}

	msg := s.MkMsg(CMDGetData, payload.NewInventory(payload.TXType, hashes))
	// It's high priority because it directly affects consensus process,
	// even though it's getdata.
	s.broadcastHPMessage(msg)
}

// iteratePeersWithSendMsg sends given message to all peers using two functions
// passed, one is to send the message and the other is to filtrate peers (the
// peer is considered invalid if it returns false).
func (s *Server) iteratePeersWithSendMsg(msg *Message, send func(Peer, []byte) error, peerOK func(Peer) bool) {
	pkt, err := msg.Bytes()
	if err != nil {
		return
	}
	// Get a copy of s.peers to avoid holding a lock while sending.
	for peer := range s.Peers() {
		if peerOK != nil && !peerOK(peer) {
			continue
		}
		// Who cares about these messages anyway?
		_ = send(peer, pkt)
	}
}

// broadcastMessage sends the message to all available peers.
func (s *Server) broadcastMessage(msg *Message) {
	s.iteratePeersWithSendMsg(msg, Peer.EnqueuePacket, nil)
}

// broadcastHPMessage sends the high-priority message to all available peers.
func (s *Server) broadcastHPMessage(msg *Message) {
	s.iteratePeersWithSendMsg(msg, Peer.EnqueueHPPacket, nil)
}

// relayBlock tells all the other connected nodes about the given block.
func (s *Server) relayBlock(b *block.Block) {
	msg := s.MkMsg(CMDInv, payload.NewInventory(payload.BlockType, []util.Uint256{b.Hash()}))
	s.broadcastMessage(msg)
}

// verifyAndPoolTX verifies the TX and adds it to the local mempool.
func (s *Server) verifyAndPoolTX(t *transaction.Transaction) RelayReason {
	if t.Type == transaction.MinerType {
		return RelayInvalid
	}
	// TODO: Implement Plugin.CheckPolicy?
	//if (!Plugin.CheckPolicy(transaction))
	// return RelayResultReason.PolicyFail;
	if err := s.chain.PoolTx(t); err != nil {
		switch err {
		case core.ErrAlreadyExists:
			return RelayAlreadyExists
		case core.ErrOOM:
			return RelayOutOfMemory
		default:
			return RelayInvalid
		}
	}
	return RelaySucceed
}

// RelayTxn a new transaction to the local node and the connected peers.
// Reference: the method OnRelay in C#: https://github.com/neo-project/neo/blob/master/neo/Network/P2P/LocalNode.cs#L159
func (s *Server) RelayTxn(t *transaction.Transaction) RelayReason {
	ret := s.verifyAndPoolTX(t)
	if ret == RelaySucceed {
		s.broadcastTX(t)
	}
	return ret
}

// broadcastTX broadcasts an inventory message about new transaction.
func (s *Server) broadcastTX(t *transaction.Transaction) {
	msg := s.MkMsg(CMDInv, payload.NewInventory(payload.TXType, []util.Uint256{t.Hash()}))

	// We need to filter out non-relaying nodes, so plain broadcast
	// functions don't fit here.
	s.iteratePeersWithSendMsg(msg, Peer.EnqueuePacket, func(p Peer) bool {
		return p.Handshaked() && p.Version().Relay
	})
}
