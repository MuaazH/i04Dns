package common

import (
	"i04Dns/config"
	"sync"
)

type AppContext struct {
	lock       sync.Mutex
	HttpServer Service
	DnsServer  Service
	config     *config.Configuration
}

type Service interface {
	Run(appCtx *AppContext)
	Stop()
	OnConfigUpdate()
}

func NewAppContext(http Service, dns Service) *AppContext {
	appCtx := &AppContext{}
	appCtx.DnsServer = dns
	appCtx.HttpServer = http
	return appCtx
}

func (ctx *AppContext) LoadConfig() error {
	ctx.lock.Lock()
	cnf, err := config.Load()
	ctx.lock.Unlock()
	if err == nil {
		ctx.config = cnf
		ctx.DnsServer.OnConfigUpdate()
	}
	return err
}

func (ctx *AppContext) GetConfig() *config.Configuration {
	ctx.lock.Lock()
	cnf := ctx.config
	ctx.lock.Unlock()
	return cnf
}

func (ctx *AppContext) RunDnsServer() {
	ctx.DnsServer.Run(ctx)
}

func (ctx *AppContext) RunHttpServer() {
	ctx.HttpServer.Run(ctx)
}
