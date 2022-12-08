// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package minifactory

import (
	"context"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
)

func processOptions(opts ...protocol.StateOption) (*protocol.StateConfig, error) {
	cfg, err := protocol.CreateStateConfig(opts...)
	if err != nil {
		return nil, err
	}
	if len(cfg.Namespace) == 0 {
		cfg.Namespace = factory.AccountKVNamespace
	}
	return cfg, nil
}

func appendActionIndex(accountNonceMap map[string][]uint64, srcAddr string, nonce uint64) {
	if nonce == 0 {
		return
	}
	if _, ok := accountNonceMap[srcAddr]; !ok {
		accountNonceMap[srcAddr] = make([]uint64, 0)
	}
	accountNonceMap[srcAddr] = append(accountNonceMap[srcAddr], nonce)
}

func calculateReceiptRoot(receipts []*action.Receipt) hash.Hash256 {
	if len(receipts) == 0 {
		return hash.ZeroHash256
	}
	h := make([]hash.Hash256, 0, len(receipts))
	for _, receipt := range receipts {
		h = append(h, receipt.Hash())
	}
	res := crypto.NewMerkleTree(h).HashTree()
	return res
}

// generateWorkingSetCacheKey generates hash key for workingset cache by hashing blockheader core and producer pubkey
func generateWorkingSetCacheKey(blkHeader block.Header, producerAddr string) hash.Hash256 {
	sum := append(blkHeader.SerializeCore(), []byte(producerAddr)...)
	return hash.Hash256b(sum)
}

func protocolPreCommit(ctx context.Context, sr protocol.StateManager) error {
	if reg, ok := protocol.GetRegistry(ctx); ok {
		for _, p := range reg.All() {
			post, ok := p.(protocol.PreCommitter)
			if ok && sr.ProtocolDirty(p.Name()) {
				if err := post.PreCommit(ctx, sr); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func protocolCommit(ctx context.Context, sr protocol.StateManager) error {
	if reg, ok := protocol.GetRegistry(ctx); ok {
		for _, p := range reg.All() {
			post, ok := p.(protocol.Committer)
			if ok && sr.ProtocolDirty(p.Name()) {
				if err := post.Commit(ctx, sr); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func readStates(kvStore db.KVStore, namespace string, keys [][]byte) ([][]byte, error) {
	if keys == nil {
		_, values, err := kvStore.Filter(namespace, func(k, v []byte) bool { return true }, nil, nil)
		if err != nil {
			if errors.Cause(err) == db.ErrNotExist || errors.Cause(err) == db.ErrBucketNotExist {
				return nil, errors.Wrapf(state.ErrStateNotExist, "failed to get states of ns = %x", namespace)
			}
			return nil, err
		}
		return values, nil
	}
	var values [][]byte
	for _, key := range keys {
		value, err := kvStore.Get(namespace, key)
		switch errors.Cause(err) {
		case db.ErrNotExist, db.ErrBucketNotExist:
			values = append(values, nil)
		case nil:
			values = append(values, value)
		default:
			return nil, err
		}
	}
	return values, nil
}
