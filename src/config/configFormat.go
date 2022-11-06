package config

type HttpServerConfig struct {
	Enabled       bool
	Network       *string
	Host          *string
	Port          uint16
	User          *string
	Pass          *string
	TlsCrt        *string
	TlsKey        *string
	TlsTrustCrt   *string
	TlsServerName *string
}

type DnsServerConfig struct {
	Enabled bool
	Network *string
	Host    *string
}

type DnsRecord struct {
	Type  uint16
	Name  *string
	Ip    *string
	Cname *string
	Pref  uint16
	Mx    *string
}

type NameFilter struct {
	Filter   *string
	Name     *string
	Disabled bool
}

type AppRule struct {
	Name      *string
	Path      *string
	Whitelist *[]NameFilter
	Blacklist *[]NameFilter
}
