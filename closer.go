/*
 * ------------------------------------------------------------------------
 *   File Name: closer.go
 *      Author: Zhao Yanbai
 *              2022-01-07 15:27:48 Friday CST
 * Description: none
 * ------------------------------------------------------------------------
 */

package main

type Closer interface {
	CloseRead() error
	CloseWrite() error
}
