package controller

import (
	"examples/server/config"
	"examples/server/config/logger"
	"examples/server/internal/constants"
	"examples/server/internal/service"
	"fmt"
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/builtin/processor"
	"github.com/rulego/rulego/components/action"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/node_pool"
	"github.com/rulego/rulego/utils/json"
	"net/http"
	"path"
	"strconv"
)

var AuthProcess = func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
	msg := exchange.In.GetMsg()
	username := exchange.In.Headers().Get(constants.KeyUsername)
	if username == "" {
		username = config.C.DefaultUsername
	}
	msg.Metadata.PutValue(constants.KeyUsername, username)
	//TODO JWT 权限校验
	return true
}

// ComponentsRouter 创建获取规则引擎节点组件列表路由
func ComponentsRouter(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		nodePool, _ := node_pool.DefaultNodePool.GetAllDef()
		//响应endpoint和节点组件配置表单列表
		list, err := json.Marshal(map[string]interface{}{
			//endpoint组件
			"endpoints": endpoint.Registry.GetComponentForms().Values(),
			//节点组件
			"nodes": rulego.Registry.GetComponentForms().Values(),
			//组件配置内置选项
			"builtins": map[string]interface{}{
				// functions节点组件
				"functions": map[string]interface{}{
					//函数名选项
					"functionName": action.Functions.Names(),
				},
				//endpoints内置路由选项
				"endpoints": map[string]interface{}{
					//in 处理器列表
					"inProcessors": processor.InBuiltins.Names(),
					//in 处理器列表
					"outProcessors": processor.OutBuiltins.Names(),
				},
				//共享节点池
				"nodePool": nodePool,
			},
		})
		if err != nil {
			exchange.Out.SetStatusCode(http.StatusInternalServerError)
			exchange.Out.SetBody([]byte(err.Error()))
		} else {
			exchange.Out.SetBody(list)
		}
		return true
	}).End()
}

// GetDslRouter 创建获取指定规则链路由
func GetDslRouter(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		chainId := msg.Metadata.GetValue(constants.KeyId)
		username := msg.Metadata.GetValue(constants.KeyUsername)
		if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
			if def, err := s.Get(chainId); err == nil {
				exchange.Out.SetBody(def)
			} else {
				exchange.Out.SetStatusCode(http.StatusNotFound)
				return false
			}
		} else {
			return userNotFound(username, exchange)
		}
		return true
	}).End()
}

// SaveDslRouter 创建保存/更新指定规则链路由
func SaveDslRouter(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		chainId := msg.Metadata.GetValue(constants.KeyId)
		username := msg.Metadata.GetValue(constants.KeyUsername)
		if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
			if err := s.Save(chainId, exchange.In.Body()); err == nil {
				exchange.Out.SetStatusCode(http.StatusOK)
			} else {
				logger.Logger.Println(err)
				exchange.Out.SetStatusCode(http.StatusBadRequest)
				exchange.Out.SetBody([]byte(err.Error()))
			}
		} else {
			return userNotFound(username, exchange)
		}
		return true
	}).End()
}

// ListDslRouter 创建获取所有规则链路由
func ListDslRouter(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		username := msg.Metadata.GetValue(constants.KeyUsername)
		keywords := msg.Metadata.GetValue(constants.KeyKeywords)
		chainTypeStr := msg.Metadata.GetValue(constants.KeyType)
		var page = 1
		var size = 20
		currentStr := msg.Metadata.GetValue(constants.KeyPage)
		if i, err := strconv.Atoi(currentStr); err == nil {
			page = i
		}
		pageSizeStr := msg.Metadata.GetValue(constants.KeySize)
		if i, err := strconv.Atoi(pageSizeStr); err == nil {
			size = i
		}
		chainType := 0
		if i, err := strconv.Atoi(chainTypeStr); err == nil {
			chainType = i
		}

		if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
			list, count, err := s.List(keywords, chainType, size, page)
			if err != nil {
				exchange.Out.SetStatusCode(http.StatusInternalServerError)
				exchange.Out.SetBody([]byte(err.Error()))
				return true
			}
			result := map[string]interface{}{
				"total": count,
				"page":  page,
				"size":  size,
				"items": list,
			}
			if v, err := json.Marshal(result); err == nil {
				exchange.Out.SetBody(v)
			} else {
				logger.Logger.Println(err)
				exchange.Out.SetStatusCode(http.StatusBadRequest)
				exchange.Out.SetBody([]byte(err.Error()))
			}
		} else {
			return userNotFound(username, exchange)
		}
		return true
	}).End()
}

// DeleteDslRouter 创建删除指定规则链路由
func DeleteDslRouter(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		chainId := msg.Metadata.GetValue(constants.KeyId)
		username := msg.Metadata.GetValue(constants.KeyUsername)
		if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
			if err := s.Delete(chainId); err == nil {
				exchange.Out.SetStatusCode(http.StatusOK)
			} else {
				logger.Logger.Println(err)
				exchange.Out.SetStatusCode(http.StatusBadRequest)
				exchange.Out.SetBody([]byte(err.Error()))
			}
		} else {
			return userNotFound(username, exchange)
		}
		return true
	}).End()
}

// SaveBaseInfo 保存规则链扩展信息
func SaveBaseInfo(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		chainId := msg.Metadata.GetValue(constants.KeyChainId)
		username := msg.Metadata.GetValue(constants.KeyUsername)
		var req types.RuleChainBaseInfo
		if err := json.Unmarshal([]byte(msg.Data), &req); err != nil {
			exchange.Out.SetStatusCode(http.StatusBadRequest)
			exchange.Out.SetBody([]byte(err.Error()))
		} else {
			if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
				if err := s.SaveBaseInfo(chainId, req); err != nil {
					exchange.Out.SetStatusCode(http.StatusBadRequest)
					exchange.Out.SetBody([]byte(err.Error()))
				}

			} else {
				return userNotFound(username, exchange)
			}
		}
		return true
	}).End()
}

// SaveConfiguration 保存规则链配置
func SaveConfiguration(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		chainId := msg.Metadata.GetValue(constants.KeyId)
		username := msg.Metadata.GetValue(constants.KeyUsername)
		varType := msg.Metadata.GetValue(constants.KeyVarType)
		var req interface{}
		if err := json.Unmarshal([]byte(msg.Data), &req); err != nil {
			exchange.Out.SetStatusCode(http.StatusBadRequest)
			exchange.Out.SetBody([]byte(err.Error()))
		} else {
			if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
				if err := s.SaveConfiguration(chainId, varType, req); err != nil {
					exchange.Out.SetStatusCode(http.StatusBadRequest)
					exchange.Out.SetBody([]byte(err.Error()))
				}
			} else {
				return userNotFound(username, exchange)
			}
		}
		return true
	}).End()
}

// ExecuteRuleRouter 处理请求，并转发到规则引擎，同步等待规则链执行结果返回给调用方
func ExecuteRuleRouter(url string) endpointApi.Router {
	var opts []types.RuleContextOption
	if config.C.SaveRunLog {
		opts = append(opts, types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
			_ = service.EventServiceImpl.SaveRunLog(ctx, snapshot)
		}))
	}

	return endpoint.NewRouter(endpointApi.RouterOptions.WithRuleGoFunc(GetRuleGoFunc)).From(url).Process(AuthProcess).Transform(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		msgId := exchange.In.GetParam("msgId")
		if msgId != "" {
			msg.Id = msgId
		}
		msgType := msg.Metadata.GetValue("msgType")
		//获取消息类型
		msg.Type = msgType
		//把http header放入消息元数据
		headers := exchange.In.Headers()
		for k := range headers {
			msg.Metadata.PutValue(k, headers.Get(k))
		}
		username := msg.Metadata.GetValue(constants.KeyUsername)
		//设置工作目录
		var paths = []string{config.C.DataDir, constants.DirWorkflows, username, constants.DirWorkflowsRule}
		msg.Metadata.PutValue(constants.KeyWorkDir, path.Join(paths...))
		return true
	}).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		exchange.Out.Headers().Set("Content-Type", "application/json")
		return true
	}).To("chain:${id}").SetOpts(opts...).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		err := exchange.Out.GetError()
		if err != nil {
			//错误
			exchange.Out.SetStatusCode(http.StatusBadRequest)
			exchange.Out.SetBody([]byte(exchange.Out.GetError().Error()))
		} else {
			//把处理结果响应给客户端，http endpoint 必须增加 Wait()，否则无法正常响应
			outMsg := exchange.Out.GetMsg()
			exchange.Out.SetBody([]byte(outMsg.Data))
		}
		return true
	}).Wait().End()
}

// PostMsgRouter 处理请求，并转发到规则引擎
func PostMsgRouter(url string) endpointApi.Router {
	var opts []types.RuleContextOption
	if config.C.SaveRunLog {
		opts = append(opts, types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
			_ = service.EventServiceImpl.SaveRunLog(ctx, snapshot)
		}))
	}
	return endpoint.NewRouter(endpointApi.RouterOptions.WithRuleGoFunc(GetRuleGoFunc)).From(url).Process(AuthProcess).Transform(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		msgId := exchange.In.GetParam("msgId")
		if msgId != "" {
			msg.Id = msgId
		}
		//把http header放入消息元数据
		headers := exchange.In.Headers()
		for k := range headers {
			msg.Metadata.PutValue(k, headers.Get(k))
		}
		msgType := msg.Metadata.GetValue("msgType")
		//获取消息类型
		msg.Type = msgType
		username := msg.Metadata.GetValue(constants.KeyUsername)
		//设置工作目录
		var paths = []string{config.C.DataDir, constants.DirWorkflows, username, constants.DirWorkflowsRule}
		msg.Metadata.PutValue(constants.KeyWorkDir, path.Join(paths...))
		return true
	}).To("chain:${id}").SetOpts(opts...).End()
}

func OperateRule(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		chainId := msg.Metadata.GetValue(constants.KeyId)
		opType := msg.Metadata.GetValue(constants.KeyType)
		username := msg.Metadata.GetValue(constants.KeyUsername)
		if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
			if opType == constants.KeyDeploy {
				if err := s.Deploy(chainId); err != nil {
					exchange.Out.SetStatusCode(http.StatusBadRequest)
					exchange.Out.SetBody([]byte(err.Error()))
				}
			} else if opType == constants.KeyUndeploy {
				if err := s.Undeploy(chainId); err != nil {
					exchange.Out.SetStatusCode(http.StatusBadRequest)
					exchange.Out.SetBody([]byte(err.Error()))
				}
			} else {
				exchange.Out.SetStatusCode(http.StatusBadRequest)
				exchange.Out.SetBody([]byte("没有该操作类型:" + opType))
			}

		} else {
			return userNotFound(username, exchange)
		}
		return true
	}).End()
}

// userNotFound 用户不存在
func userNotFound(username string, exchange *endpointApi.Exchange) bool {
	exchange.Out.SetStatusCode(http.StatusBadRequest)
	exchange.Out.SetBody([]byte("no found username for" + username))
	return false
}

// GetRuleGoFunc 动态获取指定用户规则链池
func GetRuleGoFunc(exchange *endpointApi.Exchange) types.RuleEnginePool {
	msg := exchange.In.GetMsg()
	username := msg.Metadata.GetValue(constants.KeyUsername)
	if s, ok := service.UserRuleEngineServiceImpl.Get(username); !ok {
		exchange.In.SetError(fmt.Errorf("not found username=%s", username))
		return engine.DefaultPool
	} else {
		return s.Pool
	}
}
