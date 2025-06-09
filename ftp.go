package main

/*
#include <stdlib.h>
*/
import "C"
import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net"
	"unsafe"
	"time"
	"io"
	"strings"
	"net/url"
	"strconv"
)

const (
	maxFileSize = 90 * 1024 * 1024 // 10MB límite
	timeout     = 30 * time.Second
)

//export GetFTPFile
func GetFTPFile(ftpUrl *C.char) *C.char {
	urlStr := C.GoString(ftpUrl)

	if urlStr == "" {
		return C.CString("Error: URL vacía")
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return C.CString(fmt.Sprintf("Error analizando URL: %v", err))
	}

	if u.Scheme != "ftp" {
		return C.CString("Error: El URL debe comenzar con ftp://")
	}

	user := u.User.Username()
	pass, _ := u.User.Password()
	host := u.Host
	path := u.Path

	if !strings.Contains(host, ":") {
		host += ":21"
	}

	if host == "" || user == "" {
		return C.CString("Error: URL debe incluir host y usuario")
	}

	// Conexión con timeout
	conn, err := net.DialTimeout("tcp", host, timeout)
	if err != nil {
		return C.CString(fmt.Sprintf("Error conectando al servidor: %v", err))
	}
	defer conn.Close()

	// Configurar deadline para operaciones
	conn.SetDeadline(time.Now().Add(timeout))

	var buf [1024]byte
	n, err := conn.Read(buf[:])
	if err != nil {
		return C.CString(fmt.Sprintf("Error leyendo respuesta inicial: %v", err))
	}

	// Autenticación
	if _, err = fmt.Fprintf(conn, "USER %s\r\n", user); err != nil {
		return C.CString(fmt.Sprintf("Error enviando usuario: %v", err))
	}
	if n, err = conn.Read(buf[:]); err != nil || !strings.HasPrefix(string(buf[:n]), "331") {
		return C.CString("Error en autenticación de usuario")
	}

	if _, err = fmt.Fprintf(conn, "PASS %s\r\n", pass); err != nil {
		return C.CString(fmt.Sprintf("Error enviando contraseña: %v", err))
	}
	if n, err = conn.Read(buf[:]); err != nil || !strings.HasPrefix(string(buf[:n]), "230") {
		return C.CString("Error en autenticación de contraseña")
	}

	// Configuración de transferencia
	if _, err = fmt.Fprintf(conn, "TYPE I\r\n"); err != nil {
		return C.CString(fmt.Sprintf("Error configurando modo binario: %v", err))
	}
	conn.Read(buf[:])

	// Modo pasivo
	if _, err = fmt.Fprintf(conn, "PASV\r\n"); err != nil {
		return C.CString(fmt.Sprintf("Error entrando en modo pasivo: %v", err))
	}
	if n, err = conn.Read(buf[:]); err != nil {
		return C.CString(fmt.Sprintf("Error leyendo respuesta PASV: %v", err))
	}

	pasvResp := string(buf[:n])
	dataAddr, err := parsePASV(pasvResp)
	if err != nil {
		return C.CString(fmt.Sprintf("Error analizando modo pasivo: %v", err))
	}

	dataConn, err := net.DialTimeout("tcp", dataAddr, timeout)
	if err != nil {
		return C.CString(fmt.Sprintf("Error conectando para datos: %v", err))
	}
	defer dataConn.Close()
	dataConn.SetDeadline(time.Now().Add(timeout))

	// Solicitar archivo
	if _, err = fmt.Fprintf(conn, "RETR %s\r\n", path); err != nil {
		return C.CString(fmt.Sprintf("Error solicitando archivo: %v", err))
	}

	if n, err = conn.Read(buf[:]); err != nil || !strings.HasPrefix(string(buf[:n]), "150") {
		return C.CString("Error iniciando transferencia de archivo")
	}

	// Leer con límite de tamaño
	limitedReader := &io.LimitedReader{R: dataConn, N: maxFileSize}
	var buffer bytes.Buffer

	if _, err = io.Copy(&buffer, limitedReader); err != nil {
		return C.CString(fmt.Sprintf("Error recibiendo datos: %v", err))
	}

	if limitedReader.N <= 0 {
		return C.CString("Error: archivo excede el tamaño máximo permitido")
	}

	if buffer.Len() == 0 {
		return C.CString("Advertencia: El archivo está vacío")
	}

	encoded := base64.StdEncoding.EncodeToString(buffer.Bytes())
	return C.CString(encoded)
}

func parsePASV(resp string) (string, error) {
	start := strings.Index(resp, "(")
	end := strings.Index(resp, ")")
	if start == -1 || end == -1 {
		return "", fmt.Errorf("formato PASV inválido")
	}

	parts := strings.Split(resp[start+1:end], ",")
	if len(parts) < 6 {
		return "", fmt.Errorf("formato PASV inválido")
	}

	ip := strings.Join(parts[0:4], ".")

	p1, err1 := strconv.Atoi(parts[4])
	p2, err2 := strconv.Atoi(parts[5])
	if err1 != nil || err2 != nil {
		return "", fmt.Errorf("puerto PASV inválido")
	}
	port := p1*256 + p2

	return fmt.Sprintf("%s:%d", ip, port), nil
}


//export GetFTPText
func GetFTPText(ftpUrl *C.char) *C.char {
	urlStr := C.GoString(ftpUrl)

	if urlStr == "" {
		return C.CString("Error: URL vacía")
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return C.CString(fmt.Sprintf("Error analizando URL: %v", err))
	}

	if u.Scheme != "ftp" {
		return C.CString("Error: El URL debe comenzar con ftp://")
	}

	user := u.User.Username()
	pass, _ := u.User.Password()
	host := u.Host
	path := u.Path

	if !strings.Contains(host, ":") {
		host += ":21"
	}

	if host == "" || user == "" {
		return C.CString("Error: URL debe incluir host y usuario")
	}

	// Conexión con timeout
	conn, err := net.DialTimeout("tcp", host, timeout)
	if err != nil {
		return C.CString(fmt.Sprintf("Error conectando al servidor: %v", err))
	}
	defer conn.Close()

	// Configurar deadline para operaciones
	conn.SetDeadline(time.Now().Add(timeout))

	var buf [1024]byte
	n, err := conn.Read(buf[:])
	if err != nil {
		return C.CString(fmt.Sprintf("Error leyendo respuesta inicial: %v", err))
	}

	// Autenticación
	if _, err = fmt.Fprintf(conn, "USER %s\r\n", user); err != nil {
		return C.CString(fmt.Sprintf("Error enviando usuario: %v", err))
	}
	if n, err = conn.Read(buf[:]); err != nil || !strings.HasPrefix(string(buf[:n]), "331") {
		return C.CString("Error en autenticación de usuario")
	}

	if _, err = fmt.Fprintf(conn, "PASS %s\r\n", pass); err != nil {
		return C.CString(fmt.Sprintf("Error enviando contraseña: %v", err))
	}
	if n, err = conn.Read(buf[:]); err != nil || !strings.HasPrefix(string(buf[:n]), "230") {
		return C.CString("Error en autenticación de contraseña")
	}

	// Configurar modo ASCII para archivos de texto
	if _, err = fmt.Fprintf(conn, "TYPE A\r\n"); err != nil {
		return C.CString(fmt.Sprintf("Error configurando modo ASCII: %v", err))
	}
	conn.Read(buf[:])

	// Modo pasivo
	if _, err = fmt.Fprintf(conn, "PASV\r\n"); err != nil {
		return C.CString(fmt.Sprintf("Error entrando en modo pasivo: %v", err))
	}
	if n, err = conn.Read(buf[:]); err != nil {
		return C.CString(fmt.Sprintf("Error leyendo respuesta PASV: %v", err))
	}

	pasvResp := string(buf[:n])
	dataAddr, err := parsePASV(pasvResp)
	if err != nil {
		return C.CString(fmt.Sprintf("Error analizando modo pasivo: %v", err))
	}

	dataConn, err := net.DialTimeout("tcp", dataAddr, timeout)
	if err != nil {
		return C.CString(fmt.Sprintf("Error conectando para datos: %v", err))
	}
	defer dataConn.Close()
	dataConn.SetDeadline(time.Now().Add(timeout))

	// Solicitar archivo
	if _, err = fmt.Fprintf(conn, "RETR %s\r\n", path); err != nil {
		return C.CString(fmt.Sprintf("Error solicitando archivo: %v", err))
	}

	if n, err = conn.Read(buf[:]); err != nil || !strings.HasPrefix(string(buf[:n]), "150") {
		return C.CString("Error iniciando transferencia de archivo")
	}

	// Leer con límite de tamaño
	limitedReader := &io.LimitedReader{R: dataConn, N: maxFileSize}
	var buffer bytes.Buffer

	if _, err = io.Copy(&buffer, limitedReader); err != nil {
		return C.CString(fmt.Sprintf("Error recibiendo datos: %v", err))
	}

	if limitedReader.N <= 0 {
		return C.CString("Error: archivo excede el tamaño máximo permitido")
	}

	if buffer.Len() == 0 {
		return C.CString("Advertencia: El archivo está vacío")
	}

	// Convertir a string y limpiar posibles caracteres especiales
	text := buffer.String()
	text = strings.ReplaceAll(text, "\r\n", "\n") // Normalizar saltos de línea
	text = strings.TrimSpace(text)

	return C.CString(text)
}

//export FreeString
func FreeString(str *C.char) {
	C.free(unsafe.Pointer(str))
}

func main() {}