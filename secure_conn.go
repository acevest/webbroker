/*
 * ------------------------------------------------------------------------
 *   File Name: secure_conn.go
 *      Author: Zhao Yanbai
 *              2022-01-07 11:06:01 Friday CST
 * Description: none
 * ------------------------------------------------------------------------
 */

package main

import (
	"log"
	"net"
)

type SecureConn struct {
	net.Conn
}

func norByteSlice(b []byte) {
	for i, v := range b {
		x := uint8(v)
		x = ^x
		v = byte(x)
		b[i] = v
	}
}

func (s *SecureConn) Read(b []byte) (n int, err error) {
	n, err = s.Conn.Read(b)
	norByteSlice(b)
	log.Printf("secure read %v bytes err %v", n, err)
	return n, err
}

func (s *SecureConn) Write(b []byte) (n int, err error) {
	norByteSlice(b)
	n, err = s.Conn.Write(b)
	log.Printf("secure write %v bytes err %v", n, err)
	return n, err
}

func (s *SecureConn) CloseRead() error {
	if x, ok := s.Conn.(*net.TCPConn); ok {
		return x.CloseRead()
	} else {
		panic("not tcp conn")
	}
}

func (s *SecureConn) CloseWrite() error {
	if x, ok := s.Conn.(*net.TCPConn); ok {
		return x.CloseWrite()
	} else {
		panic("not tcp conn")
	}
}

// type Conn interface {
// 	// Read reads data from the connection.
// 	// Read can be made to time out and return an error after a fixed
// 	// time limit; see SetDeadline and SetReadDeadline.
// 	Read(b []byte) (n int, err error)

// 	// Write writes data to the connection.
// 	// Write can be made to time out and return an error after a fixed
// 	// time limit; see SetDeadline and SetWriteDeadline.
// 	Write(b []byte) (n int, err error)

// 	// Close closes the connection.
// 	// Any blocked Read or Write operations will be unblocked and return errors.
// 	Close() error

// 	// LocalAddr returns the local network address.
// 	LocalAddr() Addr

// 	// RemoteAddr returns the remote network address.
// 	RemoteAddr() Addr

// 	// SetDeadline sets the read and write deadlines associated
// 	// with the connection. It is equivalent to calling both
// 	// SetReadDeadline and SetWriteDeadline.
// 	//
// 	// A deadline is an absolute time after which I/O operations
// 	// fail instead of blocking. The deadline applies to all future
// 	// and pending I/O, not just the immediately following call to
// 	// Read or Write. After a deadline has been exceeded, the
// 	// connection can be refreshed by setting a deadline in the future.
// 	//
// 	// If the deadline is exceeded a call to Read or Write or to other
// 	// I/O methods will return an error that wraps os.ErrDeadlineExceeded.
// 	// This can be tested using errors.Is(err, os.ErrDeadlineExceeded).
// 	// The error's Timeout method will return true, but note that there
// 	// are other possible errors for which the Timeout method will
// 	// return true even if the deadline has not been exceeded.
// 	//
// 	// An idle timeout can be implemented by repeatedly extending
// 	// the deadline after successful Read or Write calls.
// 	//
// 	// A zero value for t means I/O operations will not time out.
// 	SetDeadline(t time.Time) error

// 	// SetReadDeadline sets the deadline for future Read calls
// 	// and any currently-blocked Read call.
// 	// A zero value for t means Read will not time out.
// 	SetReadDeadline(t time.Time) error

// 	// SetWriteDeadline sets the deadline for future Write calls
// 	// and any currently-blocked Write call.
// 	// Even if write times out, it may return n > 0, indicating that
// 	// some of the data was successfully written.
// 	// A zero value for t means Write will not time out.
// 	SetWriteDeadline(t time.Time) error
// }
