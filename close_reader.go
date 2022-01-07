/*
 * ------------------------------------------------------------------------
 *   File Name: close_reader.go
 *      Author: Zhao Yanbai
 *              2022-01-07 15:27:48 Friday CST
 * Description: none
 * ------------------------------------------------------------------------
 */

package main

type CloseReader interface {
	CloseRead() error
}
