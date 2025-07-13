package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

var _ = net.Listen
var _ = os.Exit

func main() {
	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}
		go handleConnection(conn)
	}

}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	reqBuf := make([]byte, 1024)
	conn.Read(reqBuf)
	buffer := string(reqBuf)
	parts := strings.Split(buffer, " ")
	if len(parts) < 2 {
		conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
		return
	}
	path := strings.Split(parts[1], "/")
	reqHeader_Body := strings.SplitN(buffer, "\r\n\r\n", 2)
	reqHeaders := strings.Split(reqHeader_Body[0], "\r\n") //contains header and body
	if strings.Contains(reqHeader_Body[0], "Connection: close") {
		responseBody := "HTTP/1.1 200 OK\r\n" + "Content-Length: " + strconv.Itoa(len("Connection: close")) + "\r\n\r\nConnection: close"
		conn.Write([]byte(responseBody))
		conn.Close()
		fmt.Printf("connection closed")
		defer os.Exit(0)
	}
	fmt.Printf("parts:%v \n", parts)
	switch parts[0] {
	case "GET":
		if path[1] == "echo" {
			getHandleEcho(conn, parts, reqHeader_Body)
		} else if path[1] == "user-agent" {
			getHandleUserAgent(buffer, conn)
		} else if strings.HasPrefix(path[1], "files") {
			getHandleFile(path, conn)
		} else if openFile(parts[1]) {
			var msg = []byte("HTTP/1.1 200 OK\r\n\r\n")
			conn.Write(msg)
		} else {
			var msg = []byte("HTTP/1.1 404 Not Found\r\n\r\n")
			conn.Write(msg)
		}
	case "POST":
		postHandleFile(buffer, conn, path, reqHeaders)
	default:
		fmt.Printf("path[1]=%v\n", path[1])
		fmt.Printf("path[0]=%v\n", path[0])
		conn.Write([]byte("HTTP/1.1 405 Method Not Allowed\r\n\r\n"))
	}
}

func postHandleFile(buffer string, conn net.Conn, path []string, reqHeaders []string) {
	if strings.HasPrefix(path[1], "files") {
		fileName := path[2]
		contentLength := 0
		for _, val := range reqHeaders {
			bodyLength, found := strings.CutPrefix(val, "Content-Length: ")
			if found {
				l, err := strconv.Atoi(bodyLength)
				if err == nil {
					contentLength = l
					break
				} else {
					fmt.Printf("error:%v\n", err)
					return
				}
			}
		}
		reqHeaders = strings.SplitN(buffer, "\r\n\r\n", 2)
		currentReqBody := []byte(reqHeaders[1])
		body := make([]byte, 0, contentLength) //Instead of initializing body with full length, initialize with zero length but preallocate capacity
		body = append(body, currentReqBody...)
		remaining := contentLength - len(currentReqBody)
		for remaining > 0 {
			tmp := make([]byte, remaining)
			n, err := conn.Read(tmp)
			if err != nil {
				fmt.Printf("line 87:error:%v", err)
				return
			}
			remaining = remaining - n
			body = append(body, tmp[:n]...)
		}
		filePath := "/tmp/" + "go" + fileName
		file, err := os.Create(filePath)
		if err == nil {
			_, err := file.Write(body)
			if err != nil {
				fmt.Printf("line 98:error while writing into file:%v\n", err)
			}
			response := []byte("HTTP/1.1 201 Created\r\n\r\n")
			conn.Write(response)
		} else {
			fmt.Printf("line 104:error while creating file:%v", err)
			response := []byte("HTTP/1.1 401 file not Created\r\n\r\n")
			conn.Write(response)
		}
	}
}

func getHandleFile(path []string, conn net.Conn) {
	fileName := path[2]
	filePath := "/tmp/" + fileName
	fmt.Printf("fileName:%v\n", fileName)
	fileContent, err := os.ReadFile(filePath)
	if err == nil {
		fileLength := len(fileContent)
		body := "HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\nContent-Length:" + strconv.Itoa(fileLength) + "\r\n\r\n"
		response := append([]byte(body), fileContent...)
		conn.Write(response)
	} else {
		fmt.Printf("error:%v\n", err)
		response := []byte("HTTP/1.1 404 Not Found\r\n\r\n")
		conn.Write(response)
	}
}

func getHandleUserAgent(buffer string, conn net.Conn) {
	pth := strings.Split(buffer, "\r\n")
	for _, value := range pth {
		if strings.HasPrefix(value, "User-Agent: ") {
			after, _ := strings.CutPrefix(value, "User-Agent: ")
			body := "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length:" + strconv.Itoa(len(after)) + "\r\n\r\n" + after
			response := []byte(body)
			conn.Write(response)
			return
		}
	}
	response := []byte("HTTP/1.1 200 OK\r\n\r\n")
	conn.Write(response)
}

func getHandleEcho(conn net.Conn, parts []string, reqHeaders_Body []string) {
	var spilittedStr []string
	var body string
	headersString := reqHeaders_Body[0]
	reqHeaders := strings.Split(headersString, "\r\n")
	var reqBody string
	for _, val := range reqHeaders {
		encoding, found := strings.CutPrefix(val, "Accept-Encoding: ")
		if found {
			spilittedStr = strings.Split(parts[1], "/echo")
			reqBody = spilittedStr[1][1:len(spilittedStr[1])]
			fmt.Printf("ReqBody:%v\n", reqBody)
			if strings.Contains(encoding, "gzip") {
				var buf bytes.Buffer
				gzipWriter := gzip.NewWriter(&buf)
				gzipWriter.Write([]byte(reqBody))
				gzipWriter.Close()
				bodySize := len(reqBody)
				fmt.Printf("body size:%v\n", bodySize)
				fmt.Printf("writer:%v\n", gzipWriter)
				compressedData := buf.Bytes()
				body = "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n" + "Content-Encoding: gzip\r\n" + "Content-Length: " + strconv.Itoa(len(compressedData)) + "\r\n\r\n"
				conn.Write([]byte(body))
				conn.Write(compressedData)
				return
			} else {
				body = "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n" + "Content-Length: " + strconv.Itoa(len(spilittedStr[1])-1) + "\r\n\r\n" + reqBody
				break
			}
		} else {
			spilittedStr = strings.Split(parts[1], "/echo")
			body = "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n" + "Content-Length: " + strconv.Itoa(len(spilittedStr[1])-1) + "\r\n\r\n" + spilittedStr[1][1:len(spilittedStr[1])]
		}
	}
	var msg = []byte(body)
	conn.Write(msg)
}

func openFile(fileName string) bool {
	if fileName == "/" {
		fmt.Println("OK")
		return true
	}
	parts := strings.Split(fileName, "/")
	_, err := os.Open(parts[1])
	if err != nil {
		fmt.Println("error", err)
		return false
	} else {
		fmt.Println("OK")
	}
	return true
}
