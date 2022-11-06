package http

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"i04Dns/common"
	"i04Dns/util"
	"io"
	"net"
	"net/http"
	"os"
)

const ModuleName = "Http-Server"

type Server struct {
	runningState *util.RunningState
	appCtx       *common.AppContext
	listener     net.Listener
}

func New() *Server {
	return &Server{
		runningState: util.NewRunningState(),
	}
}

func loadCertPool(fileName *string) (*x509.CertPool, error) {
	pemBytes, err := os.ReadFile(*fileName)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(pemBytes)
	if block.Type != "CERTIFICATE" {
		return nil, errors.New("first block is not a cert")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	caPool := x509.NewCertPool()
	caPool.AddCert(cert)
	return caPool, nil
}

func (srv *Server) Run(ctx *common.AppContext) {
	if srv.runningState.IsOn() || !ctx.GetConfig().Http.Enabled {
		return
	}
	srv.runningState.SetOn()
	srv.appCtx = ctx
	conf := ctx.GetConfig().Http
	// Load TLS stuff
	util.LogInfo(ModuleName, "Loading tls files")
	cert, err := tls.LoadX509KeyPair(*conf.TlsCrt, *conf.TlsKey)
	if err != nil {
		util.LogInfo(ModuleName, "Loading tls files... ERROR")
		srv.runningState.SignalShutdownComplete()
		return
	}
	certPool, err := loadCertPool(conf.TlsTrustCrt)
	if err != nil {
		util.LogInfo(ModuleName, "Loading tls files... ERROR")
		srv.runningState.SignalShutdownComplete()
		return
	}
	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ServerName:   *conf.TlsServerName,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
		MinVersion:   tls.VersionTLS12,
	}
	util.LogInfo(ModuleName, "Loading tls files... OK")
	// open tcp port
	util.LogInfo(ModuleName, "Opening port")
	listener, err := net.Listen(*conf.Network, fmt.Sprintf("%s:%d", *conf.Host, conf.Port))
	if err != nil {
		util.LogInfo(ModuleName, "Opening port... ERROR")
		srv.runningState.SignalShutdownComplete()
		return
	}
	util.LogInfo(ModuleName, "Opening port... OK")
	// wrap in tls
	srv.listener = tls.NewListener(listener, config)
	_ = http.Serve(srv.listener, srv)
}

func (srv *Server) Stop() {
	if srv.listener != nil {
		_ = srv.listener.Close()
		srv.listener = nil
	}
	srv.runningState.SetOff()
}

func (srv *Server) OnConfigUpdate() {
	// nothing to do... a restart is required
}

func (srv *Server) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	fmt.Printf("[%s] %s\n", req.Method, req.URL.Path)
	_, _ = io.WriteString(rw, fmt.Sprintf("{\"msg\": \"You requested %s\"}", req.URL.Path))
}
