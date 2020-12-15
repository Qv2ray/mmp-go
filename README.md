# mmp-go

Mega Multiplexer, port mutiplexer for shadowsocks, supports AEAD methods only.

### Intro

You can use mmp-go to reuse single port to forward to multiple shadowsocks servers.

It is suitable for relay servers with a limited number of ports.

Consider the following:

> \- 草，这破 NAT 🐔怎么就俩端口？<br/>
> \- mmp，go！<br/>

```bash
                                                   |------> SS Server1
Client --> Single Port --> Validator / Multiplexer |------> SS Server2
                                                   |------> SS Server3
```

### Usage

```shell
go get -u github.com/Qv2ray/mmp-go
mmp-go -conf example.json
```

Refer to `example.json`

**Docker**

```bash
docker run -d \
	--restart=always \
	--network=host \
	-v /etc/mmp-go:/etc/mmp-go \
	--name mmp-go \
	mzz2017/mmp-go
```

### AEAD methods supported

- chacha20-ietf-poly1305 (chacha20-poly1305)
- aes-256-gcm
- aes-128-gcm

### TODO

- [ ] fallback mechanism

### Authors

- mzz2017
- DuckSoft

### Special Thanks:

- Qv2ray Developer Community
