package main

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/ISADBA/wasm-go/pkg/log"
	"github.com/ISADBA/wasm-go/pkg/wrapper"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/tidwall/gjson"
	"github.com/zmap/go-iptree/iptree"
)

const (
	DefaultRealIpHeader string = "X-Forwarded-For"
	DefaultDenyStatus   uint32 = 403
	DefaultDenyMessage  string = "Your IP address is blocked."
)
const (
	OriginSourceType = "origin-source"
	HeaderSourceType = "header"
)

type RestrictionConfig struct {
	IPSourceType string         `json:"ip_source_type"` //IP来源类型
	IPHeaderName string         `json:"ip_header_name"` //真实IP头
	Allow        *iptree.IPTree `json:"allow"`          //允许的IP
	Deny         *iptree.IPTree `json:"deny"`           //拒绝的IP
	Status       uint32         `json:"status"`         //被拒绝时返回的状态码
	Message      string         `json:"message"`        //被拒绝时返回的消息
}

func main() {}

func init() {
	wrapper.SetCtx(
		"ip-restriction",
		wrapper.ParseConfigBy(parseConfig),
		wrapper.ProcessRequestHeadersBy(onHttpRequestHeaders))
}

func parseConfig(json gjson.Result, config *RestrictionConfig, log log.Log) error {
	sourceType := json.Get("ip_source_type")
	if sourceType.Exists() && sourceType.String() != "" {
		switch sourceType.String() {
		case HeaderSourceType:
			config.IPSourceType = HeaderSourceType
		case OriginSourceType:
			config.IPSourceType = OriginSourceType
		default:
			config.IPSourceType = OriginSourceType
		}
	} else {
		config.IPSourceType = OriginSourceType
	}

	header := json.Get("ip_header_name")
	if header.Exists() && header.String() != "" {
		config.IPHeaderName = header.String()
	} else {
		config.IPHeaderName = DefaultRealIpHeader
	}
	status := json.Get("status")
	if status.Exists() && status.Uint() > 1 {
		config.Status = uint32(status.Uint())
	} else {
		config.Status = DefaultDenyStatus
	}
	message := json.Get("message")
	if message.Exists() && message.String() != "" {
		config.Message = message.String()
	} else {
		config.Message = DefaultDenyMessage
	}
	allowNets, err := parseIPNets(json.Get("allow").Array())
	if err != nil {
		log.Error(err.Error())
		return err
	}
	denyNets, err := parseIPNets(json.Get("deny").Array())
	if err != nil {
		log.Error(err.Error())
		return err
	}
	if allowNets != nil && denyNets != nil {
		log.Warn("allow and deny cannot be set at the same time")
		return fmt.Errorf("allow and deny cannot be set at the same time")
	}
	if allowNets == nil && denyNets == nil {
		log.Warn("allow and deny cannot be empty at the same time")
		return fmt.Errorf("allow and deny cannot be empty at the same time")
	}
	config.Allow = allowNets
	config.Deny = denyNets
	return nil
}

func getDownStreamIp(config RestrictionConfig, log log.Log) (net.IP, error) {
	var (
		s   string
		err error
	)

	if config.IPSourceType == HeaderSourceType {
		s, err = proxywasm.GetHttpRequestHeader(config.IPHeaderName)
		log.Infof("[ip-restriction] Getting IP from header '%s': %s", config.IPHeaderName, s)
	} else {
		var bs []byte
		bs, err = proxywasm.GetProperty([]string{"source", "address"})
		s = string(bs)
		log.Infof("[ip-restriction] Getting IP from source.address property: %s", s)
	}
	if err != nil {
		log.Errorf("[ip-restriction] Failed to get IP source: %v", err)
		return nil, err
	}
	ip := parseIP(s, config.IPSourceType == HeaderSourceType)
	log.Infof("[ip-restriction] Parsed IP: %s (from raw: %s)", ip, s)
	realIP := net.ParseIP(ip)
	if realIP == nil {
		log.Errorf("[ip-restriction] Invalid IP format: %s", ip)
		return nil, fmt.Errorf("invalid ip[%s]", ip)
	}
	return realIP, nil
}

func onHttpRequestHeaders(context wrapper.HttpContext, config RestrictionConfig, log log.Log) types.Action {
	log.Infof("[ip-restriction] ========== Request Start ==========")

	// 尝试读取 consumer 信息
	consumerHeader, _ := proxywasm.GetHttpRequestHeader("X-Mse-Consumer")
	log.Infof("[ip-restriction] X-Mse-Consumer header: %s", consumerHeader)

	consumerProperty, err := proxywasm.GetProperty([]string{"consumer_name"})
	if err != nil {
		log.Infof("[ip-restriction] Failed to get consumer_name property: %v", err)
	} else {
		log.Infof("[ip-restriction] consumer_name property: %s", string(consumerProperty))
	}

	// 获取真实 IP
	realIp, err := getDownStreamIp(config, log)
	if err != nil {
		log.Errorf("[ip-restriction] Failed to get downstream IP: %v", err)
		return deniedUnauthorized(config, "get_ip_failed")
	}
	log.Infof("[ip-restriction] Real IP: %s (source type: %s)", realIp.String(), config.IPSourceType)

	allow := config.Allow
	deny := config.Deny

	log.Infof("[ip-restriction] Config - Allow list exists: %v, Deny list exists: %v", allow != nil, deny != nil)

	if allow != nil {
		if realIp == nil {
			log.Error("[ip-restriction] realIp is nil, blocked")
			return deniedUnauthorized(config, "empty_ip")
		}
		if _, found, _ := allow.Get(realIp); !found {
			log.Warnf("[ip-restriction] IP %s not in allow list, blocked", realIp.String())
			return deniedUnauthorized(config, "ip_not_allowed")
		}
		log.Infof("[ip-restriction] IP %s is in allow list, allowed", realIp.String())
	}
	if deny != nil {
		if realIp == nil {
			log.Error("[ip-restriction] realIp is nil, continue")
			return types.ActionContinue
		}
		if _, found, _ := deny.Get(realIp); found {
			log.Warnf("[ip-restriction] IP %s is in deny list, blocked", realIp.String())
			return deniedUnauthorized(config, "ip_denied")
		}
		log.Infof("[ip-restriction] IP %s not in deny list, allowed", realIp.String())
	}

	log.Infof("[ip-restriction] ========== Request End (Continue) ==========")
	return types.ActionContinue
}

func deniedUnauthorized(config RestrictionConfig, reason string) types.Action {
	body, _ := json.Marshal(map[string]string{
		"message": config.Message,
	})
	_ = proxywasm.SendHttpResponseWithDetail(config.Status, "key-auth."+reason, nil, body, -1)
	return types.ActionContinue
}
