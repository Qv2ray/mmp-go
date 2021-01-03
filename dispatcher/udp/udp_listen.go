package udp

import (
	"github.com/Qv2ray/mmp-go/common/pool"
	"github.com/Qv2ray/mmp-go/dispatcher/infra"
	"log"
	"net"
	"strings"
)

func (d *UDP) Listen() (err error) {
	d.c, err = net.ListenUDP("udp", &net.UDPAddr{Port: d.group.Port})
	if err != nil {
		return
	}
	defer d.c.Close()
	log.Printf("[udp] listen on :%v\n", d.group.Port)
	var buf [MTU]byte
	for {
		n, laddr, err := d.c.ReadFrom(buf[:])
		if err != nil {
			switch err := err.(type) {
			case *net.OpError:
				// FIXME:
				// use `if errors.Is(err.Unwrap(), net.ErrClosed) {` with go1.16 instead.
				if strings.HasSuffix(err.Error(), infra.ErrNetClosing.Error()) {
					return nil
				}
			}
			log.Printf("[error] ReadFrom: %v", err)
			continue
		}
		data := pool.Get(n)
		copy(data, buf[:n])
		go func() {
			err := d.handleConn(laddr, data, n)
			if err != nil {
				log.Println(err)
			}
			pool.Put(data)
		}()
	}
}

