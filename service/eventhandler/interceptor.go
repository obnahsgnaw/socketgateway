package eventhandler

import "github.com/obnahsgnaw/socketgateway/pkg/socket"

type HandleFunc func(conn socket.Conn, pkg []byte) ([]byte, error)

func (e *Event) AppendReceiveInterceptor(i HandleFunc) {
	if i != nil {
		e.receiveInterceptors = append(e.receiveInterceptors, i)
	}
}

func (e *Event) AppendSendInterceptor(i HandleFunc) {
	if i != nil {
		e.sendInterceptors = append(e.sendInterceptors, i)
	}
}

func (e *Event) PrependReceiveInterceptor(i HandleFunc) {
	if i != nil {
		e.receiveInterceptors = append([]HandleFunc{i}, e.receiveInterceptors...)
	}
}

func (e *Event) PrependSendInterceptor(i HandleFunc) {
	if i != nil {
		e.sendInterceptors = append([]HandleFunc{i}, e.sendInterceptors...)
	}
}

func (e *Event) receiveIntercept(conn socket.Conn, pkg []byte) (newPkg []byte, err error) {
	if len(e.receiveInterceptors) == 0 {
		newPkg = pkg
		return
	}
	if len(pkg) == 0 {
		return
	}
	for _, i := range e.receiveInterceptors {
		newPkg, err = i(conn, pkg)
		if len(newPkg) == 0 || err != nil {
			break
		}
	}
	return
}

func (e *Event) sendIntercept(conn socket.Conn, pkg []byte) (newPkg []byte, err error) {
	if len(e.sendInterceptors) == 0 {
		newPkg = pkg
		return
	}
	if len(pkg) == 0 {
		return
	}
	for _, i := range e.sendInterceptors {
		newPkg, err = i(conn, pkg)
		if len(pkg) == 0 || err != nil {
			break
		}
	}
	return
}

func (e *Event) openIntercept() error {
	if e.openInterceptor == nil {
		return nil
	}
	return e.openInterceptor()
}
