// +build go1.16

package tcp

import (
	"errors"
	"fmt"
	"log"
	"net"
)

func (d *TCP) Listen() (err error) {
	d.l, err = net.Listen("tcp", fmt.Sprintf(":%d", d.group.Port))
	if err != nil {
		return
	}
	defer d.l.Close()
	log.Printf("[tcp] listen on :%v\n", d.group.Port)
	for {
		conn, err := d.l.Accept()
		if err != nil {
			switch err := err.(type) {
			case *net.OpError:
				if errors.Is(err.Unwrap(), net.ErrClosed) {
					return nil
				}
			}
			log.Printf("[error] ReadFrom: %v", err)
			continue
		}
		go func() {
			err := d.handleConn(conn)
			if err != nil {
				log.Println(err)
			}
		}()
	}
}