# mmp-go
Mega Multiplexer, port mutiplexer for shadowsocks, supports AEAD methods only.

### Intro
> \- 草，这破 NAT 🐔怎么就俩端口？<br/>
> \- mmp，go！<br/>
> \- ？<br/>
> \- ？？？

### Usage

```shell
go get -u github.com/Qv2ray/mmp-go
mmp-go -conf example.json
```

Refer to `example.json`

### AEAD methods supported

- chacha20-ietf-poly1305 (chacha20-poly1305)
- aes-256-gcm
- aes-128-gcm

### Authors

- mzz2017
- DuckSoft

### Special Thanks:

- Qv2ray Developer Community
