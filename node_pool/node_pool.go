/*
 * Copyright 2024 The RuleGo Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package node_pool manages shared node resources, allowing for efficient reuse of node instances
// across different rule chains and executions.
package node_pool

import (
	"errors"
	"fmt"
	"sync"

	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/engine"
)

var (
	ErrNotImplemented = errors.New("not SharedNode")
)
var _ types.NodePool = (*NodePool)(nil)

// DefaultNodePool 默认组件资源池管理器
var DefaultNodePool = NewNodePool(engine.NewConfig())

// NodePool is a component resource pool manager
type NodePool struct {
	Config types.Config
	// key:resourceId value:sharedNodeCtx
	entries sync.Map
}

func NewNodePool(config types.Config) *NodePool {
	return &NodePool{
		Config: config,
	}
}

func (n *NodePool) Load(dsl []byte) (types.NodePool, error) {
	if def, err := n.Config.Parser.DecodeRuleChain(dsl); err != nil {
		return nil, err
	} else {
		return n.LoadFromRuleChain(def)
	}
}

func (n *NodePool) LoadFromRuleChain(def types.RuleChain) (types.NodePool, error) {
	for _, item := range def.Metadata.Endpoints {
		if item != nil {
			if _, err := n.NewFromEndpoint(*item); err != nil {
				return nil, err
			}
		}
	}
	for _, item := range def.Metadata.Nodes {
		if item != nil {
			if _, err := n.NewFromRuleNode(*item); err != nil {
				return nil, err
			}
		}
	}
	return n, nil
}

func (n *NodePool) NewFromEndpoint(def types.EndpointDsl) (types.SharedNodeCtx, error) {
	if _, ok := n.entries.Load(def.Id); ok {
		return nil, fmt.Errorf("duplicate node id:%s", def.Id)
	}

	if ctx, err := endpoint.NewFromDef(types.EndpointDsl{RuleNode: def.RuleNode}, endpointApi.DynamicEndpointOptions.WithRestart(true)); err == nil {
		if _, ok := ctx.Target().(types.SharedNode); !ok {
			return nil, ErrNotImplemented
		} else {
			rCtx := newSharedNodeCtx(nil, ctx)
			n.entries.Store(rCtx.GetNodeId().Id, rCtx)
			return rCtx, nil
		}
	} else {
		return nil, err
	}

}

func (n *NodePool) NewFromRuleNode(def types.RuleNode) (types.SharedNodeCtx, error) {
	if _, ok := n.entries.Load(def.Id); ok {
		return nil, fmt.Errorf("duplicate node id:%s", def.Id)
	}
	if ctx, err := engine.InitNetResourceNodeCtx(n.Config, nil, nil, &def); err == nil {
		if _, ok := ctx.Node.(types.SharedNode); !ok {
			return nil, ErrNotImplemented
		} else {
			rCtx := newSharedNodeCtx(ctx, nil)
			n.entries.Store(rCtx.GetNodeId().Id, rCtx)
			return rCtx, nil
		}
	} else {
		return nil, err
	}
}

// Get retrieves a SharedNode by its ID.
func (n *NodePool) Get(id string) (types.SharedNodeCtx, bool) {
	if v, ok := n.entries.Load(id); ok {
		return v.(*sharedNodeCtx), ok
	} else {
		return nil, false
	}
}

// GetInstance retrieves a net client or server connection by its ID.
func (n *NodePool) GetInstance(id string) (interface{}, error) {
	if ctx, ok := n.Get(id); ok {
		return ctx.GetInstance()
	} else {
		return nil, fmt.Errorf("node resource not found id=%s", id)
	}
}

// Del deletes a SharedNode instance by its ID.
func (n *NodePool) Del(id string) {
	if v, ok := n.entries.Load(id); ok {
		v.(*sharedNodeCtx).Destroy()
		n.entries.Delete(id)
	}
}

// Stop stops and releases all SharedNode instances.
func (n *NodePool) Stop() {
	n.entries.Range(func(key, value any) bool {
		n.Del(key.(string))
		return true
	})
}

// GetAll get all SharedNode instances
func (n *NodePool) GetAll() []types.SharedNodeCtx {
	var items []types.SharedNodeCtx
	n.entries.Range(func(key, value any) bool {
		items = append(items, value.(*sharedNodeCtx))
		return true
	})
	return items
}

func (n *NodePool) GetAllDef() (map[string][]*types.RuleNode, error) {
	var result = make(map[string][]*types.RuleNode)
	var resultErr error
	n.entries.Range(func(key, value any) bool {
		ctx := value.(*sharedNodeCtx)
		def, err := n.Config.Parser.DecodeRuleNode(ctx.DSL())
		if err != nil {
			resultErr = err
			return false
		}
		if item, ok := result[ctx.SharedNode().Type()]; !ok {
			result[ctx.SharedNode().Type()] = []*types.RuleNode{&def}
		} else {
			item = append(item, &def)
		}
		return true
	})
	return result, resultErr
}

// Range iterates over all SharedNode instances in the pool.
func (n *NodePool) Range(f func(key, value any) bool) {
	n.entries.Range(f)
}

type sharedNodeCtx struct {
	*engine.RuleNodeCtx
	EndpointCtx *endpoint.DynamicEndpoint
	IsEndpoint  bool
}

func newSharedNodeCtx(nodeCtx *engine.RuleNodeCtx, endpointCtx *endpoint.DynamicEndpoint) *sharedNodeCtx {
	return &sharedNodeCtx{RuleNodeCtx: nodeCtx, EndpointCtx: endpointCtx}
}

// GetInstance retrieves a net client or server connection.
// Node must implement types.SharedNode interface
func (n *sharedNodeCtx) GetInstance() (interface{}, error) {
	if n.EndpointCtx != nil {
		return n.EndpointCtx.Endpoint.(types.SharedNode).GetInstance()
	}
	return n.RuleNodeCtx.Node.(types.SharedNode).GetInstance()
}
func (n *sharedNodeCtx) GetNode() interface{} {
	if n.EndpointCtx != nil {
		return n.EndpointCtx.Endpoint
	}
	return n.RuleNodeCtx.Node
}

func (n *sharedNodeCtx) DSL() []byte {
	if n.EndpointCtx != nil {
		return n.EndpointCtx.DSL()
	}
	return n.RuleNodeCtx.DSL()
}

func (n *sharedNodeCtx) GetNodeId() types.RuleNodeId {
	if n.EndpointCtx != nil {
		return n.EndpointCtx.GetNodeId()
	}
	return n.RuleNodeCtx.GetNodeId()
}
func (n *sharedNodeCtx) SharedNode() types.SharedNode {
	if n.EndpointCtx != nil {
		return n.EndpointCtx.Target().(types.SharedNode)
	}
	return n.RuleNodeCtx.Node.(types.SharedNode)
}

func (n *sharedNodeCtx) Destroy() {
	if n.EndpointCtx != nil {
		n.EndpointCtx.Destroy()
	} else {
		n.RuleNodeCtx.Destroy()
	}
}
