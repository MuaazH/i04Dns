package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"i04Dns/util"
	"io"
	"net"
	"os"
)

type configurationFile struct {
	Http          *HttpServerConfig
	Dns           *DnsServerConfig
	DnsServers    *[]string
	AppRules      *[]AppRule
	LocalDatabase *[]DnsRecord
	Whitelist     *[]NameFilter
	Blacklist     *[]NameFilter
}

func (conf *configurationFile) compileDatabase() (*Database, error) {
	db := NewDatabase()
	count := len(*conf.LocalDatabase)
	for i := 0; i < count; i++ {
		r0, r1 := (*conf.LocalDatabase)[i].toDnsResource()
		if r0 == nil {
			return nil, errors.New("invalid dns server")
		}
		db.Add(r0)
		if r1 != nil {
			db.Add(r1)
		}
	}
	return db, nil
}

func (conf *configurationFile) compileWhitelist() (*FilterList, error) {
	return compileFilterList(conf.Whitelist, "invalid whitelist")
}

func (conf *configurationFile) compileBlacklist() (*FilterList, error) {
	return compileFilterList(conf.Blacklist, "invalid blacklist")
}

func (conf *configurationFile) compileDnsServers() (*[]*net.UDPAddr, error) {
	dnsServersCount := len(*conf.DnsServers)
	dnsServers := make([]*net.UDPAddr, dnsServersCount)
	for i := 0; i < dnsServersCount; i++ {
		ipStr := (*conf.DnsServers)[i]
		if !util.IsValidIP(&ipStr) {
			return nil, errors.New("invalid dns servers, invalid ip")
		}
		ip := net.ParseIP(ipStr)
		dnsServers[i] = &net.UDPAddr{
			IP:   ip,
			Port: 53,
		}
	}
	return &dnsServers, nil
}

func (conf *configurationFile) compileAppRules() (*AppRuleList, error) {
	count := len(*conf.AppRules)
	list := make(AppRuleList, count)
	for i := 0; i < count; i++ {
		rule := (*conf.AppRules)[i]
		if util.IsNullOrBlank(rule.Name) {
			return nil, errors.New(fmt.Sprintf("invalid app name '%s'", *rule.Name))
		}
		if util.IsNullOrBlank(rule.Path) {
			return nil, errors.New("invalid app path")
		}
		wl, err := compileFilterList(rule.Whitelist, "invalid app whitelist")
		if err != nil {
			return nil, err
		}
		bl, err := compileFilterList(rule.Blacklist, "invalid app blacklist")
		if err != nil {
			return nil, err
		}
		compiled := CompiledAppRule{
			Name:      rule.Name,
			Path:      rule.Path,
			Whitelist: wl,
			Blacklist: bl,
		}
		list[i] = &compiled
	}
	list.Sort()
	return &list, nil
}

func (conf *configurationFile) compile() (*Configuration, error) {
	if conf.Http == nil {
		return nil, errors.New("missing http conf")
	}
	if conf.Dns == nil {
		return nil, errors.New("missing dns conf")
	}
	if conf.DnsServers == nil {
		return nil, errors.New("missing dns servers")
	}
	if conf.LocalDatabase == nil {
		return nil, errors.New("missing dns servers")
	}
	if conf.Whitelist == nil {
		return nil, errors.New("missing whitelist")
	}
	if conf.Blacklist == nil {
		return nil, errors.New("missing blacklist")
	}
	if conf.AppRules == nil {
		return nil, errors.New("missing app rules")
	}
	if conf.Http.Enabled {
		if !util.IsValidIP(conf.Http.Host) {
			return nil, errors.New("invalid http conf, bad host ip")
		}
		if conf.Http.Port == 0 {
			return nil, errors.New("invalid http conf, bad port")
		}
		if !util.IsValidTCPNetwork(*conf.Http.Network) {
			return nil, errors.New("invalid http conf, bad network")
		}
		if util.IsNullOrBlank(conf.Http.TlsCrt) {
			return nil, errors.New("invalid http conf, bad tls cert")
		}
		if util.IsNullOrBlank(conf.Http.TlsKey) {
			return nil, errors.New("invalid http conf, bad tls key")
		}
		if util.IsNullOrBlank(conf.Http.TlsTrustCrt) {
			return nil, errors.New("invalid http conf, bad tls trust cert")
		}
		if util.IsNullOrBlank(conf.Http.TlsServerName) {
			return nil, errors.New("invalid http conf, bad tls server name")
		}
	}
	if conf.Dns.Enabled {
		if !util.IsValidIP(conf.Dns.Host) {
			return nil, errors.New("invalid dns server host")
		}
		if !util.IsValidUDPNetwork(*conf.Dns.Network) {
			return nil, errors.New("invalid dns server network")
		}
	}
	// compile lists & what not
	dnsServers, err := conf.compileDnsServers()
	if err != nil {
		return nil, err
	}
	apps, err := conf.compileAppRules()
	if err != nil {
		return nil, err
	}
	db, err := conf.compileDatabase()
	if err != nil {
		return nil, err
	}
	wl, err := conf.compileWhitelist()
	if err != nil {
		return nil, err
	}
	bl, err := conf.compileBlacklist()
	if err != nil {
		return nil, err
	}
	return &Configuration{
		Http:          conf.Http,
		Dns:           conf.Dns,
		LocalDatabase: db,
		AppRules:      apps,
		Whitelist:     wl,
		Blacklist:     bl,
		DnsServers:    dnsServers,
	}, nil
}

// ####################

func compileFilterList(array *[]NameFilter, errorMsg string) (*FilterList, error) {
	if array == nil {
		return NewFilterList(0), nil
	}
	size := len(*array)
	list := NewFilterList(size)
	for i := 0; i < size; i++ {
		nf := (*array)[i]
		if nf.Disabled {
			continue
		}
		if nf.Filter == nil || len(*nf.Filter) == 0 {
			return nil, errors.New(errorMsg)
		}
		if nf.Name == nil {
			list.Add(*nf.Filter, *nf.Filter)
		} else {
			list.Add(*nf.Filter, *nf.Name)
		}
	}
	list.Compile()
	return list, nil
}

// ####################

type Configuration struct {
	Http          *HttpServerConfig
	Dns           *DnsServerConfig
	DnsServers    *[]*net.UDPAddr
	AppRules      *AppRuleList
	LocalDatabase *Database
	Whitelist     *FilterList
	Blacklist     *FilterList
}

func (conf *Configuration) GetNameFilters() (*FilterList, *FilterList) {
	return conf.Whitelist, conf.Blacklist
}

func Load() (*Configuration, error) {
	file, err := os.Open("conf.json")
	if err != nil {
		return nil, errors.New("i/o error")
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	config := configurationFile{}

	bytes, ioErr := io.ReadAll(file)
	if ioErr != nil {
		return nil, errors.New("i/o error")
	}

	jsonErr := json.Unmarshal(bytes, &config)
	if jsonErr != nil {
		return nil, errors.New(fmt.Sprintf("config json error %s", jsonErr.Error()))
	}
	return config.compile()
}
